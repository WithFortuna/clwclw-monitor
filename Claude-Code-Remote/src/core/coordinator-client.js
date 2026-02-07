/**
 * Coordinator Client (optional)
 * Best-effort uploader for agent heartbeat/events to the Go Coordinator API.
 *
 * - If COORDINATOR_URL is not set, this module is a no-op.
 * - Errors are swallowed to avoid breaking legacy behavior.
 */

const fs = require('fs');
const os = require('os');
const path = require('path');
const axios = require('axios');
const { v4: uuidv4 } = require('uuid');

class CoordinatorClient {
  constructor() {
    this.baseUrl = (process.env.COORDINATOR_URL || '').trim().replace(/\/+$/, '');
    this.authToken = (process.env.COORDINATOR_AUTH_TOKEN || '').trim();
    this.agentName = (process.env.AGENT_NAME || '').trim() || os.hostname();
    this.agentId = (process.env.AGENT_ID || '').trim();
    // Prefer shared agent identity (repo-level agent wrapper) if present:
    //   <repo>/agent/data/agent-id.txt
    // Fallback:
    //   Claude-Code-Remote/src/data/agent-id.txt
    const sharedPath = path.resolve(__dirname, '../../../agent/data/agent-id.txt');
    this.agentIdPath = fs.existsSync(sharedPath)
      ? sharedPath
      : path.join(__dirname, '../data/agent-id.txt');
  }

  enabled() {
    return !!this.baseUrl;
  }

  _headers() {
    const headers = { 'Content-Type': 'application/json' };
    if (this.authToken) {
      headers.Authorization = `Bearer ${this.authToken}`;
    }
    return headers;
  }

  _getOrCreateAgentId() {
    if (this.agentId) return this.agentId;

    try {
      if (fs.existsSync(this.agentIdPath)) {
        const id = fs.readFileSync(this.agentIdPath, 'utf8').trim();
        if (id) {
          this.agentId = id;
          return id;
        }
      }
    } catch {
      // ignore
    }

    try {
      const dir = path.dirname(this.agentIdPath);
      if (!fs.existsSync(dir)) fs.mkdirSync(dir, { recursive: true });
      const id = uuidv4();
      fs.writeFileSync(this.agentIdPath, id + '\n', 'utf8');
      this.agentId = id;
      return id;
    } catch {
      // Final fallback: ephemeral id (non-persistent)
      this.agentId = uuidv4();
      return this.agentId;
    }
  }

  async heartbeat(status = 'idle', currentTaskId = '', meta = {}) {
    if (!this.enabled()) return;

    const agentId = this._getOrCreateAgentId();
    const payload = {
      agent_id: agentId,
      name: this.agentName,
      status,
      current_task_id: currentTaskId,
      meta: {
        hostname: os.hostname(),
        platform: process.platform,
        pid: process.pid,
        cwd: process.cwd(),
        ...meta,
      },
    };

    await this._post('/v1/agents/heartbeat', payload);
  }

  async event(type, payload = {}, idempotencyKey = '') {
    if (!this.enabled()) return;

    const agentId = this._getOrCreateAgentId();
    await this._post('/v1/events', {
      agent_id: agentId,
      type,
      payload,
      idempotency_key: idempotencyKey,
    });
  }

  async _post(pathname, body) {
    try {
      await axios.post(`${this.baseUrl}${pathname}`, body, { headers: this._headers() });
    } catch (err) {
      // Never break legacy flows; log only when explicitly asked.
      if (process.env.LOG_LEVEL === 'debug') {
        const msg = err?.response?.data || err?.message || String(err);
        // eslint-disable-next-line no-console
        console.error('[coordinator] post failed:', msg);
      }
    }
  }
}

module.exports = CoordinatorClient;
