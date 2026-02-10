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
const net = require('net');
const os = require('os');
const path = require('path');
const { spawnSync, spawn } = require('child_process');

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

/** Per-process deploy mode. Set at startup by login/work command. */
let _deployMode = null; // 'local' | 'prod' | null

function getDeployMode() {
  if (_deployMode) return _deployMode;
  // Allow env override (e.g. AGENT_MODE=prod node agent/clw-agent.js work ...)
  const fromEnv = (process.env.AGENT_MODE || '').trim();
  if (fromEnv === 'local' || fromEnv === 'prod') return fromEnv;
  return null;
}

function setDeployMode(mode) {
  if (mode !== 'local' && mode !== 'prod') {
    throw new Error(`Invalid deploy mode: ${mode} (must be 'local' or 'prod')`);
  }
  _deployMode = mode;
}

function usage() {
  console.log(`Usage:
  node agent/clw-agent.js login           # Login + optional interactive work setup
  node agent/clw-agent.js work            # Interactive mode (prompts for channel & tmux)
  node agent/clw-agent.js work --channel <name> --tmux-target <target>
    # Flag mode (backward compatible)
    # multiple channels: --channel "backend-domain,notify"
    # target examples: "claude-code", "claude-code:1", "claude-code:1.0"
    # (deprecated) --tmux-session <session>
  node agent/clw-agent.js heartbeat
  node agent/clw-agent.js hook <completed|waiting>
  node agent/clw-agent.js run
  node agent/clw-agent.js agentd          # Start IPC daemon (auto-started by work)
  node agent/clw-agent.js auto-start --session-name <name> --command <command>
  node agent/clw-agent.js request-session --channel <name>

Env:
  COORDINATOR_URL          default: http://localhost:8080 (also persisted per mode)
  COORDINATOR_AUTH_TOKEN   optional
  AGENT_ID                 optional (persisted to agent/{mode}/data/agent-id.txt)
  AGENT_NAME               optional (default: hostname)
  AGENT_MODE               optional: "local" or "prod" (set during login, persisted)
  AGENT_CHANNELS           optional (comma-separated subscriptions; e.g. "backend-domain,notify")
  AGENT_STATE_DIR          optional (state dir override; for multi-agent/multi-session)
  AGENT_HEARTBEAT_INTERVAL_SEC  default: 15
  AGENT_WORK_POLL_INTERVAL_SEC  default: 5
`);
}

/**
 * Mode-based root data directory. Unlike agentDataDir(), this is NOT affected
 * by AGENT_STATE_DIR (which points to a per-pane instance directory).
 * Use this for global per-mode files: coordinator-url.txt, agent-token.txt.
 */
function modeDataDir() {
  const mode = getDeployMode();
  if (!mode) return path.join(__dirname, 'data');
  return path.join(__dirname, mode, 'data');
}

function coordinatorBaseUrl() {
  const fromEnv = (process.env.COORDINATOR_URL || '').trim();
  if (fromEnv) return fromEnv.replace(/\/+$/, '');

  // Fall back to persisted URL (saved by login/worker)
  // Always read from mode root, NOT instance dir (hooks set AGENT_STATE_DIR)
  const file = path.join(modeDataDir(), 'coordinator-url.txt');
  try {
    const url = fs.readFileSync(file, 'utf8').trim();
    if (url) return url.replace(/\/+$/, '');
  } catch {}

  return 'http://localhost:8080';
}

function getAgentToken() {
  // Agent token is per-mode — read from mode root (not instance dir).
  const file = path.join(modeDataDir(), 'agent-token.txt');
  try {
    if (fs.existsSync(file)) {
      const t = fs.readFileSync(file, 'utf8').trim();
      if (t) return t;
    }
  } catch {
    // ignore
  }
  return '';
}

function coordinatorHeaders() {
  const headers = {
    'Content-Type': 'application/json',
  };

  // Priority: agent token > shared API token
  const agentToken = getAgentToken();
  if (agentToken) {
    headers['Authorization'] = `Bearer ${agentToken}`;
    return headers;
  }

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

  const mode = getDeployMode();
  if (!mode) {
    // Fallback for backward compat (mode not yet configured)
    return path.join(__dirname, 'data');
  }
  return path.join(__dirname, mode, 'data');
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
  const mode = getDeployMode();
  if (!mode) return path.join(__dirname, 'data', 'instances');
  return path.join(__dirname, mode, 'data', 'instances');
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

async function getJson(pathname) {
  const url = new URL(coordinatorBaseUrl() + pathname);
  const lib = url.protocol === 'https:' ? https : http;
  const options = {
    protocol: url.protocol,
    hostname: url.hostname,
    port: url.port || (url.protocol === 'https:' ? 443 : 80),
    path: url.pathname + url.search,
    method: 'GET',
    headers: coordinatorHeaders(),
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
        const result = { statusCode: res.statusCode || 0, body: parsed, raw: buf };
        if (res.statusCode < 200 || res.statusCode >= 300) {
          reject(new Error(`GET ${pathname} failed: ${res.statusCode} ${buf}`));
        } else {
          resolve(result.body);
        }
      });
    });

    req.on('error', reject);
    req.end();
  });
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

async function fetchCurrentTaskFromCoordinator() {
  try {
    const agentId = getOrCreateAgentId();
    const result = await getJson(`/v1/agents/${agentId}/current-task`);
    return result?.task || null;
  } catch (err) {
    const errMsg = String(err?.message || err);

    // 404 means no current task (expected case)
    if (errMsg.includes('404')) {
      return null;
    }

    // ECONNREFUSED = Coordinator not running
    if (errMsg.includes('ECONNREFUSED') || errMsg.includes('connect')) {
      console.error(`[agent] ⚠️  Coordinator not running at ${coordinatorBaseUrl()}`);
      console.error(`[agent] ⚠️  Please start coordinator: go run ./coordinator/cmd/coordinator`);
    } else {
      console.error(`[agent] fetch current task failed: ${errMsg}`);
    }

    return null;
  }
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

// ═══════════════════════════════════════════════════════════════════════════
// UDS IPC Utilities
// ═══════════════════════════════════════════════════════════════════════════

function resolveSocketDir() {
  const xdg = (process.env.XDG_RUNTIME_DIR || '').trim();
  if (xdg) return path.join(xdg, 'clwclw');
  return path.join(os.homedir(), '.clwclw', 'run');
}

function resolveSocketPath() {
  return path.join(resolveSocketDir(), 'agentd.sock');
}

function resolveLockPath() {
  return path.join(resolveSocketDir(), 'agentd.pid');
}

function sendMsg(socket, obj) {
  const line = JSON.stringify({ id: uuidv4(), ...obj }) + '\n';
  try {
    socket.write(line);
  } catch {
    // socket may have closed
  }
}

function createNdjsonParser(onMessage) {
  let buffer = '';
  return (chunk) => {
    buffer += chunk;
    let idx;
    while ((idx = buffer.indexOf('\n')) !== -1) {
      const line = buffer.slice(0, idx).trim();
      buffer = buffer.slice(idx + 1);
      if (!line) continue;
      try {
        onMessage(JSON.parse(line));
      } catch {
        // ignore malformed JSON
      }
    }
  };
}

function isAgentdRunning() {
  const lockPath = resolveLockPath();
  try {
    const pid = parseInt(fs.readFileSync(lockPath, 'utf8').trim(), 10);
    if (!pid || isNaN(pid)) return false;
    process.kill(pid, 0); // signal 0 = check alive
    return true;
  } catch {
    return false;
  }
}

function cleanStaleSocket() {
  const sockPath = resolveSocketPath();
  const lockPath = resolveLockPath();
  try { fs.unlinkSync(sockPath); } catch {}
  try { fs.unlinkSync(lockPath); } catch {}
}

function writeLockFile() {
  const lockPath = resolveLockPath();
  fs.writeFileSync(lockPath, String(process.pid) + '\n', 'utf8');
}

// ═══════════════════════════════════════════════════════════════════════════
// agentd Server
// ═══════════════════════════════════════════════════════════════════════════

function startAgentd() {
  return new Promise((resolve, reject) => {
    if (isAgentdRunning()) {
      console.error('[agentd] another instance is already running');
      process.exit(1);
    }

    cleanStaleSocket();

    const dir = resolveSocketDir();
    if (!fs.existsSync(dir)) {
      fs.mkdirSync(dir, { recursive: true, mode: 0o700 });
    }

    const workers = new Map();        // paneId → {socket, agentId, mode, coordinatorUrl, channels, pid}
    const pendingHooks = new Map();   // requestId → {hookSocket, timer, attempts, paneId, hookType, cwd}
    const socketToPaneId = new Map(); // socket → paneId (for disconnect cleanup)

    const HOOK_ACK_TIMEOUT = 15000;
    const HOOK_MAX_RETRIES = 5;

    function handleRegister(msg, socket) {
      const { paneId, agentId, mode, coordinatorUrl, channels, pid } = msg;
      if (!paneId) return;

      // Replace existing worker for same paneId
      const existing = workers.get(paneId);
      if (existing && existing.socket !== socket) {
        socketToPaneId.delete(existing.socket);
      }

      workers.set(paneId, { socket, agentId, mode, coordinatorUrl, channels, pid });
      socketToPaneId.set(socket, paneId);
      console.log(`[agentd] registered worker: pane=${paneId} agent=${agentId} mode=${mode}`);
      sendMsg(socket, { type: 'register_ack', requestId: msg.id, success: true });
    }

    function handleUnregister(msg, socket) {
      const paneId = msg.paneId || socketToPaneId.get(socket);
      if (paneId) {
        workers.delete(paneId);
        console.log(`[agentd] unregistered worker: pane=${paneId}`);
      }
      socketToPaneId.delete(socket);
    }

    function handleHookRequest(msg, hookSocket) {
      const { paneId, hookType, cwd } = msg;
      const requestId = msg.id;

      const worker = workers.get(paneId);
      if (!worker) {
        console.log(`[agentd] no worker for pane=${paneId}, sending hook_no_match`);
        sendMsg(hookSocket, { type: 'hook_no_match', requestId, reason: `no worker registered for pane ${paneId}` });
        return;
      }

      pendingHooks.set(requestId, {
        hookSocket,
        paneId,
        hookType,
        cwd,
        attempts: 0,
      });

      forwardHookToWorker(requestId);
    }

    function forwardHookToWorker(requestId) {
      const pending = pendingHooks.get(requestId);
      if (!pending) return;

      pending.attempts++;
      const worker = workers.get(pending.paneId);

      if (!worker || worker.socket.destroyed) {
        console.log(`[agentd] worker disconnected during hook forward, pane=${pending.paneId}`);
        sendMsg(pending.hookSocket, { type: 'hook_no_match', requestId, reason: 'worker disconnected' });
        pendingHooks.delete(requestId);
        return;
      }

      sendMsg(worker.socket, {
        type: 'hook_forward',
        requestId,
        hookType: pending.hookType,
        paneId: pending.paneId,
        cwd: pending.cwd,
      });

      // Set ack timeout
      pending.timer = setTimeout(() => {
        const p = pendingHooks.get(requestId);
        if (!p) return;

        if (p.attempts < HOOK_MAX_RETRIES) {
          console.log(`[agentd] hook_ack timeout, retrying (attempt ${p.attempts + 1}/${HOOK_MAX_RETRIES})`);
          forwardHookToWorker(requestId);
        } else {
          console.log(`[agentd] hook_ack timeout after ${HOOK_MAX_RETRIES} retries`);
          sendMsg(p.hookSocket, { type: 'hook_result', requestId, success: false, error: 'ack timeout' });
          pendingHooks.delete(requestId);
        }
      }, HOOK_ACK_TIMEOUT);
    }

    function handleHookAck(msg) {
      const { requestId, success, taskId, error } = msg;
      const pending = pendingHooks.get(requestId);
      if (!pending) return;

      clearTimeout(pending.timer);
      sendMsg(pending.hookSocket, { type: 'hook_result', requestId, success, taskId, error });
      pendingHooks.delete(requestId);
      console.log(`[agentd] hook_ack: request=${requestId} success=${success}`);
    }

    function handleSocketClose(socket) {
      const paneId = socketToPaneId.get(socket);
      if (paneId) {
        const worker = workers.get(paneId);
        if (worker && worker.socket === socket) {
          workers.delete(paneId);
          console.log(`[agentd] worker disconnected: pane=${paneId}`);
        }
      }
      socketToPaneId.delete(socket);
    }

    const sockPath = resolveSocketPath();
    const server = net.createServer((socket) => {
      const parser = createNdjsonParser((msg) => {
        if (!msg || !msg.type) return;
        switch (msg.type) {
          case 'register':    handleRegister(msg, socket); break;
          case 'unregister':  handleUnregister(msg, socket); break;
          case 'hook_request': handleHookRequest(msg, socket); break;
          case 'hook_ack':    handleHookAck(msg); break;
        }
      });

      socket.setEncoding('utf8');
      socket.on('data', parser);
      socket.on('close', () => handleSocketClose(socket));
      socket.on('error', () => handleSocketClose(socket));
    });

    server.on('error', (err) => {
      console.error(`[agentd] server error: ${err.message}`);
      process.exit(1);
    });

    server.listen(sockPath, () => {
      // Set socket permissions
      try { fs.chmodSync(sockPath, 0o600); } catch {}
      writeLockFile();
      console.log(`[agentd] listening on ${sockPath} (pid=${process.pid})`);
      console.log(`[agentd] workers: ${workers.size}`);
    });

    const shutdown = (signal) => {
      console.log(`[agentd] shutting down (${signal})...`);
      server.close();
      try { fs.unlinkSync(sockPath); } catch {}
      try { fs.unlinkSync(resolveLockPath()); } catch {}
      process.exit(0);
    };

    process.on('SIGINT', () => shutdown('SIGINT'));
    process.on('SIGTERM', () => shutdown('SIGTERM'));
  });
}

// ═══════════════════════════════════════════════════════════════════════════
// Hook via agentd (IPC client)
// ═══════════════════════════════════════════════════════════════════════════

function tryHookViaAgentd(paneId, hookType) {
  const sockPath = resolveSocketPath();
  if (!fs.existsSync(sockPath)) return Promise.resolve(false);

  return new Promise((resolve) => {
    const TOTAL_TIMEOUT = 90000;
    let settled = false;

    const finish = (result) => {
      if (settled) return;
      settled = true;
      clearTimeout(timer);
      try { socket.end(); } catch {}
      resolve(result);
    };

    const timer = setTimeout(() => finish(false), TOTAL_TIMEOUT);

    const socket = net.createConnection(sockPath);

    socket.setEncoding('utf8');
    socket.on('error', () => finish(false));
    socket.on('close', () => finish(false));

    const parser = createNdjsonParser((msg) => {
      if (msg.type === 'hook_result') {
        finish(msg.success === true);
      } else if (msg.type === 'hook_no_match') {
        finish(false);
      }
    });

    socket.on('data', parser);

    socket.on('connect', () => {
      sendMsg(socket, {
        type: 'hook_request',
        paneId,
        hookType,
        cwd: process.cwd(),
      });
    });
  });
}

// ═══════════════════════════════════════════════════════════════════════════
// hookDirectCoordinator — extracted from existing hook logic
// ═══════════════════════════════════════════════════════════════════════════

async function hookDirectCoordinator(type, detectedPaneId, detectedTarget) {
  process.stderr.write(`[agent] ═══ Coordinator Upload Starting ═══\n`);
  process.stderr.write(`[agent] Hook type: ${type}\n`);
  process.stderr.write(`[agent] Pane ID: ${detectedPaneId || 'N/A'}\n`);
  process.stderr.write(`[agent] Coordinator URL: ${coordinatorBaseUrl()}\n`);

  // Detect Claude Code running status
  let claudeRunning = false;
  try {
    claudeRunning = detectClaudeCodeRunning(detectedTarget, detectedPaneId);
  } catch {
    claudeRunning = false;
  }

  const status = claudeRunning ? 'running' : 'idle';

  console.log(`[agent] Sending heartbeat (status=${status})...`);
  await heartbeat(status, '', {
    pane_id: detectedPaneId,
    tmux_display: getPaneTarget(detectedPaneId) || detectedPaneId,
    claude_detected: claudeRunning,
  });
  console.log(`[agent] Heartbeat sent successfully`);

  console.log(`[agent] Emitting claude.hook event...`);
  await emitEvent('claude.hook', {
    hook: type,
    cwd: process.cwd(),
    pane_id: detectedPaneId,
    claude_detected: claudeRunning,
    ts: new Date().toISOString(),
  });
  console.log(`[agent] claude.hook event emitted`);

  if (type === 'completed') {
    const agentId = getOrCreateAgentId();
    console.log(`[agent] hook completed: agent_id=${agentId}, coordinator=${coordinatorBaseUrl()}`);

    console.log(`[agent] Fetching current task from coordinator...`);
    const current = await fetchCurrentTaskFromCoordinator();
    console.log(`[agent] current task from coordinator: ${current ? JSON.stringify({ id: current.id, title: current.title, assigned_agent_id: current.assigned_agent_id }) : 'null'}`);

    if (current && current.id) {
      console.log(`[agent] attempting to complete task ${current.id} as agent ${agentId}...`);
      const result = await completeTask(current.id);

      if (result) {
        console.log(`[agent] Task ${current.id} completed successfully`);
      } else {
        console.error(`[agent] Task completion returned null (likely 409 conflict or 404)`);
        console.error(`[agent] This usually means agent's current_task_id doesn't match the task being completed`);
      }

      await emitEvent('task.completed', {
        task_id: current.id,
        pane_id: detectedPaneId,
        ts: new Date().toISOString(),
      }, `task.completed:${current.id}`);

      // Re-check Claude Code status after task completion
      let finalClaudeRunning = false;
      try {
        finalClaudeRunning = detectClaudeCodeRunning(detectedTarget, detectedPaneId);
      } catch {
        finalClaudeRunning = false;
      }

      await heartbeat(finalClaudeRunning ? 'running' : 'idle', '', {
        pane_id: detectedPaneId,
        tmux_display: getPaneTarget(detectedPaneId) || detectedPaneId,
        claude_detected: finalClaudeRunning,
      });
    } else {
      console.warn(`[agent] No current task found in coordinator`);
      console.warn(`[agent] Possible reasons:`);
      console.warn(`[agent]   - Agent has no task assigned`);
      console.warn(`[agent]   - Task already completed by another process`);
      console.warn(`[agent]   - Agent ID mismatch between claim and completion`);
    }
  }
}

// ═══════════════════════════════════════════════════════════════════════════
// Work client: agentd integration
// ═══════════════════════════════════════════════════════════════════════════

function ensureAgentdRunning() {
  if (isAgentdRunning()) return;

  console.log('[agent] starting agentd in background...');
  const agentScript = path.resolve(__filename);
  const child = spawn(process.execPath, [agentScript, 'agentd'], {
    detached: true,
    stdio: 'ignore',
    env: process.env,
  });
  child.unref();

  // Wait briefly for socket to appear
  const sockPath = resolveSocketPath();
  for (let i = 0; i < 20; i++) {
    spawnSync('sleep', ['0.1'], { stdio: 'ignore' });
    if (fs.existsSync(sockPath)) {
      console.log('[agent] agentd started successfully');
      return;
    }
  }
  console.warn('[agent] agentd may not have started (socket not found after 2s)');
}

function connectToAgentd(paneId, agentId, mode, coordinatorUrl, channels) {
  const sockPath = resolveSocketPath();
  if (!fs.existsSync(sockPath)) return null;

  return new Promise((resolve) => {
    const socket = net.createConnection(sockPath);
    let resolved = false;

    const timeout = setTimeout(() => {
      if (!resolved) {
        resolved = true;
        try { socket.end(); } catch {}
        resolve(null);
      }
    }, 5000);

    socket.setEncoding('utf8');
    socket.on('error', () => {
      if (!resolved) {
        resolved = true;
        clearTimeout(timeout);
        resolve(null);
      }
    });

    socket.on('connect', () => {
      sendMsg(socket, {
        type: 'register',
        paneId,
        agentId,
        mode: mode || getDeployMode() || 'local',
        coordinatorUrl: coordinatorUrl || coordinatorBaseUrl(),
        channels: channels || [],
        pid: process.pid,
      });
    });

    const parser = createNdjsonParser((msg) => {
      if (msg.type === 'register_ack') {
        if (!resolved) {
          resolved = true;
          clearTimeout(timeout);
          console.log(`[agent] registered with agentd: pane=${paneId}`);
          resolve(socket);
        }
      }
    });

    socket.on('data', parser);
  });
}

function setupAgentdListener(agentdSocket, tmuxTarget, tmuxPaneId, channels) {
  const parser = createNdjsonParser(async (msg) => {
    if (msg.type === 'hook_forward') {
      await handleHookForward(agentdSocket, msg, tmuxTarget, tmuxPaneId);
    }
  });

  agentdSocket.on('data', parser);
}

async function handleHookForward(agentdSocket, msg, tmuxTarget, tmuxPaneId) {
  const { requestId, hookType, paneId, cwd } = msg;
  try {
    // Detect Claude Code running status
    let claudeRunning = false;
    try {
      claudeRunning = detectClaudeCodeRunning(tmuxTarget, tmuxPaneId);
    } catch {
      claudeRunning = false;
    }

    const status = claudeRunning ? 'running' : 'idle';

    await heartbeat(status, '', {
      pane_id: tmuxPaneId,
      tmux_display: getPaneTarget(tmuxPaneId) || tmuxPaneId,
      claude_detected: claudeRunning,
    });

    await emitEvent('claude.hook', {
      hook: hookType,
      cwd,
      pane_id: tmuxPaneId,
      claude_detected: claudeRunning,
      ts: new Date().toISOString(),
    });

    let taskId = '';
    if (hookType === 'completed') {
      const current = await fetchCurrentTaskFromCoordinator();
      if (current && current.id) {
        const result = await completeTask(current.id);
        taskId = current.id;

        await emitEvent('task.completed', {
          task_id: current.id,
          pane_id: tmuxPaneId,
          ts: new Date().toISOString(),
        }, `task.completed:${current.id}`);

        // Post-completion heartbeat
        let finalClaudeRunning = false;
        try {
          finalClaudeRunning = detectClaudeCodeRunning(tmuxTarget, tmuxPaneId);
        } catch {
          finalClaudeRunning = false;
        }

        await heartbeat(finalClaudeRunning ? 'running' : 'idle', '', {
          pane_id: tmuxPaneId,
          tmux_display: getPaneTarget(tmuxPaneId) || tmuxPaneId,
          claude_detected: finalClaudeRunning,
        });
      }
    }

    sendMsg(agentdSocket, { type: 'hook_ack', requestId, success: true, taskId });
  } catch (err) {
    console.error(`[agent] hook_forward handling failed: ${err.message}`);
    sendMsg(agentdSocket, { type: 'hook_ack', requestId, success: false, error: err.message });
  }
}

function disconnectFromAgentd(agentdSocket, paneId) {
  if (!agentdSocket || agentdSocket.destroyed) return;
  try {
    sendMsg(agentdSocket, { type: 'unregister', paneId });
    agentdSocket.end();
  } catch {
    // ignore
  }
}

function readUserInput(promptText) {
  const readline = require('readline');
  const rl = readline.createInterface({
    input: process.stdin,
    output: process.stdout,
  });

  return new Promise((resolve) => {
    rl.question(promptText, (answer) => {
      rl.close();
      resolve(answer);
    });
  });
}

async function askYesNo(promptText) {
  const answer = await readUserInput(promptText);
  return /^y(es)?$/i.test(String(answer || '').trim());
}

async function fetchChannels() {
  try {
    const data = await getJson('/v1/channels');
    // API returns { channels: [...] }
    return Array.isArray(data?.channels) ? data.channels : (Array.isArray(data) ? data : []);
  } catch (err) {
    console.error(`[agent] Failed to fetch channels: ${String(err?.message || err)}`);
    return [];
  }
}

async function promptForWorkConfig() {
  const readline = require('readline');
  const rl = readline.createInterface({
    input: process.stdin,
    output: process.stdout,
  });

  const question = (prompt) => {
    return new Promise((resolve) => {
      rl.question(prompt, resolve);
    });
  };

  try {
    // Step 1: Channel selection
    console.log('\n--- Channel Selection ---');
    const channels = await fetchChannels();

    let selectedChannels = [];

    if (channels.length > 0) {
      console.log('Available channels:');
      channels.forEach((ch, i) => {
        const desc = ch.description ? ` - ${ch.description}` : '';
        console.log(`  ${i + 1}. ${ch.name}${desc}`);
      });
      console.log(`  0. Enter channel name manually`);

      const channelInput = await question('\nSelect channels (comma-separated numbers, e.g. 1,3): ');
      const selections = channelInput.split(',').map(s => s.trim()).filter(Boolean);

      for (const sel of selections) {
        const num = parseInt(sel, 10);
        if (num === 0) {
          const manualName = await question('Enter channel name: ');
          if (manualName.trim()) selectedChannels.push(manualName.trim());
        } else if (num >= 1 && num <= channels.length) {
          selectedChannels.push(channels[num - 1].name);
        } else {
          console.warn(`  Invalid selection: ${sel}`);
        }
      }
    } else {
      console.log('No channels found on server.');
      const manualName = await question('Enter channel name to subscribe: ');
      if (manualName.trim()) selectedChannels.push(manualName.trim());
    }

    if (selectedChannels.length === 0) {
      console.log('[agent] No channels selected. Channels can be assigned later via dashboard UI.');
    }

    // Deduplicate
    selectedChannels = [...new Set(selectedChannels)];
    console.log(`\nSelected channels: ${selectedChannels.join(', ')}`);

    // Step 2: tmux mode selection (reuse promptForRunMode pattern)
    console.log('\n--- Tmux Session Setup ---');
    console.log('How would you like to run the Claude Code session?');
    console.log('\n1. Automatic Mode (Recommended)');
    console.log('   - A new tmux session will be created in the background.');
    console.log('   - Claude Code will be started automatically.');
    console.log('\n2. Manual Mode');
    console.log('   - You start/manage the tmux session and Claude Code yourself.');
    console.log('   - The agent will connect to your existing session.');
    console.log('\n3. Skip (set up later)');
    console.log('   - No tmux session now. When a task arrives, you will be prompted.');
    console.log('   - Or use "request-session" from dashboard to trigger auto-setup.');
    console.log('---------------------------------');

    const choice = await question('Enter your choice (1, 2, or 3): ');
    let tmuxTarget = null;

    if (choice === '1') {
      const defaultSessionName = `claude-agent-session-${Date.now()}`;
      const sessionNameInput = await question(`Enter a name for the new tmux session (default: ${defaultSessionName}): `);
      const sessionName = sessionNameInput.trim() || defaultSessionName;
      createTmuxSessionAndStartClaudeCode(sessionName, 'claude');
      tmuxTarget = sessionName;
    } else if (choice === '2') {
      console.log('\nPlease start your tmux session and Claude Code now.');
      const manualTarget = await question('Enter the tmux target to connect to (e.g., session:window.pane or just session): ');
      if (!manualTarget.trim()) {
        console.error('Tmux target cannot be empty for Manual Mode.');
        return null;
      }
      tmuxTarget = manualTarget.trim();
    } else if (choice === '3') {
      console.log('[agent] Tmux setup skipped. Will be configured when a task arrives.');
      tmuxTarget = null;
    } else {
      console.error('Invalid choice. Please enter 1, 2, or 3.');
      return null;
    }

    return { channels: selectedChannels, tmuxTarget };
  } finally {
    rl.close();
  }
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

  if (res.statusCode === 200 && res.body && res.body.task) {
    console.log(`[agent] CompleteTask API: 200 OK - task ${taskId} marked as done`);
    return res.body.task;
  }

  if (res.statusCode === 409) {
    console.error(`[agent] CompleteTask API: 409 Conflict - task_id=${taskId}, agent_id=${agentId}`);
    console.error(`[agent] Coordinator rejected completion (current_task_id mismatch or task already done)`);
    console.error(`[agent] Response: ${res.raw}`);
    return null;
  }

  if (res.statusCode === 404) {
    console.error(`[agent] CompleteTask API: 404 Not Found - task_id=${taskId}`);
    console.error(`[agent] Task doesn't exist or agent not assigned`);
    return null;
  }

  console.error(`[agent] CompleteTask API: ${res.statusCode} - unexpected error`);
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

function createTmuxSessionAndStartClaudeCode(sessionName, command) {
  if (!sessionName || !command) {
    console.error('[auto-start] --session-name and --command are required.');
    emitEvent('agent.automation.error', { error: 'Session name or command not provided for auto-start.' }).catch(console.error);
    return;
  }

  const sessionExists = tmuxHasSession(sessionName);

  if (!sessionExists) {
    console.log(`[auto-start] Creating new detached tmux session: ${sessionName}`);
    const result = spawnSync('tmux', ['new-session', '-d', '-s', sessionName], {
      stdio: 'inherit',
      encoding: 'utf8',
    });
    if (result.status !== 0) {
      console.error(`[auto-start] Failed to create tmux session: ${sessionName}`);
      if (result.stderr) {
        console.error(result.stderr);
      }
      emitEvent('agent.tmux.session.create_failed', { session_name: sessionName, error: result.stderr || 'Unknown error' }).catch(console.error);
      return;
    }
    console.log(`[auto-start] Session ${sessionName} created.`);
    emitEvent('agent.tmux.session.created', { session_name: sessionName }).catch(console.error);
  } else {
    console.log(`[auto-start] Attaching to existing tmux session: ${sessionName}`);
    emitEvent('agent.tmux.session.existing', { session_name: sessionName }).catch(console.error);
  }

  console.log(`[auto-start] Sending command to session ${sessionName}: ${command}`);
  try {
    // Send the command to the session's first window (target: sessionName)
    // The command is sent with 'enter: true' to execute it.
    tmuxSend(sessionName, command, { enter: true });
    console.log('[auto-start] Command sent successfully.');
    emitEvent('agent.command.sent', { session_name: sessionName, command: command }).catch(console.error);
    console.log('\n--------------------------------------------------');
    console.log('CLAUDE CODE SESSION STARTED IN TMUX (BACKGROUND)');
    console.log(`To view and interact with the session, run:`);
    console.log(`tmux attach -t ${sessionName}`);
    console.log('--------------------------------------------------\n');
  } catch (err) {
    console.error(`[auto-start] Failed to send command to tmux session: ${err.message}`);
    emitEvent('agent.command.send_failed', { session_name: sessionName, command: command, error: err.message }).catch(console.error);
  }
}

async function promptForRunMode() {
  const readline = require('readline');
  const rl = readline.createInterface({
    input: process.stdin,
    output: process.stdout,
  });

  const question = (prompt) => {
    return new Promise((resolve) => {
      rl.question(prompt, resolve);
    });
  };

  try {
    console.log('\n--- Claude Code Agent Setup ---');
    console.log('How would you like to run the Claude Code session?');
    console.log('\n1. Automatic Mode (Recommended)');
    console.log('   - A new tmux session will be created in the background.');
    console.log('   - Claude Code will be started automatically.');
    console.log('\n2. Manual Mode');
    console.log('   - You start/manage the tmux session and Claude Code yourself.');
    console.log('   - The agent will connect to your existing session.');
    console.log('---------------------------------');

    const choice = await question('Enter your choice (1 or 2): ');
    let target = null;

    if (choice === '1') {
      const defaultSessionName = `claude-agent-session-${Date.now()}`;
      const sessionNameInput = await question(`Enter a name for the new tmux session (default: ${defaultSessionName}): `);
      const sessionName = sessionNameInput.trim() || defaultSessionName;
      const claudeCommand = 'claude'; // This could be made configurable.
      createTmuxSessionAndStartClaudeCode(sessionName, claudeCommand);
      target = sessionName;
    } else if (choice === '2') {
      console.log('\nPlease start your tmux session and Claude Code now.');
      const manualTarget = await question('Enter the tmux target to connect to (e.g., session:window.pane or just session): ');
      if (!manualTarget) {
        console.error('Tmux target cannot be empty for Manual Mode.');
      }
      target = manualTarget;
    } else {
      console.error('Invalid choice. Please enter 1 or 2.');
    }
    return target;
  } finally {
    rl.close();
  }
}

function formatTaskForInjection(task) {
  const title = String(task?.title || '').trim();
  const desc = String(task?.description || '').trim();
  const combined = desc ? `[TASK] ${title} — ${desc}` : `[TASK] ${title}`;
  return combined.replace(/\s+/g, ' ').trim();
}

function agentLogin() {
  const { exec } = require('child_process');
  const baseUrl = coordinatorBaseUrl();

  return new Promise((resolveLogin, rejectLogin) => {
    // Start a temporary HTTP server to receive the callback
    const server = http.createServer((req, res) => {
      const url = new URL(req.url, `http://localhost`);
      if (url.pathname === '/callback') {
        const code = url.searchParams.get('code');
        if (code) {
          res.writeHead(200, { 'Content-Type': 'text/html' });
          res.end('<html><body style="background:#0a0a0a;color:#6ee7b7;font-family:sans-serif;display:flex;justify-content:center;align-items:center;height:100vh"><h2>Agent authenticated successfully! You can close this tab.</h2></body></html>');
          exchangeCodeForToken(code).then(() => {
            server.close();
            resolveLogin();
          }).catch((err) => {
            console.error(`[agent] Failed to exchange code: ${err.message}`);
            server.close();
            rejectLogin(err);
          });
        } else {
          res.writeHead(400, { 'Content-Type': 'text/plain' });
          res.end('Missing code parameter');
        }
      } else {
        res.writeHead(404);
        res.end('Not found');
      }
    });

    let timeoutHandle;

    server.listen(0, '127.0.0.1', () => {
      const port = server.address().port;
      const authUrl = `${baseUrl}/agent-auth.html?callback_port=${port}&agent_name=${encodeURIComponent(agentName())}`;
      console.log(`[agent] Opening browser for authentication...`);
      console.log(`[agent] URL: ${authUrl}`);
      console.log(`[agent] Waiting for callback on port ${port}...`);

      // Open browser
      const platform = process.platform;
      if (platform === 'darwin') {
        exec(`open "${authUrl}"`);
      } else if (platform === 'linux') {
        exec(`xdg-open "${authUrl}" 2>/dev/null || sensible-browser "${authUrl}" 2>/dev/null`);
      } else if (platform === 'win32') {
        exec(`start "" "${authUrl}"`);
      } else {
        console.log(`[agent] Please open this URL in your browser: ${authUrl}`);
      }

      // Timeout after 5 minutes
      timeoutHandle = setTimeout(() => {
        console.error('[agent] Authentication timed out (5 minutes)');
        server.close();
        rejectLogin(new Error('Authentication timed out'));
      }, 5 * 60 * 1000);
    });
  });
}

async function exchangeCodeForToken(code) {
  const baseUrl = coordinatorBaseUrl();
  const url = new URL('/v1/auth/agent-token', baseUrl);

  return new Promise((resolve, reject) => {
    const payload = JSON.stringify({ code });
    const mod = url.protocol === 'https:' ? https : http;
    const req = mod.request(url, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Content-Length': Buffer.byteLength(payload),
      },
    }, (res) => {
      let body = '';
      res.on('data', (chunk) => { body += chunk; });
      res.on('end', () => {
        if (res.statusCode !== 200) {
          reject(new Error(`Server responded with ${res.statusCode}: ${body}`));
          return;
        }
        try {
          const data = JSON.parse(body);
          if (!data.token) {
            reject(new Error('No token in response'));
            return;
          }

          // Save the token to mode root (not instance dir)
          const dir = modeDataDir();
          if (!fs.existsSync(dir)) {
            fs.mkdirSync(dir, { recursive: true });
          }
          fs.writeFileSync(path.join(dir, 'agent-token.txt'), data.token + '\n', 'utf8');
          console.log(`[agent] Token saved to ${path.join(dir, 'agent-token.txt')}`);
          console.log(`[agent] Authenticated as user: ${data.username} (${data.user_id})`);
          resolve();
        } catch (e) {
          reject(new Error(`Failed to parse response: ${e.message}`));
        }
      });
    });
    req.on('error', reject);
    req.write(payload);
    req.end();
  });
}

async function startWorkLoop(channels, initialTarget) {
  // Persist coordinator URL so hooks can discover it (hooks run in separate shell without env vars)
  // Always write to mode root, not instance dir
  const persistDir = modeDataDir();
  ensureDir(persistDir);
  fs.writeFileSync(path.join(persistDir, 'coordinator-url.txt'), coordinatorBaseUrl() + '\n', 'utf8');

  // CRITICAL: Convert #S:#I.#P to pane ID immediately (pane ID is the stable identifier)
  let tmuxPaneId = initialTarget ? tmuxPaneIdForTarget(initialTarget) : '';

  // Deprecated: Keep tmuxTarget for backward compatibility (will be removed in future)
  // All connection logic should use tmuxPaneId instead
  let tmuxTarget = initialTarget || '';
  let tmuxSession = tmuxTargetSession(initialTarget);

  const pollSec = Math.max(2, parseInt(process.env.AGENT_WORK_POLL_INTERVAL_SEC || '2', 10) || 2);
  let rrIndex = 0;
  let lastPromptHash = '';
  let channelPollCounter = 0;
  const channelPollInterval = Math.max(1, Math.ceil(30 / pollSec)); // ~30 seconds

  let agentdSocket = null;

  // Cleanup agentd connection on shutdown
  const cleanupAgentd = () => {
    if (agentdSocket && tmuxPaneId) {
      disconnectFromAgentd(agentdSocket, tmuxPaneId);
      agentdSocket = null;
    }
  };
  process.on('SIGINT', cleanupAgentd);
  process.on('SIGTERM', cleanupAgentd);
  process.on('exit', cleanupAgentd);

  console.log(`[agent] worker started: channels=${channels.length ? channels.join(',') : '(none, waiting for server assignment)'} poll=${pollSec}s`);
  if (tmuxPaneId) {
    const state = configureStateDirForWork(tmuxPaneId);
    if (!(process.env.AGENT_NAME || '').trim() && state.label) {
      process.env.AGENT_NAME = `${os.hostname()}@${state.label}`;
    }
    console.log(`[agent] attached to initial pane: pane_id=${tmuxPaneId} (from target: ${initialTarget})`);

    // Register with agentd for IPC hook routing
    ensureAgentdRunning();
    agentdSocket = await connectToAgentd(tmuxPaneId, getOrCreateAgentId(), getDeployMode(), coordinatorBaseUrl(), channels);
    if (agentdSocket) {
      setupAgentdListener(agentdSocket, initialTarget, tmuxPaneId, channels);
      agentdSocket.on('close', () => { agentdSocket = null; });
      agentdSocket.on('error', () => { agentdSocket = null; });
    }
  } else if (initialTarget) {
    console.warn(`[agent] WARNING: Could not resolve pane ID from target: ${initialTarget}`);
    console.log('[agent] waiting for a task to configure session...');
  } else {
    console.log('[agent] waiting for a task to configure session...');
  }

  // Channel polling helper — called from both STATE 1 and STATE 2
  async function pollChannelsFromServer() {
    channelPollCounter++;
    const shouldPoll = channels.length === 0
      ? true  // poll every cycle when no channels
      : channelPollCounter % channelPollInterval === 0;

    if (!shouldPoll) return;

    try {
      const agentId = getOrCreateAgentId();
      const serverAgent = await getJson(`/v1/agents/${agentId}`);
      const serverSubs = Array.isArray(serverAgent?.agent?.meta?.subscriptions)
        ? serverAgent.agent.meta.subscriptions.filter(Boolean).sort()
        : [];
      const currentSorted = [...channels].sort();
      if (JSON.stringify(serverSubs) !== JSON.stringify(currentSorted) && serverSubs.length > 0) {
        const old = channels.join(',') || '(none)';
        channels = [...serverSubs];
        rrIndex = 0;
        console.log(`[agent] channels updated from server: [${old}] → [${channels.join(',')}]`);
      }
    } catch {
      // best-effort; ignore errors
    }
  }

  while (true) {
    // STATE 1: SETUP MODE (No tmux target)
    // ===================================
    // Only accepts request_claude_session tasks.
    // Regular tasks are left in the queue for other agents.
    // On receiving a session request: auto-creates tmux + claude code.
    if (!tmuxTarget) {
      // Poll for channel updates from server (UI may have assigned channels)
      await pollChannelsFromServer();

      // Look for request_claude_session tasks (don't use claimTask — it would grab any task)
      let sessionTask = null;
      if (channels.length > 0) {
        try {
          // Resolve channel names to IDs
          const channelData = await getJson('/v1/channels');
          const allChannels = channelData?.channels || [];
          const subscribedIds = new Set();
          for (const ch of allChannels) {
            if (channels.includes(ch.name)) subscribedIds.add(ch.id);
          }

          // Find queued request_claude_session tasks in our channels
          if (subscribedIds.size > 0) {
            const tasksData = await getJson('/v1/tasks?status=queued');
            const allTasks = tasksData?.tasks || [];
            sessionTask = allTasks.find(t =>
              t.type === 'request_claude_session' &&
              t.status === 'queued' &&
              subscribedIds.has(t.channel_id)
            );
          }
        } catch (err) {
          // best-effort; ignore and retry next cycle
        }
      }

      if (!sessionTask) {
        await heartbeat('idle', '', {
          subscriptions: channels,
          work_channels: channels,
          state: 'setup_waiting',
        });
        await sleep(pollSec * 1000);
        continue;
      }

      // Assign the session request task to self
      const agentId = getOrCreateAgentId();
      try {
        await postJson('/v1/tasks/assign', {
          task_id: sessionTask.id,
          agent_id: agentId,
          idempotency_key: `setup:${sessionTask.id}:${agentId}`,
        });
      } catch (err) {
        console.error(`[agent] failed to assign session request task: ${err.message}`);
        await sleep(pollSec * 1000);
        continue;
      }

      // Auto-create tmux session + start claude code
      const sessionName = `claude-agent-${Date.now()}`;
      console.log(`[agent] received request_claude_session, auto-creating tmux: ${sessionName}`);
      createTmuxSessionAndStartClaudeCode(sessionName, 'claude');

      // Wait briefly for tmux session to initialize
      await sleep(1000);

      // Resolve pane ID
      tmuxPaneId = tmuxPaneIdForTarget(sessionName);
      if (!tmuxPaneId) {
        console.error(`[agent] Failed to resolve pane ID from auto-created session: ${sessionName}`);
        await failTask(sessionTask.id, `Failed to resolve pane ID for session: ${sessionName}`).catch(console.error);
        await sleep(pollSec * 2000);
        continue;
      }

      tmuxTarget = sessionName;
      tmuxSession = tmuxTargetSession(sessionName);

      const state = configureStateDirForWork(tmuxPaneId);
      if (!(process.env.AGENT_NAME || '').trim() && state.label) {
        process.env.AGENT_NAME = `${os.hostname()}@${state.label}`;
      }

      console.log(`[agent] tmux session ready: pane_id=${tmuxPaneId} session=${sessionName}`);
      await emitEvent('agent.automation.target_set', { pane_id: tmuxPaneId, session_name: sessionName });

      // Register with agentd for IPC hook routing
      ensureAgentdRunning();
      agentdSocket = await connectToAgentd(tmuxPaneId, getOrCreateAgentId(), getDeployMode(), coordinatorBaseUrl(), channels);
      if (agentdSocket) {
        setupAgentdListener(agentdSocket, sessionName, tmuxPaneId, channels);
        agentdSocket.on('close', () => { agentdSocket = null; });
        agentdSocket.on('error', () => { agentdSocket = null; });
      }

      // Complete the session request task
      await completeTask(sessionTask.id);
      console.log(`[agent] request_claude_session completed, transitioning to STATE 2`);

      await sleep(pollSec * 1000);
      continue;
    }

    // STATE 2: MAIN WORKER MODE (tmuxTarget is set)
    // ============================================
    let inFlight = await fetchCurrentTaskFromCoordinator().catch(() => null);

    if (inFlight && inFlight.id) {
      const taskId = inFlight.id;
      let prompt = null;
      let claudeRunning = false;
      try {
        claudeRunning = detectClaudeCodeRunning(tmuxTarget, tmuxPaneId);
        if (claudeRunning) {
          const cap = tmuxCapture(tmuxTarget, 120, tmuxPaneId);
          prompt = detectInteractivePrompt(cap);
        }
      } catch {}

      const promptHash = prompt ? sha1hex(JSON.stringify(prompt)) : '';
      if (prompt && promptHash !== lastPromptHash) {
        lastPromptHash = promptHash;
        await emitEvent('task.prompt', { task_id: taskId, ...prompt }, `task.prompt:${taskId}:${promptHash}`, taskId);
      }
      if (!prompt) lastPromptHash = '';

      await heartbeat(claudeRunning ? 'running' : 'idle', taskId, {
        pane_id: tmuxPaneId, // PRIMARY: Stable pane identifier
        tmux_display: getPaneTarget(tmuxPaneId) || tmuxPaneId, // UI display only
        subscriptions: channels,
        work_channels: channels,
      });

    } else { // No task in flight, try to claim one
      let task = null;
      let claimedFromChannel = '';
      for (let i = 0; i < channels.length; i++) {
        const ch = channels[(rrIndex + i) % channels.length];
        try {
          const t = await claimTask(ch);
          if (t) {
            task = t;
            claimedFromChannel = ch;
            rrIndex = (rrIndex + i + 1) % channels.length;
            break;
          }
        } catch (err) {
          console.error(`[agent] claim error: ${String(err?.message || err)}`);
          await sleep(pollSec * 2000);
          break;
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
    // Poll for channel updates from server (UI may have changed channels)
    await pollChannelsFromServer();

    await sleep(pollSec * 1000);
  }
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

  if (cmd === 'agentd') {
    await startAgentd();
    return;
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

    // CRITICAL: Use pane ID as primary identifier (stable across pane rearrangements)
    const detectedPaneId = detectTmuxPaneId();
    const detectedTarget = detectTmuxTarget(); // Deprecated: only for state dir fallback

    // DEBUG: Log pane ID detection for hook
    process.stderr.write(`[agent] ═══ Hook Pane ID Detection ═══\n`);
    process.stderr.write(`[agent] Hook type: ${type}\n`);
    process.stderr.write(`[agent] $TMUX_PANE env: ${process.env.TMUX_PANE || 'NOT SET'}\n`);
    process.stderr.write(`[agent] detectTmuxPaneId(): ${detectedPaneId || 'NULL'}\n`);
    process.stderr.write(`[agent] detectTmuxTarget(): ${detectedTarget || 'NULL'}\n`);
    process.stderr.write(`[agent] Will use pane_id: ${detectedPaneId || 'FALLBACK TO TARGET'}\n`);

    const state = configureStateDirForHook(detectedPaneId || detectedTarget);
    if (!(process.env.AGENT_NAME || '').trim() && state.label) {
      process.env.AGENT_NAME = `${os.hostname()}@${state.label}`;
    }

    // 1) Preserve legacy behavior first (CWD/env needed by legacy hook).
    const exitCode = runLegacyHook(type);

    // 2) Best-effort coordinator upload via agentd IPC or direct fallback.
    try {
      let ipcHandled = false;
      if (detectedPaneId) {
        process.stderr.write(`[agent] Attempting agentd IPC for pane=${detectedPaneId}...\n`);
        ipcHandled = await tryHookViaAgentd(detectedPaneId, type);
        if (ipcHandled) {
          process.stderr.write(`[agent] Hook handled via agentd IPC\n`);
        } else {
          process.stderr.write(`[agent] agentd IPC not available, falling back to direct coordinator\n`);
        }
      }

      if (!ipcHandled) {
        await hookDirectCoordinator(type, detectedPaneId, detectedTarget);
      }
    } catch (err) {
      process.stderr.write(`[agent] Coordinator upload failed (ignored)\n`);
      process.stderr.write(`[agent] Error type: ${err?.constructor?.name || 'Unknown'}\n`);
      process.stderr.write(`[agent] Error message: ${String(err?.message || err)}\n`);
      if (err?.stack) {
        process.stderr.write(`[agent] Stack trace:\n${err.stack}\n`);
      }
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

  if (cmd === 'auto-start') {
    const sessionName = parseFlag(args, '--session-name');
    const command = parseFlag(args, '--command');
    createTmuxSessionAndStartClaudeCode(sessionName, command);
    return;
  }

  if (cmd === 'request-session') {
    const channelName = (parseFlag(args, '--channel') || '').trim();
    if (!channelName) {
      console.error('Error: --channel is a required argument for the "request-session" command.');
      usage();
      process.exit(1);
    }

    try {
      console.log(`[agent] looking up channel ID for name: "${channelName}"...`);
      const { channel } = await getJson(`/v1/channels/by-name/${encodeURIComponent(channelName)}`);

      if (!channel || !channel.id) {
        console.error(`Error: Channel "${channelName}" not found on coordinator.`);
        process.exit(1);
      }

      const channelId = channel.id;
      console.log(`[agent] found channel ID: ${channelId}`);

      console.log(`[agent] requesting new session for channel ID: ${channelId}...`);
      const { task } = await postJson('/v1/agents/request-session', {
        channel_id: channelId,
      });

      console.log('[agent] successfully created session request task.');
      console.log(`[agent] Task ID: ${task.id}`);
      process.exit(0);

    } catch (err) {
      console.error(`[agent] failed to request session: ${String(err?.message || err)}`);
      process.exit(2);
    }
  }

  if (cmd === 'login') {
    // Mode selection (if not yet configured)
    let mode = getDeployMode();
    if (!mode) {
      console.log('\n  Agent Mode Selection:');
      console.log('  1) local  - Connect to local coordinator (localhost)');
      console.log('  2) prod   - Connect to remote/deployed coordinator');
      const choice = await readUserInput('\n  Select mode (1 or 2): ');
      mode = choice.trim() === '2' ? 'prod' : 'local';
      setDeployMode(mode);
      console.log(`[agent] Mode set to: ${mode}`);
    } else {
      console.log(`[agent] Using mode: ${mode}`);
    }

    // For prod mode, prompt for COORDINATOR_URL if not set via env
    if (mode === 'prod' && !(process.env.COORDINATOR_URL || '').trim()) {
      const url = await readUserInput('  Coordinator URL: ');
      if (url.trim()) {
        const dir = modeDataDir();
        ensureDir(dir);
        fs.writeFileSync(path.join(dir, 'coordinator-url.txt'), url.trim() + '\n', 'utf8');
        console.log(`[agent] Coordinator URL saved to ${path.join(dir, 'coordinator-url.txt')}`);
      }
    }

    // For local mode, persist default localhost URL
    if (mode === 'local') {
      const dir = modeDataDir();
      ensureDir(dir);
      fs.writeFileSync(path.join(dir, 'coordinator-url.txt'), 'http://localhost:8080\n', 'utf8');
    }

    try {
      await agentLogin();
      console.log('[agent] Login successful.');
    } catch (err) {
      console.error(`[agent] Login failed: ${String(err?.message || err)}`);
      process.exit(2);
    }

    // After login, offer to start work immediately
    const proceed = await askYesNo('\nStart agent work now? (y/n): ');
    if (proceed) {
      const config = await promptForWorkConfig();
      if (config) {
        await startWorkLoop(config.channels, config.tmuxTarget);
      }
    }
    return;
  }

  if (cmd === 'work') {
    const channelArg = (parseFlag(args, '--channel') || '').trim();
    const initialTarget = (parseFlag(args, '--tmux-target') || parseFlag(args, '--tmux-session') || process.env.TMUX_TARGET || process.env.TMUX_SESSION || '').trim();

    if (!channelArg && !initialTarget) {
      // Interactive mode: no --channel and no --tmux-target flags
      const config = await promptForWorkConfig();
      if (config) {
        await startWorkLoop(config.channels, config.tmuxTarget);
      }
      return;
    }

    // Flag-based mode (backward compatible, channels may be empty)
    const channels = parseCommaList(channelArg);
    await startWorkLoop(channels, initialTarget);
    return;
  }

  usage();
  process.exit(1);
}

main().catch((err) => {
  console.error(String(err?.message || err));
  process.exit(1);
});
