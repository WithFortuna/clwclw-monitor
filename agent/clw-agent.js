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

function detectTmuxPaneId() {
  // Get stable pane ID (%0, %1, etc.) that never changes even after splits
  // Priority 1: $TMUX_PANE environment variable (ALWAYS correct)
  // tmux sets this when the pane is created; all child processes inherit it.
  // tmux display-message returns the FOCUSED pane, which may differ.
  const envPaneId = (process.env.TMUX_PANE || '').trim();
  if (envPaneId && /^%\d+$/.test(envPaneId)) {
    return envPaneId;
  }

  // Priority 2: tmux command (fallback for non-shell contexts)
  try {
    const result = spawnSync('tmux', ['display-message', '-p', '#D'], {
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

function tmuxPaneIdForTarget(target) {
  // Get pane ID for a given target (session:window.pane or session:window)
  if (!target) return null;
  try {
    const result = spawnSync('tmux', ['display-message', '-t', target, '-p', '#D'], {
      encoding: 'utf8',

      stdio: ['ignore', 'pipe', 'pipe'], // Capture stderr for debugging
    });

    // DEBUG: Log tmux command result
    if (result.status !== 0) {
      console.error(`[tmuxPaneIdForTarget] Failed to get pane ID for target: ${target}`);
      console.error(`[tmuxPaneIdForTarget]   Exit code: ${result.status}`);
      console.error(`[tmuxPaneIdForTarget]   Stderr: ${(result.stderr || '').trim()}`);
      console.error(`[tmuxPaneIdForTarget]   Command: tmux display-message -t "${target}" -p '#D'`);
    }

    if (result.status === 0) {
      const s = (result.stdout || '').trim();
      console.error(`[tmuxPaneIdForTarget] Success: target="${target}" → pane_id="${s}"`);
      return s || null;
    }
  } catch (err) {
    console.error(`[tmuxPaneIdForTarget] Exception: ${err.message}`);
  }
  return null;
}

function getPaneTarget(paneId) {
  // Get #S:#I.#P format from pane ID (for logging/debugging only)
  // This should ONLY be used when #S:#I.#P is explicitly needed (e.g., display purposes)
  // Connection tracking should ALWAYS use pane ID instead
  if (!paneId) return null;
  try {
    const result = spawnSync('tmux', ['display-message', '-t', paneId, '-p', '#S:#I.#P'], {
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

function configureStateDirForWork(paneIdOrTarget) {
  // CRITICAL: Use pane ID as primary identifier (stable across pane rearrangements)
  // Fallback to target for backward compatibility
  if (String(process.env.AGENT_STATE_DIR || '').trim()) return { label: '', dir: agentDataDir() };
  const label = String(paneIdOrTarget || '').trim();
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
    claude_status: status,  // NEW: Explicit Claude execution state
    status: status,          // Legacy: Keep for backward compatibility
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

async function failTask(taskId, reason = '') {
  const agentId = getOrCreateAgentId();
  const res = await postJsonResult('/v1/tasks/fail', {
    task_id: taskId,
    agent_id: agentId,
    reason: String(reason || 'Task execution failed'),
    idempotency_key: `fail:${taskId}:${Date.now()}`,
  });

  if (res.statusCode === 200 && res.body && res.body.task) return res.body.task;
  if (res.statusCode === 409) return null; // conflict; ignore
  if (res.statusCode === 404) return null;
  throw new Error(`fail failed: ${res.statusCode} ${res.raw}`);
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

function tmuxHasPaneId(paneId) {
  if (!paneId) return false;
  const result = spawnSync('tmux', ['display-message', '-t', paneId, '-p', '#D'], { stdio: 'ignore' });
  return result.status === 0;
}

function isPaneId(target) {
  // Check if target is a pane ID format (%0, %1, etc.)
  return /^%\d+$/.test(String(target || '').trim());
}

function resolveTarget(target, fallbackPaneId) {
  // Use pane ID if available and valid, otherwise use target
  const t = String(target || '').trim();
  const paneId = String(fallbackPaneId || '').trim();

  if (isPaneId(t)) {
    return t;
  }
  if (paneId && isPaneId(paneId) && tmuxHasPaneId(paneId)) {
    return paneId;
  }
  return t;
}

function tmuxCapture(target, lines = 80, paneId = '') {
  const n = Math.max(10, Math.min(400, parseInt(String(lines || '80'), 10) || 80));
  const resolved = resolveTarget(target, paneId);
  const result = spawnSync('tmux', ['capture-pane', '-t', resolved, '-p', '-S', `-${n}`], {
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

/**
 * Build process tree map from all running processes.
 * Returns: Map<parentPid, [childPid1, childPid2, ...]>
 */
function buildProcessTree() {
  try {
    // Get all processes with PID and PPID (macOS compatible)
    const psResult = spawnSync('ps', ['-ax', '-o', 'pid,ppid'], {
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'ignore'],
    });

    if (psResult.status !== 0) return new Map();

    const lines = String(psResult.stdout || '').split('\n');
    const tree = new Map();

    for (const line of lines) {
      const match = line.trim().match(/^\s*(\d+)\s+(\d+)/);
      if (!match) continue;

      const pid = match[1];
      const ppid = match[2];

      if (!tree.has(ppid)) tree.set(ppid, []);
      tree.get(ppid).push(pid);
    }

    return tree;
  } catch {
    return new Map();
  }
}

/**
 * Find all descendant PIDs recursively.
 */
function findAllDescendants(parentPid, processTree) {
  const descendants = [];
  const children = processTree.get(parentPid) || [];

  for (const childPid of children) {
    descendants.push(childPid);
    // Recursively find descendants of this child
    const childDescendants = findAllDescendants(childPid, processTree);
    descendants.push(...childDescendants);
  }

  return descendants;
}

/**
 * Find all running 'claude' process PIDs.
 */
function findClaudePids() {
  try {
    const psResult = spawnSync('ps', ['-eo', 'pid,comm'], {
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'ignore'],
    });

    if (psResult.status !== 0) return [];

    const lines = String(psResult.stdout || '').split('\n');
    const pids = [];

    for (const line of lines) {
      const match = line.trim().match(/^\s*(\d+)\s+(.+)$/);
      if (!match) continue;

      const pid = match[1];
      const comm = match[2].trim();

      if (comm === 'claude') {
        pids.push(pid);
      }
    }

    return pids;
  } catch {
    return [];
  }
}

/**
 * Detect if Claude Code is running in the given tmux pane.
 * Uses recursive process tree traversal to find 'claude' processes.
 */
function detectClaudeCodeRunning(target, paneId = '') {
  try {
    const resolved = resolveTarget(target, paneId);

    // Get tmux pane PID
    const panePidResult = spawnSync('tmux', ['display-message', '-t', resolved, '-p', '#{pane_pid}'], {
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'ignore'],
    });

    if (panePidResult.status !== 0) return false;

    const panePid = String(panePidResult.stdout || '').trim();
    if (!panePid) return false;

    // Build process tree
    const processTree = buildProcessTree();

    // Find all descendants of this pane
    const descendants = findAllDescendants(panePid, processTree);

    // Find all running claude processes
    const claudePids = findClaudePids();

    if (claudePids.length === 0) return false;

    // Check if any claude PID is in the descendants
    for (const claudePid of claudePids) {
      if (descendants.includes(claudePid)) {
        return true;
      }
    }

    return false;
  } catch (err) {
    return false;
  }
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

function tmuxInject(target, text, paneId = '') {
  const resolved = resolveTarget(target, paneId);

  // Check if pane exists (use pane ID if available, otherwise check session)
  if (isPaneId(resolved)) {
    if (!tmuxHasPaneId(resolved)) {
      throw new Error(`tmux pane not found: ${resolved}`);
    }
  } else if (!tmuxHasSession(resolved)) {
    throw new Error(`tmux session not found: ${tmuxTargetSession(resolved) || resolved}`);
  }

  // best-effort clear + send + ctrl+enter (for Claude Code submission)
  spawnSync('tmux', ['send-keys', '-t', resolved, 'C-u'], { stdio: 'ignore' });
  const resultSend = spawnSync('tmux', ['send-keys', '-t', resolved, text], { stdio: 'ignore' });
  if (resultSend.status !== 0) {
    throw new Error('tmux send-keys failed');
  }
  // Use C-Enter for Claude Code submission (Enter alone = newline)
  spawnSync('tmux', ['send-keys', '-t', resolved, 'C-Enter'], { stdio: 'ignore' });
}

function tmuxSend(target, text, opts = {}) {
  const paneId = opts.paneId || '';
  const resolved = resolveTarget(target, paneId);

  // Check if pane exists
  if (isPaneId(resolved)) {
    if (!tmuxHasPaneId(resolved)) {
      throw new Error(`tmux pane not found: ${resolved}`);
    }
  } else if (!tmuxHasSession(resolved)) {
    throw new Error(`tmux session not found: ${tmuxTargetSession(resolved) || resolved}`);
  }

  const clear = !!opts.clear;
  const enter = opts.enter !== false;
  const payload = String(text || '');

  if (clear) {
    spawnSync('tmux', ['send-keys', '-t', resolved, 'C-u'], { stdio: 'ignore' });
  }
  if (payload) {
    const resultSend = spawnSync('tmux', ['send-keys', '-t', resolved, payload], { stdio: 'ignore' });
    if (resultSend.status !== 0) {
      throw new Error('tmux send-keys failed');
    }
  }
  if (enter) {
    spawnSync('tmux', ['send-keys', '-t', resolved, 'Enter'], { stdio: 'ignore' });
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

function tmuxSendKeys(target, keys, paneId = '') {
  const resolved = resolveTarget(target, paneId);

  // Check if pane exists
  if (isPaneId(resolved)) {
    if (!tmuxHasPaneId(resolved)) {
      throw new Error(`tmux pane not found: ${resolved}`);
    }
  } else if (!tmuxHasSession(resolved)) {
    throw new Error(`tmux session not found: ${tmuxTargetSession(resolved) || resolved}`);
  }

  const list = Array.isArray(keys) ? keys.filter((k) => String(k || '').trim().length > 0) : [];
  if (!list.length) return;

  const resultSend = spawnSync('tmux', ['send-keys', '-t', resolved, ...list], { stdio: 'ignore' });
  if (resultSend.status !== 0) {
    throw new Error('tmux send-keys failed');
  }
}

/**
 * Detect current Claude Code execution mode from tmux screen capture.
 * Returns: 'accept-edits', 'plan-mode', 'bypass-permission', or 'normal'.
 */
function detectCurrentMode(target, paneId = '') {
  try {
    // Capture last 5 lines of screen (mode indicators are usually at the bottom)
    const capture = tmuxCapture(target, 5, paneId);
    const text = String(capture || '').toLowerCase();

    // Check for mode indicators
    if (text.includes('accept edits on')) {
      return 'accept-edits';
    }
    if (text.includes('plan mode on')) {
      return 'plan-mode';
    }
    if (text.includes('bypass permission on')) {
      return 'bypass-permission';
    }

    return 'normal';
  } catch (err) {
    console.error(`[detectCurrentMode] Error: ${err.message}`);
    return 'normal'; // Default to normal on error
  }
}

/**
 * Switch Claude Code to target execution mode using Shift+Tab cycling.
 * Mode order: normal → accept-edits → plan-mode → (bypass-permission) → normal
 * Returns true on success, false on failure.
 */
function switchToMode(target, targetMode, paneId = '', maxRetries = 9) {
  if (!targetMode || targetMode === 'normal') {
    console.log('[switchToMode] No mode switch needed (target: normal or empty)');
    return true;
  }

  const validModes = ['accept-edits', 'plan-mode', 'bypass-permission'];
  if (!validModes.includes(targetMode)) {
    console.error(`[switchToMode] Invalid target mode: ${targetMode}`);
    return false;
  }

  for (let attempt = 1; attempt <= maxRetries; attempt++) {
    try {
      console.log(`[switchToMode] Attempt ${attempt}/${maxRetries}: Switching to ${targetMode}`);

      // Detect current mode
      const currentMode = detectCurrentMode(target, paneId);
      console.log(`[switchToMode] Current mode: ${currentMode}`);

      if (currentMode === targetMode) {
        console.log(`[switchToMode] Already in target mode: ${targetMode}`);
        return true;
      }

      // Calculate shift count based on mode cycle
      // Cycle: normal(0) → accept-edits(1) → plan-mode(2) → bypass-permission(3) → normal(0)
      const modeOrder = ['normal', 'accept-edits', 'plan-mode', 'bypass-permission'];
      const currentIndex = modeOrder.indexOf(currentMode);
      const targetIndex = modeOrder.indexOf(targetMode);

      let shiftCount;
      if (targetIndex > currentIndex) {
        shiftCount = targetIndex - currentIndex;
      } else {
        shiftCount = modeOrder.length - currentIndex + targetIndex;
      }

      console.log(`[switchToMode] Sending Shift+Tab x${shiftCount}`);

      // Send Shift+Tab N times
      for (let i = 0; i < shiftCount; i++) {
        tmuxSendKeys(target, ['S-Tab'], paneId);
        // Small delay between key presses
        spawnSync('sleep', ['0.2'], { stdio: 'ignore' });
      }

      // Wait for mode switch to complete
      spawnSync('sleep', ['0.3'], { stdio: 'ignore' });

      // Verify mode switch
      const newMode = detectCurrentMode(target, paneId);
      console.log(`[switchToMode] New mode after switch: ${newMode}`);

      if (newMode === targetMode) {
        console.log(`[switchToMode] Successfully switched to ${targetMode}`);
        return true;
      }

      console.warn(`[switchToMode] Mode switch verification failed (expected: ${targetMode}, got: ${newMode})`);
    } catch (err) {
      console.error(`[switchToMode] Attempt ${attempt} failed: ${err.message}`);
    }
  }

  console.error(`[switchToMode] Failed to switch to ${targetMode} after ${maxRetries} attempts`);
  return false;
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
      // Detect Claude Code running status
      let claudeRunning = false;
      try {
        const paneId = detectTmuxPaneId();
        claudeRunning = detectClaudeCodeRunning(detectedTarget, paneId);
      } catch {
        claudeRunning = false;
      }

      // Simple: Claude Code running = 'running', not running = 'idle'
      const status = claudeRunning ? 'running' : 'idle';

      await heartbeat(status, '', { tmux_target: detectedTarget, claude_detected: claudeRunning });
      await emitEvent('claude.hook', {
        hook: type,
        cwd: process.cwd(),
        tmux_session: detectTmuxSession(),
        tmux_target: detectedTarget,
        claude_detected: claudeRunning,
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

            // Re-check Claude Code status after task completion
            let finalClaudeRunning = false;
            try {
              const paneId = detectTmuxPaneId();
              finalClaudeRunning = detectClaudeCodeRunning(detectedTarget, paneId);
            } catch {
              finalClaudeRunning = false;
            }

            await heartbeat(finalClaudeRunning ? 'running' : 'idle', '', { claude_detected: finalClaudeRunning });
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

    const intervalSec = Math.max(2, parseInt(process.env.AGENT_HEARTBEAT_INTERVAL_SEC || '2', 10) || 2);

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
    const pollSec = Math.max(2, parseInt(process.env.AGENT_WORK_POLL_INTERVAL_SEC || '2', 10) || 2);
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

    // Get stable pane ID for the target
    const tmuxPaneId = tmuxPaneIdForTarget(tmuxTarget);
    if (tmuxPaneId) {
      console.log(`[agent] detected stable pane ID: ${tmuxPaneId} for target ${tmuxTarget}`);
    }

    console.log(`[agent] worker started: channels=${channels.join(',')} tmux=${tmuxTarget} pane=${tmuxPaneId || 'N/A'} poll=${pollSec}s`);
    await heartbeat('idle', '', {
      tmux_session: tmuxSession || tmuxTarget,
      tmux_target: tmuxTarget,
      tmux_pane_id: tmuxPaneId,
      subscriptions: channels,
      work_channels: channels,
    });

    while (true) {
      const inFlight = readCurrentTask();
      if (inFlight && inFlight.task_id) {
        const taskId = String(inFlight.task_id || '').trim();
        const target = String(inFlight.tmux_target || tmuxTarget).trim();
        const session = String(inFlight.tmux_session || tmuxSession || tmuxTarget).trim();
        const paneId = String(inFlight.tmux_pane_id || tmuxPaneId || '').trim();

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
                tmuxSendKeys(target, keys, paneId);
              } else {
                tmuxSend(target, text, { enter: sendEnter, clear: false, paneId });
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

        // 2) Best-effort: detect Claude Code running status and interactive prompts from tmux output.
        let prompt = null;
        let claudeRunning = false;
        try {
          claudeRunning = detectClaudeCodeRunning(target, paneId);
          const cap = tmuxCapture(target, 120, paneId);
          prompt = detectInteractivePrompt(cap);
        } catch {
          prompt = null;
          claudeRunning = false;
        }

        const promptHash = prompt ? sha1hex(JSON.stringify(prompt)) : '';

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

        // Simple: Claude Code running = 'running', not running = 'idle'
        const status = claudeRunning ? 'running' : 'idle';

        await heartbeat(status, taskId, {
          tmux_session: session,
          tmux_target: target,
          tmux_pane_id: paneId,
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
          let claudeRunning = false;
          try { claudeRunning = detectClaudeCodeRunning(tmuxTarget, tmuxPaneId); } catch {}
          await heartbeat(claudeRunning ? 'running' : 'idle', '', {
            pane_id: tmuxPaneId, // PRIMARY: Stable pane identifier
            tmux_display: getPaneTarget(tmuxPaneId) || tmuxPaneId, // UI display only
            subscriptions: channels,
            work_channels: channels,
            claude_detected: claudeRunning,
          });
        } else { // New task claimed, inject it
          try {
            const executionMode = String(task.execution_mode || '').trim();
            if (executionMode) {
              console.log(`[agent] Switching to execution mode: ${executionMode}`);
              const modeSwitchSuccess = switchToMode(tmuxTarget, executionMode, tmuxPaneId);
              if (!modeSwitchSuccess) throw new Error(`Failed to switch to mode: ${executionMode}`);
              await emitEvent('mode_switched', { task_id: task.id, target_mode: executionMode }, `mode_switched:${task.id}`, task.id);
            }
            
            const payload = formatTaskForInjection(task);
            tmuxInject(tmuxTarget, payload, tmuxPaneId);
            await emitEvent('task.injected', { task_id: task.id, payload }, `task.injected:${task.id}`, task.id);
          } catch (err) {
            console.error(`[agent] inject failed for task ${task.id}: ${String(err?.message || err)}`);
            await failTask(task.id, `Task injection failed: ${err.message}`).catch(console.error);
          }
        }

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
