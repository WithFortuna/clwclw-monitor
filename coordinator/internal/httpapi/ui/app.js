const els = {
  refreshBtn: document.getElementById('refreshBtn'),
  autoRefresh: document.getElementById('autoRefresh'),
  apiKey: document.getElementById('apiKey'),
  saveApiKeyBtn: document.getElementById('saveApiKeyBtn'),
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
let lastPromptByTaskId = new Map();
let promptModalState = { taskId: '', agentId: '', event: null };

function getApiKey() {
  return (localStorage.getItem('clw_api_key') || '').trim();
}

function setApiKey(value) {
  const v = String(value || '').trim();
  if (!v) localStorage.removeItem('clw_api_key');
  else localStorage.setItem('clw_api_key', v);
}

async function api(path, options = {}) {
  const apiKey = getApiKey();
  const res = await fetch(path, {
    headers: {
      'Content-Type': 'application/json',
      ...(apiKey ? { 'X-Api-Key': apiKey } : {}),
    },
    ...options,
  });
  if (!res.ok) {
    const text = await res.text().catch(() => '');
    throw new Error(`${res.status} ${text}`);
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
  const apiKey = getApiKey();
  const url = apiKey ? `/v1/stream?api_key=${encodeURIComponent(apiKey)}` : '/v1/stream';
  stream = new EventSource(url);
  stream.addEventListener('update', scheduleRefresh);
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

function renderAgents(agents) {
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
      const tmux = a?.meta?.tmux_display || a?.meta?.pane_id || a?.meta?.tmux_target || a?.meta?.tmux_session || '';
      const workerStatus = a.worker_status ? a.worker_status : deriveWorkerStatus(a.last_seen);
      const claudeStatus = a.claude_status || a.status || 'idle';
      return `<tr>
        <td>${escapeHtml(name)}</td>
        <td>${workerStatusBadge(a.last_seen)}</td>
        <td>${claudeStatusBadge(claudeStatus)}</td>
        <td class="muted">${escapeHtml(subs)}</td>
        <td class="muted">${escapeHtml(tmux)}</td>
        <td class="muted">${escapeHtml(fmtTime(a.last_seen))}</td>
        <td class="muted">${escapeHtml(a.current_task_id || '')}</td>
      </tr>`;
    })
    .join('');
}

function groupTasksByChannel(tasks) {
  const by = new Map();
  for (const t of tasks) {
    const channelId = t.channel_id || 'unknown';
    if (!by.has(channelId)) by.set(channelId, []);
    by.get(channelId).push(t);
  }
  return by;
}

function renderTaskBoard(channels, chains, tasks) {
  if (!channels.length && !chains.length && !tasks.length) {
    els.taskBoard.innerHTML = `<div class="muted">No channels, chains, or tasks yet.</div>`;
    return;
  }

  const tasksByChain = new Map();
  const nonChainTasks = [];
  for (const t of tasks) {
    if (t.chain_id) {
      if (!tasksByChain.has(t.chain_id)) tasksByChain.set(t.chain_id, []);
      tasksByChain.get(t.chain_id).push(t);
    } else {
      nonChainTasks.push(t);
    }
  }

  const output = [];

  // Render chains
  for (const ch of chains) {
    const list = tasksByChain.get(ch.id) || [];
    const queued = list.filter((t) => t.status === 'queued').sort((a, b) => a.sequence - b.sequence);
    const prog = list.filter((t) => t.status === 'in_progress').sort((a, b) => a.sequence - b.sequence);
    const done = list.filter((t) => t.status === 'done').sort((a, b) => a.sequence - b.sequence);
    const failed = list.filter((t) => t.status === 'failed').sort((a, b) => a.sequence - b.sequence);

    output.push(`
      <div class="col">
        <div class="col-title">
          <div>Chain: ${escapeHtml(ch.name)} (${claudeStatusBadge(ch.status)})</div>
          <span class="pill">${list.length} tasks</span>
        </div>
        <div class="board">
          ${renderTaskCol('Queued', queued, { variant: 'queued' })}
          ${renderTaskCol('In Progress', prog, { variant: 'in_progress' })}
          ${renderTaskCol('Done', done, { variant: 'done' })}
          ${renderTaskCol('Failed', failed, { variant: 'failed' })}
        </div>
      </div>
    `);
  }

  // Render non-chain tasks, grouped by channel
  const nonChainTasksByChannel = groupTasksByChannel(nonChainTasks);
  for (const ch of channels) {
    const list = nonChainTasksByChannel.get(ch.id) || [];

    const queued = list.filter((t) => t.status === 'queued');
    const prog = list.filter((t) => t.status === 'in_progress');
    const done = list.filter((t) => t.status === 'done');
    const failed = list.filter((t) => t.status === 'failed');

    output.push(`
      <div class="col">
        <div class="col-title">
          <div>Channel: ${escapeHtml(ch.name)} (No Chain)</div>
          <span class="pill">${list.length} tasks</span>
        </div>
        <div class="board">
          ${renderTaskCol('Queued', queued, { variant: 'queued' })}
          ${renderTaskCol('In Progress', prog, { variant: 'in_progress' })}
          ${renderTaskCol('Done', done, { variant: 'done' })}
          ${renderTaskCol('Failed', failed, { variant: 'failed' })}
        </div>
      </div>
    `);
  }
  
  els.taskBoard.innerHTML = output.join('');
}

function renderTaskCol(title, tasks, opts = {}) {
  const variant = opts.variant || '';
  const items = tasks
    .map(
      (t) => `
      <div class="task">
        <div class="task-title">${escapeHtml(t.title)}</div>
        <div class="task-desc">${escapeHtml(t.description || '')}</div>
        ${
          variant === 'queued'
            ? `<div style="margin-top:10px;display:flex;gap:10px;align-items:center;">
                 <button class="btn" data-action="assign" data-task-id="${escapeHtml(t.id)}">Assign…</button>
               </div>`
            : ''
        }
        ${
          variant === 'in_progress'
            ? `<div style="margin-top:10px;display:flex;gap:10px;align-items:center;">
                 ${
                   lastPromptByTaskId.has(t.id)
                     ? `<button class="btn" data-action="prompt" data-task-id="${escapeHtml(t.id)}">Prompt…</button>`
                     : ''
                 }
                 <button class="btn" data-action="complete" data-task-id="${escapeHtml(t.id)}">Mark Done</button>
                 <button class="btn danger" data-action="fail" data-task-id="${escapeHtml(t.id)}">Fail</button>
                 <div class="muted" style="font-size:11px;">agent: ${escapeHtml(t.assigned_agent_id || '')}</div>
               </div>`
            : ''
        }
        <div class="muted" style="margin-top:6px;font-size:11px;">
          ${t.chain_id ? `chain: ${escapeHtml(t.chain_id)} seq: ${t.sequence}<br>` : ''}
          ${escapeHtml(t.id)}
        </div>
      </div>
    `,
    )
    .join('');

  return `
    <div class="col">
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
  selectEl.innerHTML = channels
    .map((c) => `<option value="${escapeHtml(c.id)}">${escapeHtml(c.name)}</option>`)
    .join('');
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
  lastChainsById = new Map(chains.map((c) => [c.id, c])); // Populate lastChainsById
  lastPromptByTaskId = computeLatestPrompts(events);

  els.lastRefresh.textContent = fmtTime(new Date().toISOString());
  renderAgents(agents);
  fillChannelSelect(els.taskChannel, channels);
  fillChannelSelect(els.claimChannel, channels);
  renderTaskBoard(channels, chains, tasks); // Pass chains to renderTaskBoard
  renderEvents(events);
}

async function main() {
  els.refreshBtn.addEventListener('click', () => refresh().catch(showError));

  // Auth (optional)
  if (els.apiKey) {
    els.apiKey.value = getApiKey();
  }
  if (els.saveApiKeyBtn) {
    els.saveApiKeyBtn.addEventListener('click', () => {
      setApiKey(els.apiKey?.value || '');
      startStream();
      scheduleRefresh();
    });
  }

  els.taskBoard.addEventListener('click', async (ev) => {
    const btn = ev.target?.closest?.('button[data-action]');
    if (!btn) return;

    const task_id = btn.getAttribute('data-task-id');
    if (!task_id && btn.getAttribute('data-action') !== 'prompt-option') return;
    const action = btn.getAttribute('data-action') || '';

    try {
      if (action === 'prompt') {
        openPromptModal(task_id);
        return;
      }
      if (action === 'complete') {
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
    const title = els.taskTitle.value.trim();
    const description = els.taskDesc.value.trim();
    const execution_mode = els.taskExecutionMode.value.trim();
    if (!channel_id || !title) return;
    const body = { channel_id, title, description, status: 'queued', priority: 0 };
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

  await refresh();
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
