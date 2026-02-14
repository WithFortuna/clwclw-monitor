// --- Auth Guard ---
(function authGuard() {
  const token = localStorage.getItem('clw_jwt');
  if (!token) {
    window.location.href = '/';
    return;
  }
  // Async verify - redirect if token expired
  fetch('/v1/auth/verify', { headers: { 'Authorization': 'Bearer ' + token } })
    .then(r => { if (!r.ok) { localStorage.removeItem('clw_jwt'); localStorage.removeItem('clw_username'); window.location.href = '/'; } })
    .catch(() => {});
})();

const els = {
  refreshBtn: document.getElementById('refreshBtn'),
  autoRefresh: document.getElementById('autoRefresh'),
  userInfo: document.getElementById('userInfo'),
  logoutBtn: document.getElementById('logoutBtn'),
  agentsCount: document.getElementById('agentsCount'),
  lastRefresh: document.getElementById('lastRefresh'),
  agentsTbody: document.getElementById('agentsTbody'),
  taskBoard: document.getElementById('taskBoard'),
  eventsList: document.getElementById('eventsList'),
  promptModal: document.getElementById('promptModal'),
  promptCloseBtn: document.getElementById('promptCloseBtn'),
  promptMeta: document.getElementById('promptMeta'),
  promptSnippet: document.getElementById('promptSnippet'),
  promptOptions: document.getElementById('promptOptions'),
  promptControls: document.getElementById('promptControls'),
  promptInput: document.getElementById('promptInput'),
  promptSendBtn: document.getElementById('promptSendBtn'),
  promptStatus: document.getElementById('promptStatus'),
  channelName: document.getElementById('channelName'),
  channelDesc: document.getElementById('channelDesc'),
  createChannelBtn: document.getElementById('createChannelBtn'),
  taskChannel: document.getElementById('taskChannel'),
  taskChain: document.getElementById('taskChain'),
  taskTitle: document.getElementById('taskTitle'),
  taskDesc: document.getElementById('taskDesc'),
  taskExecutionMode: document.getElementById('taskExecutionMode'),
  createTaskBtn: document.getElementById('createTaskBtn'),
  claimAgentId: document.getElementById('claimAgentId'),
  claimChannel: document.getElementById('claimChannel'),
  claimTaskBtn: document.getElementById('claimTaskBtn'),
};

let lastTasksById = new Map();
let lastChainsById = new Map();
let lastAgentsById = new Map();
let lastPromptByTaskId = new Map();
let lastChannels = [];
let lastChains = [];
let lastAgents = [];
let promptModalState = { taskId: '', agentId: '', event: null };

// --- Notification State ---
let notifications = [];        // full list from server (panel)
let toasts = [];               // active toasts on screen
let unseenCount = 0;           // badge count
let isNotifPanelOpen = false;
let toastIdCounter = 0;

function getAuthToken() {
  return (localStorage.getItem('clw_jwt') || '').trim();
}

async function api(path, options = {}) {
  const token = getAuthToken();
  const res = await fetch(path, {
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { 'Authorization': 'Bearer ' + token } : {}),
    },
    ...options,
  });
  if (res.status === 401) {
    localStorage.removeItem('clw_jwt');
    localStorage.removeItem('clw_username');
    window.location.href = '/';
    return;
  }
  if (!res.ok) {
    let payload = null;
    let text = '';
    try {
      payload = await res.json();
    } catch {
      text = await res.text().catch(() => '');
    }
    const msg = payload?.error?.message || text || `request failed (${res.status})`;
    const err = new Error(msg);
    err.status = res.status;
    err.code = payload?.error?.code || '';
    err.payload = payload;
    throw err;
  }
  return res.json();
}

let refreshTimer = null;
function scheduleRefresh() {
  if (refreshTimer) return;
  refreshTimer = setTimeout(() => {
    refreshTimer = null;
    refresh().catch(() => {});
  }, 200);
}

let stream = null;
function startStream() {
  if (stream) stream.close();
  const token = getAuthToken();
  const url = token ? `/v1/stream?token=${encodeURIComponent(token)}` : '/v1/stream';
  stream = new EventSource(url);
  stream.addEventListener('update', scheduleRefresh);
  stream.addEventListener('notification', handleNotificationEvent);
  stream.addEventListener('hello', () => {});
  stream.onerror = () => {
    // Keep the UI usable even if SSE is unavailable; polling remains as fallback.
  };
}

function fmtTime(ts) {
  if (!ts) return '-';
  try {
    return new Date(ts).toLocaleString();
  } catch {
    return ts;
  }
}

function statusBadge(status) {
  const s = String(status || '').toLowerCase();
  let cls = 'badge';
  if (s.includes('idle')) cls += ' ok';
  else if (s.includes('wait')) cls += ' warn';
  else if (s.includes('run')) cls += ' ok';
  else cls += ' warn';
  return `<span class="${cls}">${escapeHtml(status || '-')}</span>`;
}

function deriveWorkerStatus(lastSeen) {
  const threshold = 30 * 1000; // 30 seconds
  const age = Date.now() - new Date(lastSeen).getTime();
  return age < threshold ? 'online' : 'offline';
}

function workerStatusBadge(lastSeen) {
  const status = deriveWorkerStatus(lastSeen);
  const cls = status === 'online' ? 'badge ok' : 'badge err';
  return `<span class="${cls}">${status}</span>`;
}

function claudeStatusBadge(claudeStatus) {
  const s = String(claudeStatus || 'idle').toLowerCase();
  let cls = 'badge';
  let displayStatus = claudeStatus || 'idle'; // Default display

  if (s === 'idle' || s === 'not running' || !s) { // If status is idle, not running, or empty
    cls += ' not-running';
    displayStatus = 'not running';
  } else if (s === 'waiting') {
    cls += ' warn';
  } else if (s === 'running') {
    cls += ' ok';
  } else if (s === 'queued') {
    cls += ' muted-badge';
  } else if (s === 'in_progress') {
    cls += ' ok';
  } else if (s === 'done') {
    cls += ' success';
  } else if (s === 'failed') {
    cls += ' err';
  } else {
    cls += ' muted-badge';
  }
  return `<span class="${cls}">${escapeHtml(displayStatus)}</span>`;
}

function escapeHtml(str) {
  return String(str || '')
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;');
}

function normalizeSubs(subs) {
  if (!Array.isArray(subs)) return [];
  return subs.map((s) => String(s || '').trim()).filter(Boolean);
}

function renderAgents(agents) {
  // 인라인 편집 input이 실제 포커스 상태일 때만 리렌더링 스킵
  const activeSubsInput = els.agentsTbody.querySelector('.subs-cell input');
  if (activeSubsInput && document.activeElement === activeSubsInput) return;

  els.agentsCount.textContent = String(agents.length);

  if (!agents.length) {
    els.agentsTbody.innerHTML = `<tr><td colspan="7" class="muted">No agents yet.</td></tr>`;
    return;
  }

  els.agentsTbody.innerHTML = agents
    .map((a) => {
      const name = a.name || a.id || '-';
      const subs = Array.isArray(a?.meta?.subscriptions) ? a.meta.subscriptions.join(', ') : '';
      // Display tmux info: prefer tmux_display (dynamically resolved #S:#I.#P from pane_id)
      const tmux = a?.meta?.tmux_display || a?.meta?.pane_id || a?.meta?.tmux_target || '';
      const workerStatus = a.worker_status ? a.worker_status : deriveWorkerStatus(a.last_seen);
      const claudeStatus = a.claude_status || a.status || 'idle';
      const agentState = a?.meta?.state || '';
      const isSetupWaiting = agentState === 'setup_waiting' && !tmux;
      const hasSubs = Array.isArray(a?.meta?.subscriptions) && a.meta.subscriptions.length > 0;

      // Tmux cell: show state + start session button if applicable
      let tmuxCell;
      if (isSetupWaiting) {
        if (hasSubs) {
          tmuxCell = `<span class="badge warn" style="margin-right:6px;">waiting</span>`
            + `<button class="btn primary" style="padding:4px 8px;font-size:11px;" `
            + `data-action="request-session" data-agent-id="${escapeHtml(a.id)}" `
            + `data-channel="${escapeHtml(a.meta.subscriptions[0])}">Start Session</button>`;
        } else {
          tmuxCell = `<span class="badge warn">waiting</span> `
            + `<span class="muted" style="font-size:11px;">assign channels first</span>`;
        }
      } else {
        tmuxCell = escapeHtml(tmux) || '<span class="muted" style="opacity:0.4">-</span>';
      }

      return `<tr>
        <td>${escapeHtml(name)}</td>
        <td>${workerStatusBadge(a.last_seen)}</td>
        <td>${claudeStatusBadge(claudeStatus)}</td>
        <td class="muted subs-cell" data-agent-id="${escapeHtml(a.id)}" data-subs="${escapeHtml(subs)}" title="Click to edit"
            style="cursor:pointer;">${escapeHtml(subs) || '<span class="muted" style="opacity:0.5">click to assign</span>'}</td>
        <td class="muted">${tmuxCell}</td>
        <td class="muted">${escapeHtml(fmtTime(a.last_seen))}</td>
        <td class="muted">${escapeHtml(a.current_task_id || '')}</td>
      </tr>`;
    })
    .join('');
}

function renderTaskBoard(channels, chains, tasks, agents) {
  if (!channels.length && !chains.length && !tasks.length) {
    els.taskBoard.innerHTML = `<div class="muted">No channels, chains, or tasks yet.</div>`;
    return;
  }

  // Index chains by channel
  const chainsByChannel = new Map();
  for (const ch of chains) {
    const cid = ch.channel_id || 'unknown';
    if (!chainsByChannel.has(cid)) chainsByChannel.set(cid, []);
    chainsByChannel.get(cid).push(ch);
  }

  // Index tasks by chain (all tasks must belong to a chain)
  const tasksByChain = new Map();
  for (const t of tasks) {
    const chainId = t.chain_id || 'unknown';
    if (!tasksByChain.has(chainId)) tasksByChain.set(chainId, []);
    tasksByChain.get(chainId).push(t);
  }

  const output = [];

  for (const channel of channels) {
    const channelChains = chainsByChannel.get(channel.id) || [];
    const totalTasks = channelChains.reduce((sum, ch) => sum + (tasksByChain.get(ch.id) || []).length, 0);

    let inner = '';

    // Render each chain within this channel (all tasks belong to a chain)
    for (const ch of channelChains) {
      const list = tasksByChain.get(ch.id) || [];
      if (!list.length) continue;
      const queued = list.filter((t) => t.status === 'queued').sort((a, b) => a.sequence - b.sequence);
      const locked = list.filter((t) => t.status === 'locked').sort((a, b) => a.sequence - b.sequence);
      const prog = list.filter((t) => t.status === 'in_progress' || t.status === 'locked').sort((a, b) => a.sequence - b.sequence);
      const done = list.filter((t) => t.status === 'done').sort((a, b) => a.sequence - b.sequence);
      const failed = list.filter((t) => t.status === 'failed').sort((a, b) => a.sequence - b.sequence);

      // Owner agent info
      const ownerAgent = ch.owner_agent_id ? lastAgentsById.get(ch.owner_agent_id) : null;
      const ownerName = ownerAgent ? (ownerAgent.name || ownerAgent.id) : '';
      const isLocked = ch.status === 'locked' || locked.length > 0;
      const hasOwner = !!ch.owner_agent_id;

      // Build agent assignment dropdown (online agents)
      const onlineAgents = agents.filter(a => {
        const ws = a.worker_status || deriveWorkerStatus(a.last_seen);
        return ws === 'online';
      });
      let assignDropdown = '';
      if (!hasOwner || isLocked) {
        const opts = onlineAgents.map(a =>
          `<option value="${escapeHtml(a.id)}">${escapeHtml(a.name || a.id)}</option>`
        ).join('');
        assignDropdown = `<select class="agent-assign-dropdown" data-chain-id="${escapeHtml(ch.id)}" data-channel-name="${escapeHtml(channel.name)}">
          <option value="">Assign agent…</option>${opts}
        </select>`;
      }

      // Owner display with detach button
      let ownerHtml = '';
      if (hasOwner) {
        ownerHtml = `<span class="chain-owner">${escapeHtml(ownerName)}</span>
          <button class="detach-btn" data-action="detach" data-chain-id="${escapeHtml(ch.id)}" data-agent-id="${escapeHtml(ch.owner_agent_id)}" title="Detach agent">&times;</button>`;
      } else {
        ownerHtml = `<span class="muted" style="font-size:11px;">No agent</span>`;
      }

      const chainClass = isLocked ? 'chain-group chain-locked' : 'chain-group';

      inner += `
        <div class="${chainClass}">
          <div class="chain-title">
            <div class="chain-badge"><strong>Chain</strong>: ${escapeHtml(ch.name)} ${claudeStatusBadge(ch.status)} ${ownerHtml}</div>
            <div style="display:flex;align-items:center;gap:8px;">${assignDropdown}<span class="pill">${list.length} tasks</span></div>
          </div>
          <div class="board">
            ${renderTaskCol('Queued', queued, { variant: 'queued' })}
            ${renderTaskCol('In Progress', prog, { variant: 'in_progress' })}
            ${renderTaskCol('Done', done, { variant: 'done' })}
            ${renderTaskCol('Failed', failed, { variant: 'failed' })}
          </div>
        </div>
      `;
    }

    if (!inner) {
      inner = `<div class="muted" style="padding:8px;">No tasks in this channel.</div>`;
    }

    output.push(`
      <div class="channel-group">
        <div class="col-title">
          <div>Channel: ${escapeHtml(channel.name)}</div>
          <span class="pill">${totalTasks} tasks</span>
        </div>
        ${inner}
      </div>
    `);
  }

  els.taskBoard.innerHTML = output.join('');
}

function renderTaskCol(title, tasks, opts = {}) {
  const variant = opts.variant || '';
  const items = tasks
    .map(
      (t) => {
        const isLocked = t.status === 'locked';
        const taskClass = isLocked ? 'task task-locked' : `task task-${variant}`;

        let actions = '';
        if (isLocked) {
          actions = `<div style="margin-top:10px;display:flex;gap:10px;align-items:center;">
            <button class="btn" data-action="task-status" data-task-id="${escapeHtml(t.id)}" data-status="queued">→ Queued</button>
            <button class="btn" data-action="task-status" data-task-id="${escapeHtml(t.id)}" data-status="done">→ Done</button>
            <div class="muted" style="font-size:11px;">locked</div>
          </div>`;
        } else if (variant === 'queued') {
          actions = `<div style="margin-top:10px;display:flex;gap:10px;align-items:center;">
            <button class="btn" data-action="assign" data-task-id="${escapeHtml(t.id)}">Assign…</button>
          </div>`;
        } else if (variant === 'in_progress') {
          actions = `<div style="margin-top:10px;display:flex;gap:10px;align-items:center;">
            ${lastPromptByTaskId.has(t.id)
              ? `<button class="btn" data-action="prompt" data-task-id="${escapeHtml(t.id)}">Prompt…</button>`
              : ''}
            <button class="btn" data-action="complete" data-task-id="${escapeHtml(t.id)}">Mark Done</button>
            <button class="btn danger" data-action="fail" data-task-id="${escapeHtml(t.id)}">Fail</button>
            <div class="muted" style="font-size:11px;">agent: ${escapeHtml(t.assigned_agent_id || '')}</div>
          </div>`;
        }

        return `
        <div class="${taskClass}">
          <div class="task-title">${escapeHtml(t.title)}</div>
          <div class="task-desc">${escapeHtml(t.description || '')}</div>
          ${actions}
          <div class="muted" style="margin-top:6px;font-size:11px;">
            ${t.chain_id ? `chain: ${escapeHtml(t.chain_id)} seq: ${t.sequence}<br>` : ''}
            ${escapeHtml(t.id)}
          </div>
        </div>
      `;
      },
    )
    .join('');

  return `
    <div class="col col-${variant}">
      <div class="col-title">
        <div>${escapeHtml(title)}</div>
        <span class="pill">${tasks.length}</span>
      </div>
      ${items || `<div class="muted">Empty</div>`}
    </div>
  `;
}

function renderEvents(events) {
  if (!events.length) {
    els.eventsList.innerHTML = `<div class="muted">No events yet.</div>`;
    return;
  }

  els.eventsList.innerHTML = events
    .slice(0, 80)
    .map((e) => {
      const payload = e.payload ? JSON.stringify(e.payload, null, 2) : '';
      return `
      <div class="event">
        <div class="event-top">
          <div class="event-type">${escapeHtml(e.type)}</div>
          <div class="event-time">${escapeHtml(fmtTime(e.created_at))}</div>
        </div>
        <div class="event-meta">${escapeHtml(payload)}</div>
      </div>
    `;
    })
    .join('');
}

function fillChannelSelect(selectEl, channels) {
  const prev = selectEl.value;
  selectEl.innerHTML =
    `<option value="">Select channel</option>` +
    channels
      .map((c) => `<option value="${escapeHtml(c.id)}">${escapeHtml(c.name)}</option>`)
      .join('');
  if (prev && Array.from(selectEl.options).some((o) => o.value === prev)) {
    selectEl.value = prev;
  }
}

function fillChainSelect(selectEl, chains, channelId) {
  const prev = selectEl.value;
  const filtered = chains.filter((c) => c.channel_id === channelId);
  selectEl.innerHTML =
    `<option value="">New Chain (auto)</option>` +
    filtered
      .map((c) => `<option value="${escapeHtml(c.id)}">${escapeHtml(c.name)} (${c.status})</option>`)
      .join('');
  if (prev && Array.from(selectEl.options).some((o) => o.value === prev)) {
    selectEl.value = prev;
  }
}

function computeLatestPrompts(events) {
  const m = new Map();
  for (const e of events || []) {
    if (e.type !== 'task.prompt') continue;
    if (!e.task_id) continue;
    if (m.has(e.task_id)) continue; // already have latest (events are sorted desc)
    m.set(e.task_id, e);
  }
  return m;
}

function normalizeOptKey(v) {
  return String(v ?? '').trim();
}

function computeArrowSelectKeys(payload, targetKey) {
  const opts = Array.isArray(payload.options) ? payload.options : [];
  if (!opts.length) return null;

  const target = normalizeOptKey(targetKey);
  const targetIdx = opts.findIndex((o) => normalizeOptKey(o?.key) === target);
  if (targetIdx < 0) return null;

  let selectedIdx = Number.isFinite(payload.selected_index) ? payload.selected_index : -1;
  if (selectedIdx < 0) {
    const selectedKey = normalizeOptKey(payload.selected_key) || normalizeOptKey(opts[0]?.key);
    selectedIdx = opts.findIndex((o) => normalizeOptKey(o?.key) === selectedKey);
  }
  if (selectedIdx < 0) selectedIdx = 0;

  const delta = targetIdx - selectedIdx;
  const keys = [];
  if (delta > 0) for (let i = 0; i < delta; i++) keys.push('Down');
  if (delta < 0) for (let i = 0; i < -delta; i++) keys.push('Up');
  keys.push('Enter');
  return keys;
}

function openPromptModal(taskId) {
  const task = lastTasksById.get(taskId);
  const ev = lastPromptByTaskId.get(taskId);
  if (!task || !ev) {
    alert('No prompt found for this task.');
    return;
  }
  if (!task.assigned_agent_id) {
    alert('Task has no assigned_agent_id (in_progress agent missing).');
    return;
  }

  promptModalState = { taskId, agentId: task.assigned_agent_id, event: ev };

  const payload = ev.payload || {};
  const promptText = payload.prompt || '(no prompt text)';
  const kind = payload.kind || 'unknown';
  const inputMode = payload.input_mode || 'number';
  const tmux = payload.tmux_target || payload.tmux_session || '';
  els.promptMeta.textContent = `task=${taskId} agent=${task.assigned_agent_id} kind=${kind} mode=${inputMode} tmux=${tmux}`;
  els.promptSnippet.textContent = payload.snippet || JSON.stringify(payload, null, 2);

  const opts = Array.isArray(payload.options) ? payload.options : [];
  const selectedKey = normalizeOptKey(payload.selected_key);
  els.promptOptions.innerHTML = opts
    .map((o) => {
      const key = String(o?.key || '').trim();
      const label = String(o?.label || '').trim();
      const text = label ? `${key}. ${label}` : key;
      const cls = `btn${selectedKey && selectedKey === key ? ' selected' : ''}`;
      return `<button class="${cls}" data-action="prompt-option" data-key="${escapeHtml(key)}">${escapeHtml(text)}</button>`;
    })
    .join('');

  if (els.promptControls) {
    const controls = [
      { key: 'Up', label: 'Up' },
      { key: 'Down', label: 'Down' },
      { key: 'Tab', label: 'Tab' },
      { key: 'Enter', label: 'Enter' },
      { key: 'Escape', label: 'Esc' },
    ];
    els.promptControls.innerHTML = controls
      .map((c) => `<button class="btn" data-action="prompt-key" data-key="${escapeHtml(c.key)}">${escapeHtml(c.label)}</button>`)
      .join('');
  }

  els.promptInput.value = '';
  els.promptInput.placeholder =
    inputMode === 'arrows'
      ? 'Type response (or use Up/Down/Enter/Esc above)'
      : promptText
        ? `${promptText} (type response)`
        : 'Type response';
  els.promptStatus.textContent = '';

  els.promptModal.classList.remove('hidden');
  els.promptInput.focus();
}

function closePromptModal() {
  els.promptModal.classList.add('hidden');
  promptModalState = { taskId: '', agentId: '', event: null };
  els.promptStatus.textContent = '';
}

async function sendTaskInput(kind, text, sendEnter = true, opts = {}) {
  const taskId = promptModalState.taskId;
  const agentId = promptModalState.agentId;
  const ev = promptModalState.event;
  if (!taskId || !agentId || !ev) return;

  const inputKind = String(kind || 'text').trim() || 'text';
  const normalizedText = String(text || '');
  const enter = !!sendEnter;
  const marker = (opts && opts.marker) || '';
  const idem = `ui.input:${taskId}:${ev.id}:${marker || inputKind}:${normalizedText || (enter ? 'ENTER' : 'NOENTER')}`;

  els.promptStatus.textContent = 'Sending...';
  await api('/v1/tasks/inputs', {
    method: 'POST',
    body: JSON.stringify({
      task_id: taskId,
      agent_id: agentId,
      kind: inputKind,
      text: normalizedText,
      send_enter: enter,
      idempotency_key: idem,
    }),
  });
  els.promptStatus.textContent = 'Sent.';
}

async function sendPromptKeys(keys, opts = {}) {
  const list = Array.isArray(keys) ? keys : [keys];
  const text = list
    .map((k) => String(k || '').trim())
    .filter((k) => k.length > 0)
    .join('\n');
  if (!text) return;
  await sendTaskInput('keys', text, false, opts);
}

async function refresh() {
  const dash = await api('/v1/dashboard');
  const agents = dash.agents || [];
  const channels = dash.channels || [];
  const chains = dash.chains || []; // Extract chains data
  const tasks = dash.tasks || [];
  const events = dash.events || [];

  lastTasksById = new Map(tasks.map((t) => [t.id, t]));
  lastChainsById = new Map(chains.map((c) => [c.id, c]));
  lastAgentsById = new Map(agents.map((a) => [a.id, a]));
  lastPromptByTaskId = computeLatestPrompts(events);

  lastChannels = channels;
  lastChains = chains;
  lastAgents = agents;

  els.lastRefresh.textContent = fmtTime(new Date().toISOString());
  renderAgents(agents);
  fillChannelSelect(els.taskChannel, channels);
  fillChannelSelect(els.claimChannel, channels);
  fillChainSelect(els.taskChain, chains, els.taskChannel.value);
  renderTaskBoard(channels, chains, tasks, agents);
  renderEvents(events);
}

// --- Notification Functions ---

// Fetch all notifications from server and render in panel
async function fetchNotifications() {
  try {
    const data = await api('/v1/notifications');
    notifications = (data.notifications || []).map(n => ({
      key: n.key,
      agentId: n.agent_id,
      agentName: n.agent_name || n.agent_id,
      type: n.type,
      channel: n.channel || '',
      message: n.message || '',
      time: new Date(n.created_at),
    }));
    renderNotifPanel();
  } catch { /* ignore */ }
}

// SSE push → show toast + bump unseen badge
function handleNotificationEvent(e) {
  try {
    const data = JSON.parse(e.data);
    const payload = data.payload || {};
    const agentId = payload.agent_id || '';
    const type = payload.notification_type || '';
    if (!agentId || !type) return;

    // Show toast
    addToast({
      agentId,
      agentName: payload.agent_name || agentId,
      type,
      channel: payload.channel || '',
      message: payload.message || '',
    });

    // Increment unseen badge
    unseenCount++;
    renderBellBadge();
  } catch { /* ignore */ }
}

// --- Toast (floating popup, top-right) ---

function addToast(data) {
  const id = ++toastIdCounter;
  toasts.push({ id, ...data });
  renderToasts();
  // Auto-dismiss after 15 seconds
  setTimeout(() => removeToast(id), 15000);
}

function removeToast(id) {
  toasts = toasts.filter(t => t.id !== id);
  renderToasts();
}

function renderToasts() {
  const container = document.getElementById('toastContainer');
  if (!container) return;

  container.innerHTML = toasts.map(t => {
    const hasChannel = !!t.channel;
    const actionBtn = hasChannel
      ? `<button class="btn primary toast-action-btn" data-toast-action="start"
           data-agent-id="${escapeHtml(t.agentId)}" data-channel="${escapeHtml(t.channel)}"
           data-toast-id="${t.id}">Start Session</button>`
      : `<span class="muted" style="font-size:11px;">Assign channel first</span>`;
    return `
      <div class="toast-item" data-toast-id="${t.id}">
        <div class="toast-content">
          <div class="toast-title">Agent Setup Required</div>
          <div class="toast-msg">${escapeHtml(t.message)}</div>
          <div class="toast-actions">${actionBtn}</div>
        </div>
        <button class="toast-close" data-toast-dismiss="${t.id}">&times;</button>
      </div>`;
  }).join('');
}

// --- Bell Badge ---

function renderBellBadge() {
  const badge = document.getElementById('notifBadge');
  if (!badge) return;
  badge.textContent = String(unseenCount);
  badge.style.display = unseenCount > 0 ? 'flex' : 'none';

  const bell = document.getElementById('notifBell');
  if (bell) bell.classList.toggle('has-notif', unseenCount > 0);
}

// --- Notification Panel (all history) ---

function renderNotifPanel() {
  const list = document.getElementById('notifList');
  if (!list) return;

  if (!notifications.length) {
    list.innerHTML = '<div class="muted" style="padding:12px;text-align:center;">No notifications</div>';
    return;
  }

  // Show newest first
  const sorted = [...notifications].reverse();
  list.innerHTML = sorted.map(n => {
    const timeStr = fmtTime(n.time.toISOString());
    return `
      <div class="notification-item">
        <div class="notification-item-header">
          <span class="notification-item-title">Agent Setup Required</span>
          <span class="notification-item-time">${escapeHtml(timeStr)}</span>
        </div>
        <div class="notification-item-msg">${escapeHtml(n.message)}</div>
      </div>`;
  }).join('');
}

function toggleNotifPanel() {
  isNotifPanelOpen = !isNotifPanelOpen;
  const panel = document.getElementById('notifPanel');
  if (panel) panel.classList.toggle('hidden', !isNotifPanelOpen);

  if (isNotifPanelOpen) {
    // Mark all as seen
    unseenCount = 0;
    renderBellBadge();
    fetchNotifications();
  }
}

async function main() {
  els.refreshBtn.addEventListener('click', () => refresh().catch(showError));

  // Display username
  const username = localStorage.getItem('clw_username') || '';
  if (els.userInfo) {
    els.userInfo.textContent = username ? `@${username}` : '';
  }

  // Logout
  if (els.logoutBtn) {
    els.logoutBtn.addEventListener('click', () => {
      localStorage.removeItem('clw_jwt');
      localStorage.removeItem('clw_username');
      window.location.href = '/';
    });
  }

  els.taskBoard.addEventListener('click', async (ev) => {
    const btn = ev.target?.closest?.('button[data-action]');
    if (!btn) return;

    const action = btn.getAttribute('data-action') || '';
    const task_id = btn.getAttribute('data-task-id');

    try {
      if (action === 'prompt') {
        openPromptModal(task_id);
        return;
      }
      if (action === 'detach') {
        const chainId = btn.getAttribute('data-chain-id');
        const agentId = btn.getAttribute('data-agent-id');
        if (!chainId || !agentId) return;
        await api(`/v1/chains/${encodeURIComponent(chainId)}/detach`, {
          method: 'POST',
          body: JSON.stringify({ agent_id: agentId }),
        });
      } else if (action === 'task-status') {
        const status = btn.getAttribute('data-status');
        if (!task_id || !status) return;
        await api(`/v1/tasks/${encodeURIComponent(task_id)}/status`, {
          method: 'POST',
          body: JSON.stringify({ status }),
        });
      } else if (action === 'complete') {
        await api('/v1/tasks/complete', {
          method: 'POST',
          body: JSON.stringify({ task_id }),
        });
      } else if (action === 'fail') {
        await api('/v1/tasks/fail', {
          method: 'POST',
          body: JSON.stringify({ task_id }),
        });
      } else if (action === 'assign') {
        const agent_id = prompt('Assign to agent_id (uuid)');
        if (!agent_id) return;
        await api('/v1/tasks/assign', {
          method: 'POST',
          body: JSON.stringify({ task_id, agent_id }),
        });
      }
      await refresh();
    } catch (err) {
      showError(err);
    }
  });

  // Agent assignment dropdown on chain cards
  els.taskBoard.addEventListener('change', async (ev) => {
    const dropdown = ev.target?.closest?.('.agent-assign-dropdown');
    if (!dropdown) return;
    const chainId = dropdown.getAttribute('data-chain-id');
    const channelName = (dropdown.getAttribute('data-channel-name') || '').trim();
    const agentId = dropdown.value;
    if (!chainId || !agentId) return;
    try {
      await api(`/v1/chains/${encodeURIComponent(chainId)}/assign-agent`, {
        method: 'POST',
        body: JSON.stringify({ agent_id: agentId }),
      });
      await refresh();
    } catch (err) {
      if (err?.code === 'agent_not_subscribed_channel' && channelName) {
        const shouldSubscribe = window.confirm(
          `이 에이전트는 '${channelName}' 채널을 구독하지 않습니다.\n지금 구독에 추가하고 체인 할당을 다시 시도할까요?`,
        );
        if (!shouldSubscribe) {
          dropdown.value = '';
          return;
        }

        const agent = lastAgentsById.get(agentId);
        const currentSubs = normalizeSubs(agent?.meta?.subscriptions);
        const exists = currentSubs.some((s) => s.toLowerCase() === channelName.toLowerCase());
        const nextSubs = exists ? currentSubs : [...currentSubs, channelName];

        try {
          await api(`/v1/agents/${encodeURIComponent(agentId)}/channels`, {
            method: 'PATCH',
            body: JSON.stringify({ subscriptions: nextSubs }),
          });
          await api(`/v1/chains/${encodeURIComponent(chainId)}/assign-agent`, {
            method: 'POST',
            body: JSON.stringify({ agent_id: agentId }),
          });
          await refresh();
          return;
        } catch (retryErr) {
          showError(retryErr);
          return;
        }
      }
      showError(err);
    }
  });

  if (els.promptModal) {
    els.promptModal.addEventListener('click', async (ev) => {
      const close = ev.target?.closest?.('[data-action="close-modal"]');
      if (close) closePromptModal();

      const optBtn = ev.target?.closest?.('button[data-action="prompt-option"]');
      if (optBtn) {
        const key = optBtn.getAttribute('data-key') || '';
        try {
          const payload = promptModalState.event?.payload || {};
          const mode = payload.input_mode || 'number';
          if (mode === 'arrows') {
            const keys = computeArrowSelectKeys(payload, key);
            if (keys) await sendPromptKeys(keys, { marker: `opt:${key}` });
            else await sendTaskInput('text', key, true, { marker: `opt:${key}:fallback` });
          } else if (key === 'Enter') {
            await sendTaskInput('text', '', true, { marker: 'Enter' });
          } else {
            els.promptInput.value = key;
            await sendTaskInput('text', key, true, { marker: `opt:${key}` });
          }
        } catch (err) {
          els.promptStatus.textContent = `Error: ${String(err?.message || err)}`;
        }
      }

      const keyBtn = ev.target?.closest?.('button[data-action="prompt-key"]');
      if (keyBtn) {
        const key = keyBtn.getAttribute('data-key') || '';
        try {
          await sendPromptKeys([key], { marker: `key:${key}` });
        } catch (err) {
          els.promptStatus.textContent = `Error: ${String(err?.message || err)}`;
        }
      }
    });
  }
  if (els.promptCloseBtn) {
    els.promptCloseBtn.addEventListener('click', () => closePromptModal());
  }
  if (els.promptSendBtn) {
    els.promptSendBtn.addEventListener('click', async () => {
      const v = (els.promptInput.value || '').trim();
      try {
        await sendTaskInput('text', v, true, { marker: 'freeform' });
      } catch (err) {
        els.promptStatus.textContent = `Error: ${String(err?.message || err)}`;
      }
    });
  }

  // Start Session button for agents in setup_waiting state
  els.agentsTbody.addEventListener('click', async (ev) => {
    const btn = ev.target?.closest?.('button[data-action="request-session"]');
    if (!btn) return;

    const channelName = btn.getAttribute('data-channel') || '';
    if (!channelName) {
      showError(new Error('No channel assigned to this agent'));
      return;
    }

    btn.disabled = true;
    btn.textContent = 'Requesting...';
    try {
      await api('/v1/agents/request-session', {
        method: 'POST',
        body: JSON.stringify({ channel_name: channelName }),
      });
      btn.textContent = 'Sent!';
      await refresh();
    } catch (err) {
      btn.disabled = false;
      btn.textContent = 'Start Session';
      showError(err);
    }
  });

  // Inline editing for agent subscriptions
  els.agentsTbody.addEventListener('click', (ev) => {
    const cell = ev.target?.closest?.('.subs-cell');
    if (!cell) return;
    if (cell.querySelector('input')) return; // already editing

    const agentId = cell.getAttribute('data-agent-id');
    const currentSubs = cell.getAttribute('data-subs') || '';

    const currentList = currentSubs
      .split(',')
      .map((s) => s.trim())
      .filter(Boolean);
    const current = currentList[0] || '';
    const channelNames = (lastChannels || [])
      .map((c) => String(c?.name || '').trim())
      .filter(Boolean)
      .sort((a, b) => a.localeCompare(b));

    let finished = false;

    const renderCellValue = (subsText, stateText = '') => {
      const safeSubs = escapeHtml(subsText);
      const valueHtml = safeSubs || '<span class="muted" style="opacity:0.5">click to assign</span>';
      const stateHtml = stateText
        ? ` <span class="muted" style="font-size:11px;opacity:0.85;">${escapeHtml(stateText)}</span>`
        : '';
      cell.setAttribute('data-subs', subsText);
      cell.style.cursor = 'pointer';
      cell.innerHTML = `${valueHtml}${stateHtml}`;
    };

    if (!channelNames.length) {
      renderCellValue(currentSubs, 'no channels to select');
      return;
    }

    const select = document.createElement('select');
    select.style.cssText = 'width:100%;box-sizing:border-box;padding:2px 4px;font-size:12px;background:#1a1a2e;color:#e0e0e0;border:1px solid #6ee7b7;border-radius:3px;';

    const emptyOpt = document.createElement('option');
    emptyOpt.value = '';
    emptyOpt.textContent = '(none)';
    if (!current) emptyOpt.selected = true;
    select.appendChild(emptyOpt);

    channelNames.forEach((name) => {
      const opt = document.createElement('option');
      opt.value = name;
      opt.textContent = name;
      if (name === current) opt.selected = true;
      select.appendChild(opt);
    });

    cell.textContent = '';
    cell.appendChild(select);
    select.focus();

    const save = async () => {
      if (finished) return;
      finished = true;
      select.removeEventListener('keydown', onKeyDown);
      select.removeEventListener('change', onChange);
      if (document.activeElement === select) select.blur();

      const selected = String(select.value || '').trim();
      const subs = selected ? [selected] : [];
      const nextSubs = subs.join(', ');
      renderCellValue(nextSubs, 'saving...');

      try {
        await api(`/v1/agents/${encodeURIComponent(agentId)}/channels`, {
          method: 'PATCH',
          body: JSON.stringify({ subscriptions: subs }),
        });
        renderCellValue(nextSubs, 'saved');
      } catch (err) {
        showError(err);
        renderCellValue(currentSubs, 'save failed');
      }

      // SSE will trigger refresh; do explicit refresh as fallback
      await refresh().catch(() => {});
    };

    const cancel = () => {
      if (finished) return;
      finished = true;
      select.removeEventListener('keydown', onKeyDown);
      select.removeEventListener('change', onChange);
      if (document.activeElement === select) select.blur();
      renderCellValue(currentSubs, 'cancelled');
      refresh().catch(() => {});
    };

    const onKeyDown = (e) => {
      if (e.key === 'Escape') {
        e.preventDefault();
        cancel();
      }
    };
    const onChange = () => {
      save().catch(() => {});
    };

    select.addEventListener('keydown', onKeyDown);
    select.addEventListener('change', onChange);
  });

  els.taskChannel.addEventListener('change', () => {
    fillChainSelect(els.taskChain, lastChains, els.taskChannel.value);
  });

  els.createChannelBtn.addEventListener('click', async () => {
    const name = els.channelName.value.trim();
    const description = els.channelDesc.value.trim();
    if (!name) return;
    await api('/v1/channels', {
      method: 'POST',
      body: JSON.stringify({ name, description }),
    });
    els.channelName.value = '';
    els.channelDesc.value = '';
    await refresh();
  });

  els.createTaskBtn.addEventListener('click', async () => {
    const channel_id = els.taskChannel.value;
    const chain_id = els.taskChain.value;
    const title = els.taskTitle.value.trim();
    const description = els.taskDesc.value.trim();
    const execution_mode = els.taskExecutionMode.value.trim();
    if (!channel_id || !title) return;
    const body = { channel_id, title, description, status: 'queued', priority: 0 };
    if (chain_id) {
      body.chain_id = chain_id;
    }
    if (execution_mode) {
      body.execution_mode = execution_mode;
    }
    await api('/v1/tasks', {
      method: 'POST',
      body: JSON.stringify(body),
    });
    els.taskTitle.value = '';
    els.taskDesc.value = '';
    els.taskExecutionMode.value = '';
    await refresh();
  });

  els.claimTaskBtn.addEventListener('click', async () => {
    const agent_id = els.claimAgentId.value.trim();
    const channel_id = els.claimChannel.value;
    if (!agent_id || !channel_id) return;
    await api('/v1/tasks/claim', {
      method: 'POST',
      body: JSON.stringify({ agent_id, channel_id }),
    });
    await refresh();
  });

  // --- Notification Bell & Panel ---
  const notifBell = document.getElementById('notifBell');
  const notifPanelClose = document.getElementById('notifPanelClose');
  const toastContainer = document.getElementById('toastContainer');

  if (notifBell) {
    notifBell.addEventListener('click', () => toggleNotifPanel());
  }
  if (notifPanelClose) {
    notifPanelClose.addEventListener('click', () => {
      isNotifPanelOpen = false;
      const panel = document.getElementById('notifPanel');
      if (panel) panel.classList.add('hidden');
    });
  }

  // Close panel on outside click
  document.addEventListener('click', (e) => {
    if (!isNotifPanelOpen) return;
    const panel = document.getElementById('notifPanel');
    if (panel && !panel.contains(e.target) && notifBell && !notifBell.contains(e.target)) {
      isNotifPanelOpen = false;
      panel.classList.add('hidden');
    }
  });

  // Toast actions (event delegation)
  if (toastContainer) {
    toastContainer.addEventListener('click', async (ev) => {
      // X button to dismiss toast
      const closeBtn = ev.target?.closest?.('[data-toast-dismiss]');
      if (closeBtn) {
        const id = Number(closeBtn.getAttribute('data-toast-dismiss'));
        removeToast(id);
        return;
      }
      // Start Session button on toast
      const actionBtn = ev.target?.closest?.('[data-toast-action]');
      if (actionBtn) {
        const channel = actionBtn.getAttribute('data-channel') || '';
        const toastId = Number(actionBtn.getAttribute('data-toast-id'));
        if (channel) {
          try {
            await api('/v1/agents/request-session', {
              method: 'POST',
              body: JSON.stringify({ channel_name: channel }),
            });
            removeToast(toastId);
            await refresh();
          } catch (err) { showError(err); }
        }
      }
    });
  }

  await refresh();
  // Load initial unseen count from server
  try {
    const data = await api('/v1/notifications');
    const list = data.notifications || [];
    unseenCount = list.length;
    renderBellBadge();
  } catch { /* ignore */ }
  startStream();

  setInterval(() => {
    if (els.autoRefresh.checked) refresh().catch(() => {});
  }, 10000);
}

function showError(err) {
  // eslint-disable-next-line no-console
  console.error(err);
  alert(String(err?.message || err));
}

main().catch(showError);
