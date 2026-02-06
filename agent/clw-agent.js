#!/usr/bin/env node

/**
 * clw-agent.js
 *
 * Minimal local agent bridge:
 * - Keep existing Claude-Code-Remote behavior by executing its scripts.
 * - Upload agent heartbeat/events to the Coordinator API (best-effort).
 *
 * NOTE: This is intentionally dependency-free (Node.js built-ins only).
 */

const fs = require('fs');
const http = require('http');
const https = require('https');
const os = require('os');
const path = require('path');
const { spawnSync } = require('child_process');

function loadDotEnvIfPresent(filePath) {
  if (!filePath) return;
  if (!fs.existsSync(filePath)) return;

  try {
    const content = fs.readFileSync(filePath, 'utf8');
    for (const rawLine of content.split(/\r?\n/)) {
      const line = rawLine.trim();
      if (!line || line.startsWith('#')) continue;
      const eq = line.indexOf('=');
      if (eq <= 0) continue;
      const key = line.slice(0, eq).trim();
      let value = line.slice(eq + 1).trim();
      if (!key) continue;
      if (process.env[key] !== undefined && process.env[key] !== '') continue;

      // Remove wrapping quotes
      if (
        (value.startsWith('"') && value.endsWith('"')) ||
        (value.startsWith("'") && value.endsWith("'"))
      ) {
        value = value.slice(1, -1);
      }

      process.env[key] = value;
    }
  } catch {
    // ignore dotenv parsing errors (best-effort)
  }
}

function usage() {
  console.log(`Usage:
  node agent/clw-agent.js heartbeat
  node agent/clw-agent.js hook <completed|waiting>
  node agent/clw-agent.js run
  node agent/clw-agent.js work --channel <name> --tmux-target <target>
    # multiple channels: --channel "backend-domain,notify"
    # target examples: "claude-code", "claude-code:1", "claude-code:1.0"
    # (deprecated) --tmux-session <session>

Env:
  COORDINATOR_URL          default: http://localhost:8080
  COORDINATOR_AUTH_TOKEN   optional
  AGENT_ID                 optional (persisted to agent/data/agent-id.txt)
  AGENT_NAME               optional (default: hostname)
  AGENT_CHANNELS           optional (comma-separated subscriptions; e.g. "backend-domain,notify")
  AGENT_STATE_DIR          optional (state dir override; for multi-agent/multi-session)
  AGENT_HEARTBEAT_INTERVAL_SEC  default: 15
  AGENT_WORK_POLL_INTERVAL_SEC  default: 5
`);
}

function coordinatorBaseUrl() {
  return (process.env.COORDINATOR_URL || 'http://localhost:8080').replace(/\/+$/, '');
}

function coordinatorHeaders() {
  const headers = {
    'Content-Type': 'application/json',
  };

  const token = (process.env.COORDINATOR_AUTH_TOKEN || '').trim();
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }
  return headers;
}

function agentDataDir() {
  const override = String(process.env.AGENT_STATE_DIR || '').trim();
  if (override) {
    if (path.isAbsolute(override)) return override;
    // Relative paths are resolved from repo root for stability across cwd changes.
    const repoRoot = path.resolve(__dirname, '..');
    return path.resolve(repoRoot, override);
  }
  return path.join(__dirname, 'data');
}

function getOrCreateAgentId() {
  const fromEnv = (process.env.AGENT_ID || '').trim();
  if (fromEnv) return fromEnv;

  const dir = agentDataDir();
  const file = path.join(dir, 'agent-id.txt');
  try {
    if (fs.existsSync(file)) {
      const id = fs.readFileSync(file, 'utf8').trim();
      if (id) return id;
    }
  } catch {
    // ignore
  }

  if (!fs.existsSync(dir)) {
    fs.mkdirSync(dir, { recursive: true });
  }

  const id = uuidv4();
  fs.writeFileSync(file, id + '\n', 'utf8');
  return id;
}

function agentName() {
  return (process.env.AGENT_NAME || '').trim() || os.hostname();
}

function parseCommaList(v) {
  const raw = String(v || '').trim();
  if (!raw) return [];
  return raw
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean);
}

function agentSubscriptions(extra = []) {
  const envSubs = parseCommaList(process.env.AGENT_CHANNELS || '');
  const merged = [...envSubs, ...(Array.isArray(extra) ? extra : [])]
    .map((s) => String(s).trim())
    .filter(Boolean);
  return Array.from(new Set(merged)).sort();
}

function uuidv4() {
  const buf = Buffer.alloc(16);
  require('crypto').randomFillSync(buf);

  // RFC 4122 variant + version 4
  buf[6] = (buf[6] & 0x0f) | 0x40;
  buf[8] = (buf[8] & 0x3f) | 0x80;

  const hex = buf.toString('hex');
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}

function detectTmuxSession() {
  try {
    const result = spawnSync('tmux', ['display-message', '-p', '#S'], {
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'ignore'],
    });
    if (result.status === 0) {
      const s = (result.stdout || '').trim();
      return s || null;
    }
  } catch {
    // ignore
  }
  return null;
}

function detectTmuxTarget() {
  // session:window.pane (requires being inside tmux)
  try {
    const result = spawnSync('tmux', ['display-message', '-p', '#S:#I.#P'], {
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'ignore'],
    });
    if (result.status === 0) {
      const s = (result.stdout || '').trim();
      return s || null;
    }
  } catch {
    // ignore
  }
  return null;
}

function tmuxTargetSession(target) {
  const raw = String(target || '').trim();
  if (!raw) return '';
  const idx = raw.indexOf(':');
  if (idx === -1) return raw;
  return raw.slice(0, idx);
}

function tmuxTargetWithoutPane(target) {
  const raw = String(target || '').trim();
  if (!raw) return '';
  const idx = raw.indexOf(':');
  if (idx === -1) return raw;
  const rest = raw.slice(idx + 1);
  const dot = rest.lastIndexOf('.');
  if (dot === -1) return raw;
  return raw.slice(0, idx+1+dot);
}

function stateInstancesRoot() {
  return path.join(__dirname, 'data', 'instances');
}

function safeLabel(label) {
  return String(label || '')
    .trim()
    .replace(/[^a-zA-Z0-9_-]+/g, '_')
    .replace(/^_+|_+$/g, '')
    .slice(0, 32);
}

function hash12(label) {
  try {
    return require('crypto').createHash('sha1').update(String(label || '')).digest('hex').slice(0, 12);
  } catch {
    return '000000000000';
  }
}

function stateKeyForLabel(label) {
  const base = safeLabel(label) || 'tmux';
  return `${base}_${hash12(label)}`;
}

function stateDirForLabel(label) {
  return path.join(stateInstancesRoot(), stateKeyForLabel(label));
}

function ensureDir(dir) {
  if (!dir) return;
  if (!fs.existsSync(dir)) fs.mkdirSync(dir, { recursive: true });
}

function configureStateDirForWork(tmuxTarget) {
  if (String(process.env.AGENT_STATE_DIR || '').trim()) return { label: '', dir: agentDataDir() };
  const label = String(tmuxTarget || '').trim();
  if (!label) return { label: '', dir: agentDataDir() };
  const dir = stateDirForLabel(label);
  ensureDir(dir);
  process.env.AGENT_STATE_DIR = dir;
  return { label, dir };
}

function configureStateDirForHook(detectedTarget) {
  if (String(process.env.AGENT_STATE_DIR || '').trim()) return { label: '', dir: agentDataDir() };
  const full = String(detectedTarget || '').trim();
  if (!full) return { label: '', dir: agentDataDir() };

  const session = tmuxTargetSession(full);
  const noPane = tmuxTargetWithoutPane(full);
  const fullDir = stateDirForLabel(full);
  const noPaneDir = noPane ? stateDirForLabel(noPane) : '';
  const sessionDir = session ? stateDirForLabel(session) : '';

  // Prefer an existing state dir (so hook and worker agree), otherwise default to session-level.
  if (fs.existsSync(fullDir)) {
    ensureDir(fullDir);
    process.env.AGENT_STATE_DIR = fullDir;
    return { label: full, dir: fullDir };
  }
  if (noPaneDir && fs.existsSync(noPaneDir)) {
    ensureDir(noPaneDir);
    process.env.AGENT_STATE_DIR = noPaneDir;
    return { label: noPane, dir: noPaneDir };
  }
  if (sessionDir && fs.existsSync(sessionDir)) {
    ensureDir(sessionDir);
    process.env.AGENT_STATE_DIR = sessionDir;
    return { label: session, dir: sessionDir };
  }

  if (sessionDir) {
    ensureDir(sessionDir);
    process.env.AGENT_STATE_DIR = sessionDir;
    return { label: session, dir: sessionDir };
  }

  ensureDir(fullDir);
  process.env.AGENT_STATE_DIR = fullDir;
  return { label: full, dir: fullDir };
}

async function postJson(pathname, body) {
  const result = await postJsonResult(pathname, body);
  if (result.statusCode < 200 || result.statusCode >= 300) {
    throw new Error(`Coordinator ${pathname} failed: ${result.statusCode} ${result.raw}`);
  }
  return result.body;
}

async function postJsonResult(pathname, body) {
  const url = new URL(coordinatorBaseUrl() + pathname);
  const data = JSON.stringify(body);

  const lib = url.protocol === 'https:' ? https : http;
  const options = {
    protocol: url.protocol,
    hostname: url.hostname,
    port: url.port || (url.protocol === 'https:' ? 443 : 80),
    path: url.pathname + url.search,
    method: 'POST',
    headers: {
      ...coordinatorHeaders(),
      'Content-Length': Buffer.byteLength(data),
    },
  };

  return new Promise((resolve, reject) => {
    const req = lib.request(options, (res) => {
      let buf = '';
      res.setEncoding('utf8');
      res.on('data', (chunk) => (buf += chunk));
      res.on('end', () => {
        let parsed = null;
        try {
          parsed = buf ? JSON.parse(buf) : {};
        } catch {
          parsed = null;
        }
        resolve({ statusCode: res.statusCode || 0, body: parsed, raw: buf });
      });
    });

    req.on('error', reject);
    req.write(data);
    req.end();
  });
}

async function heartbeat(status = 'idle', currentTaskId = '', meta = {}) {
  const agentId = getOrCreateAgentId();
  const tmuxSession = detectTmuxSession();
  const extraSubs = Array.isArray(meta?.subscriptions) ? meta.subscriptions : [];
  const payload = {
    agent_id: agentId,
    name: agentName(),
    status,
    current_task_id: currentTaskId,
    meta: {
      hostname: os.hostname(),
      platform: process.platform,
      pid: process.pid,
      cwd: process.cwd(),
      tmux_session: tmuxSession,
      ...meta,
      subscriptions: agentSubscriptions(extraSubs),
    },
  };
  return postJson('/v1/agents/heartbeat', payload);
}

async function emitEvent(type, payload, idempotencyKey = '', taskId = '') {
  const agentId = getOrCreateAgentId();
  return postJson('/v1/events', {
    agent_id: agentId,
    task_id: String(taskId || '').trim(),
    type,
    payload,
    idempotency_key: idempotencyKey,
  });
}

function runLegacyHook(type) {
  const legacyScript = path.join(__dirname, '..', 'Claude-Code-Remote', 'claude-hook-notify.js');
  const result = spawnSync('node', [legacyScript, type], {
    stdio: 'inherit',
    env: process.env,
  });
  return result.status ?? 1;
}

function currentTaskPath() {
  return path.join(agentDataDir(), 'current-task.json');
}

function readCurrentTask() {
  const file = currentTaskPath();
  if (!fs.existsSync(file)) return null;
  try {
    return JSON.parse(fs.readFileSync(file, 'utf8'));
  } catch {
    return null;
  }
}

function writeCurrentTask(task) {
  const dir = agentDataDir();
  if (!fs.existsSync(dir)) fs.mkdirSync(dir, { recursive: true });
  fs.writeFileSync(currentTaskPath(), JSON.stringify(task, null, 2), 'utf8');
}

function clearCurrentTask() {
  const file = currentTaskPath();
  if (fs.existsSync(file)) fs.unlinkSync(file);
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function parseFlag(args, name) {
  const idx = args.findIndex((a) => a === name);
  if (idx === -1) return null;
  const v = args[idx + 1];
  if (!v) return null;
  return String(v);
}

async function claimTask(channelName, idempotencyKey = '') {
  const agentId = getOrCreateAgentId();
  const res = await postJsonResult('/v1/tasks/claim', {
    agent_id: agentId,
    channel: channelName,
    idempotency_key: String(idempotencyKey || '').trim(),
  });

  if (res.statusCode === 200 && res.body && res.body.task) return res.body.task;
  if (res.statusCode === 404) return null; // no queued tasks
  throw new Error(`claim failed: ${res.statusCode} ${res.raw}`);
}

async function completeTask(taskId) {
  const agentId = getOrCreateAgentId();
  const res = await postJsonResult('/v1/tasks/complete', {
    task_id: taskId,
    agent_id: agentId,
    idempotency_key: `hook:${taskId}:${Date.now()}`,
  });

  if (res.statusCode === 200 && res.body && res.body.task) return res.body.task;
  if (res.statusCode === 409) return null; // conflict; ignore
  if (res.statusCode === 404) return null;
  throw new Error(`complete failed: ${res.statusCode} ${res.raw}`);
}

async function claimTaskInput(taskId) {
  const agentId = getOrCreateAgentId();
  const res = await postJsonResult('/v1/tasks/inputs/claim', {
    task_id: taskId,
    agent_id: agentId,
  });

  if (res.statusCode === 200 && res.body && res.body.input) return res.body.input;
  if (res.statusCode === 404) return null; // no pending inputs
  throw new Error(`claim input failed: ${res.statusCode} ${res.raw}`);
}

function tmuxHasSession(sessionName) {
  const session = tmuxTargetSession(sessionName);
  if (!session) return false;
  const result = spawnSync('tmux', ['has-session', '-t', session], { stdio: 'ignore' });
  return result.status === 0;
}

function tmuxCapture(target, lines = 80) {
  const n = Math.max(10, Math.min(400, parseInt(String(lines || '80'), 10) || 80));
  const result = spawnSync('tmux', ['capture-pane', '-t', target, '-p', '-S', `-${n}`], {
    encoding: 'utf8',
    stdio: ['ignore', 'pipe', 'ignore'],
  });
  if (result.status !== 0) return '';
  return String(result.stdout || '');
}

function normalizePromptText(text) {
  return String(text || '')
    .replace(/\r/g, '')
    .replace(/[ \t]+/g, ' ')
    .trim();
}

function detectInteractivePrompt(captureText) {
  const raw = String(captureText || '');
  if (!raw.trim()) return null;

  const normalized = raw.replace(/\r/g, '');
  const lines = normalized
    .split('\n')
    .map((l) => l.replace(/\s+$/, ''))
    .filter((l) => l.trim().length > 0);

  const tail = lines.slice(-60).join('\n');

  // yes/no prompts
  if (/\(y\/n\)|\[y\/n\]|\[y\/N\]|\[Y\/n\]/i.test(tail)) {
    return {
      kind: 'yes_no',
      prompt: 'Confirm (y/n)',
      options: [
        { key: 'y', label: 'Yes' },
        { key: 'n', label: 'No' },
      ],
      snippet: tail,
    };
  }

  // press-enter prompts
  if (/press\s+enter/i.test(tail)) {
    return {
      kind: 'press_enter',
      prompt: 'Press Enter',
      options: [{ key: 'Enter', label: 'Enter' }],
      snippet: tail,
    };
  }

  // Navigation hints (arrow/tab UI)
  const hintCandidates = lines.slice(-20);
  const navHintLine = hintCandidates.find((l) => {
    const t = String(l || '');
    return /enter\s+to\s+(select|confirm|submit|continue)/i.test(t) && /(esc|escape)\s+to\s+cancel/i.test(t);
  });

  // Numeric option list (Claude Code confirmations, etc.)
  const optRe = /^\s*(?:([❯▷▶▸►>›»→])\s*)?(\d+)\.\s+(.*)$/;
  const opts = [];
  let selectedKey = '';
  for (const line of lines.slice(-40)) {
    const m = line.match(optRe);
    if (!m) continue;
    const marker = String(m[1] || '').trim();
    const key = String(m[2] || '').trim();
    const label = String(m[3] || '').trim();
    if (!key) continue;
    opts.push({ key, label });
    if (!selectedKey && marker) selectedKey = key;
  }

  if (opts.length > 0) {
    // Pick a best-effort prompt line: nearest line above first option that contains '?' or 'Choose'.
    let promptLine = '';
    for (let i = lines.length - 1; i >= 0; i--) {
      if (optRe.test(lines[i])) continue;
      const t = lines[i].trim();
      if (!t) continue;
      if (navHintLine && t === navHintLine.trim()) continue;
      if (t.includes('?') || /choose|select|proceed/i.test(t)) {
        promptLine = t;
        break;
      }
      if (!promptLine) promptLine = t;
    }

    // Deduplicate by key (keep first), preserve numeric order.
    const seen = new Set();
    const unique = [];
    for (const o of opts) {
      if (seen.has(o.key)) continue;
      seen.add(o.key);
      unique.push(o);
    }
    unique.sort((a, b) => parseInt(a.key, 10) - parseInt(b.key, 10));

    const selectedIndex = selectedKey ? unique.findIndex((o) => o.key === selectedKey) : -1;
    return {
      kind: 'choice',
      prompt: normalizePromptText(promptLine),
      options: unique,
      snippet: tail,
      input_mode: navHintLine ? 'arrows' : 'number',
      hint: navHintLine ? String(navHintLine).trim() : '',
      selected_key: selectedKey,
      selected_index: selectedIndex,
    };
  }

  return null;
}

function sha1hex(s) {
  try {
    return require('crypto').createHash('sha1').update(String(s || '')).digest('hex');
  } catch {
    return '';
  }
}

function tmuxInject(target, text) {
  if (!tmuxHasSession(target)) {
    throw new Error(`tmux session not found: ${tmuxTargetSession(target) || target}`);
  }

  // best-effort clear + send + ctrl+enter (for Claude Code submission)
  spawnSync('tmux', ['send-keys', '-t', target, 'C-u'], { stdio: 'ignore' });
  const resultSend = spawnSync('tmux', ['send-keys', '-t', target, text], { stdio: 'ignore' });
  if (resultSend.status !== 0) {
    throw new Error('tmux send-keys failed');
  }
  // Use C-Enter for Claude Code submission (Enter alone = newline)
  spawnSync('tmux', ['send-keys', '-t', target, 'C-Enter'], { stdio: 'ignore' });
}

function tmuxSend(target, text, opts = {}) {
  if (!tmuxHasSession(target)) {
    throw new Error(`tmux session not found: ${tmuxTargetSession(target) || target}`);
  }

  const clear = !!opts.clear;
  const enter = opts.enter !== false;
  const payload = String(text || '');

  if (clear) {
    spawnSync('tmux', ['send-keys', '-t', target, 'C-u'], { stdio: 'ignore' });
  }
  if (payload) {
    const resultSend = spawnSync('tmux', ['send-keys', '-t', target, payload], { stdio: 'ignore' });
    if (resultSend.status !== 0) {
      throw new Error('tmux send-keys failed');
    }
  }
  if (enter) {
    spawnSync('tmux', ['send-keys', '-t', target, 'Enter'], { stdio: 'ignore' });
  }
}

function normalizeTmuxKey(key) {
  const t = String(key || '').trim();
  if (!t) return '';

  const lower = t.toLowerCase();
  if (lower === 'esc') return 'Escape';
  if (lower === 'escape') return 'Escape';
  if (lower === 'enter' || lower === 'return') return 'Enter';
  if (lower === 'tab') return 'Tab';
  if (lower === 'up' || lower === 'arrowup') return 'Up';
  if (lower === 'down' || lower === 'arrowdown') return 'Down';
  if (lower === 'left' || lower === 'arrowleft') return 'Left';
  if (lower === 'right' || lower === 'arrowright') return 'Right';

  if (lower.startsWith('ctrl+')) return `C-${t.slice(5)}`;
  if (lower.startsWith('ctrl-')) return `C-${t.slice(5)}`;
  if (lower.startsWith('c-')) return `C-${t.slice(2)}`;
  if (lower.startsWith('meta+')) return `M-${t.slice(5)}`;
  if (lower.startsWith('meta-')) return `M-${t.slice(5)}`;
  if (lower.startsWith('m-')) return `M-${t.slice(2)}`;

  return t;
}

function parseTmuxKeySequence(text) {
  const out = [];
  for (const line of String(text || '').split('\n')) {
    const k = normalizeTmuxKey(line);
    if (!k) continue;
    if (/\s/.test(k)) continue;
    out.push(k);
  }
  return out;
}

function tmuxSendKeys(target, keys) {
  if (!tmuxHasSession(target)) {
    throw new Error(`tmux session not found: ${tmuxTargetSession(target) || target}`);
  }

  const list = Array.isArray(keys) ? keys.filter((k) => String(k || '').trim().length > 0) : [];
  if (!list.length) return;

  const resultSend = spawnSync('tmux', ['send-keys', '-t', target, ...list], { stdio: 'ignore' });
  if (resultSend.status !== 0) {
    throw new Error('tmux send-keys failed');
  }
}

function formatTaskForInjection(task) {
  const title = String(task?.title || '').trim();
  const desc = String(task?.description || '').trim();
  const combined = desc ? `[TASK] ${title} — ${desc}` : `[TASK] ${title}`;
  return combined.replace(/\s+/g, ' ').trim();
}

async function main() {
  // Best-effort: load Coordinator config from existing Claude-Code-Remote .env
  // so that hooks can work without requiring users to export env vars manually.
  const repoRoot = path.resolve(__dirname, '..');
  loadDotEnvIfPresent(path.join(repoRoot, 'Claude-Code-Remote', '.env'));

  const [cmd, ...args] = process.argv.slice(2);
  if (!cmd) {
    usage();
    process.exit(1);
  }

  if (cmd === 'heartbeat') {
    try {
      await heartbeat();
      console.log('heartbeat ok');
      process.exit(0);
    } catch (err) {
      console.error(String(err?.message || err));
      process.exit(2);
    }
  }

  if (cmd === 'hook') {
    const type = (args[0] || '').trim();
    if (!['completed', 'waiting'].includes(type)) {
      usage();
      process.exit(1);
    }

    const detectedTarget = detectTmuxTarget();
    const state = configureStateDirForHook(detectedTarget);
    if (!(process.env.AGENT_NAME || '').trim() && state.label) {
      process.env.AGENT_NAME = `${os.hostname()}@${state.label}`;
    }

    // 1) Preserve legacy behavior first.
    const exitCode = runLegacyHook(type);

    // 2) Best-effort coordinator upload (must not affect hook outcome).
    try {
      await heartbeat(type === 'completed' ? 'idle' : 'waiting', '', { tmux_target: detectedTarget });
      await emitEvent('claude.hook', {
        hook: type,
        cwd: process.cwd(),
        tmux_session: detectTmuxSession(),
        tmux_target: detectedTarget,
        ts: new Date().toISOString(),
      });

      if (type === 'completed') {
        const current = readCurrentTask();
        const tmuxSession = detectTmuxSession();
        const tmuxTarget = detectTmuxTarget();
        if (current && current.task_id) {
          if (current.tmux_target && tmuxTarget && current.tmux_target !== tmuxTarget) {
            console.error(`[agent] current task tmux mismatch: expected ${current.tmux_target}, got ${tmuxTarget} (skip auto-complete)`);
          } else if (current.tmux_session && tmuxSession && current.tmux_session !== tmuxSession) {
            console.error(`[agent] current task tmux mismatch: expected ${current.tmux_session}, got ${tmuxSession} (skip auto-complete)`);
          } else {
            await completeTask(current.task_id);
            await emitEvent('task.completed', {
              task_id: current.task_id,
              tmux_session: current.tmux_session || tmuxSession,
              tmux_target: current.tmux_target || tmuxTarget,
              ts: new Date().toISOString(),
            }, `task.completed:${current.task_id}`);
            clearCurrentTask();
            await heartbeat('idle', '');
          }
        }
      }
    } catch (err) {
      console.error(`[agent] coordinator upload failed (ignored): ${String(err?.message || err)}`);
    }

    process.exit(exitCode);
  }

  if (cmd === 'run') {
    const legacyDir = path.join(repoRoot, 'Claude-Code-Remote');
    const legacyEntrypoint = path.join(legacyDir, 'start-all-webhooks.js');

    if (!fs.existsSync(legacyEntrypoint)) {
      console.error(`Legacy entrypoint not found: ${legacyEntrypoint}`);
      process.exit(1);
    }

    const intervalSec = Math.max(5, parseInt(process.env.AGENT_HEARTBEAT_INTERVAL_SEC || '15', 10) || 15);

    console.log(`[agent] starting legacy services: ${legacyEntrypoint}`);
    const child = require('child_process').spawn('node', [legacyEntrypoint], {
      stdio: 'inherit',
      cwd: legacyDir,
      env: process.env,
    });

    const beat = async () => {
      try {
        await heartbeat('idle', '', { legacy_pid: child.pid });
      } catch (err) {
        console.error(`[agent] heartbeat failed (ignored): ${String(err?.message || err)}`);
      }
    };

    await beat();
    const timer = setInterval(beat, intervalSec * 1000);

    const shutdown = (signal) => {
      console.log(`[agent] shutting down (${signal})...`);
      clearInterval(timer);
      child.kill(signal);
    };

    process.on('SIGINT', () => shutdown('SIGINT'));
    process.on('SIGTERM', () => shutdown('SIGTERM'));

    child.on('exit', (code, signal) => {
      clearInterval(timer);
      if (signal) {
        console.log(`[agent] legacy exited with signal ${signal}`);
        process.exit(0);
      }
      process.exit(code ?? 0);
    });

    return;
  }

  if (cmd === 'work') {
    const channelArg = (parseFlag(args, '--channel') || '').trim();
    const channels = parseCommaList(channelArg);
    const tmuxTarget = (parseFlag(args, '--tmux-target') || parseFlag(args, '--tmux-session') || process.env.TMUX_TARGET || process.env.TMUX_SESSION || '').trim();
    const tmuxSession = tmuxTargetSession(tmuxTarget);
    const pollSec = Math.max(1, parseInt(process.env.AGENT_WORK_POLL_INTERVAL_SEC || '5', 10) || 5);
    let rrIndex = 0;
    let pendingClaim = null; // { channel: string, key: string }
    let lastPromptHash = '';

    if (!channels.length || !tmuxTarget) {
      usage();
      process.exit(1);
    }

    const state = configureStateDirForWork(tmuxTarget);
    if (!(process.env.AGENT_NAME || '').trim() && state.label) {
      process.env.AGENT_NAME = `${os.hostname()}@${state.label}`;
    }

    console.log(`[agent] worker started: channels=${channels.join(',')} tmux=${tmuxTarget} poll=${pollSec}s`);
    await heartbeat('idle', '', {
      tmux_session: tmuxSession || tmuxTarget,
      tmux_target: tmuxTarget,
      subscriptions: channels,
      work_channels: channels,
    });

    while (true) {
      const inFlight = readCurrentTask();
      if (inFlight && inFlight.task_id) {
        const taskId = String(inFlight.task_id || '').trim();
        const target = String(inFlight.tmux_target || tmuxTarget).trim();
        const session = String(inFlight.tmux_session || tmuxSession || tmuxTarget).trim();

        // 1) If user sent a response, claim and inject it.
        try {
          const input = await claimTaskInput(taskId);
          if (input) {
            const kind = String(input.kind || '').trim() || 'text';
            const text = String(input.text || '');
            const sendEnter = kind === 'keys' ? input.send_enter === true : input.send_enter !== false;

            try {
              if (kind === 'keys') {
                const keys = parseTmuxKeySequence(text);
                if (sendEnter) keys.push('Enter');
                tmuxSendKeys(target, keys);
              } else {
                tmuxSend(target, text, { enter: sendEnter, clear: false });
              }
              await emitEvent(
                'task.input.injected',
                {
                  task_id: taskId,
                  input_id: input.id,
                  kind,
                  text,
                  send_enter: sendEnter,
                  tmux_session: session,
                  tmux_target: target,
                  ts: new Date().toISOString(),
                },
                `task.input.injected:${taskId}:${input.id}`,
                taskId,
              );
            } catch (err) {
              await emitEvent(
                'task.input.inject_failed',
                {
                  task_id: taskId,
                  input_id: input.id,
                  error: String(err?.message || err),
                  tmux_session: session,
                  tmux_target: target,
                  ts: new Date().toISOString(),
                },
                `task.input.inject_failed:${taskId}:${input.id}`,
                taskId,
              );
            }
          }
        } catch (err) {
          console.error(`[agent] input claim failed (ignored): ${String(err?.message || err)}`);
        }

        // 2) Best-effort: detect interactive prompts from tmux output.
        let prompt = null;
        try {
          const cap = tmuxCapture(target, 120);
          prompt = detectInteractivePrompt(cap);
        } catch {
          prompt = null;
        }

        const promptHash = prompt ? sha1hex(JSON.stringify(prompt)) : '';
        const waiting = !!(prompt && prompt.options && prompt.options.length);

        if (prompt && promptHash && promptHash !== lastPromptHash) {
          lastPromptHash = promptHash;
          await emitEvent(
            'task.prompt',
            {
              task_id: taskId,
              kind: prompt.kind,
              prompt: prompt.prompt,
              options: prompt.options,
              snippet: prompt.snippet,
              input_mode: prompt.input_mode,
              hint: prompt.hint,
              selected_key: prompt.selected_key,
              selected_index: prompt.selected_index,
              tmux_session: session,
              tmux_target: target,
              ts: new Date().toISOString(),
            },
            `task.prompt:${taskId}:${promptHash}`,
            taskId,
          );
        }
        if (!prompt) {
          lastPromptHash = '';
        }

        await heartbeat(waiting ? 'waiting' : 'running', taskId, {
          tmux_session: session,
          tmux_target: target,
          subscriptions: channels,
          work_channels: channels,
          interactive_prompt: waiting,
        });
        await sleep(pollSec * 1000);
        continue;
      }

      let task = null;
      let claimedFromChannel = '';

      if (pendingClaim && pendingClaim.channel && pendingClaim.key) {
        try {
          claimedFromChannel = pendingClaim.channel;
          task = await claimTask(pendingClaim.channel, pendingClaim.key);
        } catch (err) {
          console.error(`[agent] claim error: ${String(err?.message || err)}`);
          await sleep(pollSec * 1000);
          continue;
        }
        if (!task) {
          pendingClaim = null;
        } else {
          pendingClaim = null;
        }
      }

      if (!task) {
        for (let i = 0; i < channels.length; i++) {
          const ch = channels[(rrIndex + i) % channels.length];
          const key = `claim:${ch}:${uuidv4()}`;
          try {
            const t = await claimTask(ch, key);
            if (!t) continue;
            task = t;
            claimedFromChannel = ch;
            rrIndex = (rrIndex + i + 1) % channels.length;
            break;
          } catch (err) {
            pendingClaim = { channel: ch, key };
            console.error(`[agent] claim error: ${String(err?.message || err)}`);
            break;
          }
        }
      }

      if (!task) {
        await heartbeat('idle', '', {
          tmux_session: tmuxSession || tmuxTarget,
          tmux_target: tmuxTarget,
          subscriptions: channels,
          work_channels: channels,
        });
        await sleep(pollSec * 1000);
        continue;
      }

      const taskId = String(task.id || '').trim();
      writeCurrentTask({
        task_id: taskId,
        channel_id: task.channel_id,
        channel: claimedFromChannel,
        title: task.title,
        description: task.description,
        tmux_session: tmuxSession,
        tmux_target: tmuxTarget,
        claimed_at: new Date().toISOString(),
      });

      await heartbeat('running', taskId, {
        tmux_session: tmuxSession || tmuxTarget,
        tmux_target: tmuxTarget,
        subscriptions: channels,
        work_channels: channels,
        work_channel: claimedFromChannel,
      });
      await emitEvent('task.claimed', {
        task_id: taskId,
        channel: claimedFromChannel,
        tmux_session: tmuxSession,
        tmux_target: tmuxTarget,
        title: task.title,
      }, `task.claimed:${taskId}`, taskId);

      try {
        const payload = formatTaskForInjection(task);
        tmuxInject(tmuxTarget, payload);
        await emitEvent('task.injected', {
          task_id: taskId,
          tmux_session: tmuxSession,
          tmux_target: tmuxTarget,
          payload,
        }, `task.injected:${taskId}`, taskId);
      } catch (err) {
        console.error(`[agent] inject failed: ${String(err?.message || err)}`);
        await emitEvent('task.inject_failed', {
          task_id: taskId,
          tmux_session: tmuxSession,
          tmux_target: tmuxTarget,
          error: String(err?.message || err),
        }, `task.inject_failed:${taskId}`, taskId);
        // Keep current-task for manual intervention; worker will stay in running state.
      }

      await sleep(pollSec * 1000);
    }
  }

  usage();
  process.exit(1);
}

main().catch((err) => {
  console.error(String(err?.message || err));
  process.exit(1);
});
