# Dual Status Architecture

## Overview

The dual status system separates agent worker lifecycle from Claude Code execution state.

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                     Agent Worker Process                     │
│  ┌───────────────────────────────────────────────────────┐  │
│  │  Heartbeat Loop (every 15s)                           │  │
│  │  ├─ Read current-task.json                            │  │
│  │  ├─ Detect Claude state (idle/running/waiting)        │  │
│  │  └─ POST /v1/agents/heartbeat                         │  │
│  │     {                                                  │  │
│  │       "agent_id": "uuid",                             │  │
│  │       "claude_status": "idle|running|waiting",        │  │
│  │       "current_task_id": "uuid"                       │  │
│  │     }                                                  │  │
│  └───────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                             │
                             │ HTTP POST
                             ▼
┌─────────────────────────────────────────────────────────────┐
│                    Coordinator API Server                    │
│  ┌───────────────────────────────────────────────────────┐  │
│  │  handleAgentsHeartbeat()                              │  │
│  │  ├─ Parse request                                     │  │
│  │  ├─ Store claude_status in database                   │  │
│  │  ├─ Update last_seen timestamp                        │  │
│  │  └─ Broadcast "agents" SSE event                      │  │
│  └───────────────────────────────────────────────────────┘  │
│                             │                                │
│  ┌───────────────────────────────────────────────────────┐  │
│  │  Database (agents table)                              │  │
│  │  ┌──────────────────────────────────────────────┐    │  │
│  │  │ id | name | claude_status | last_seen | ... │    │  │
│  │  ├──────────────────────────────────────────────┤    │  │
│  │  │ 1  | dev  | running       | 2026-02-06  ...│    │  │
│  │  │ 2  | test | idle          | 2026-02-06  ...│    │  │
│  │  └──────────────────────────────────────────────┘    │  │
│  └───────────────────────────────────────────────────────┘  │
│                             │                                │
│  ┌───────────────────────────────────────────────────────┐  │
│  │  handleAgentsList() / handleDashboard()               │  │
│  │  ├─ Fetch agents from database                        │  │
│  │  ├─ Compute worker_status for each:                   │  │
│  │  │   if (now - last_seen) < 30s → "online"           │  │
│  │  │   else → "offline"                                 │  │
│  │  └─ Return { agents: [...{agent, worker_status}] }   │  │
│  └───────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                             │
                             │ SSE / HTTP GET
                             ▼
┌─────────────────────────────────────────────────────────────┐
│                      Dashboard (Browser)                     │
│  ┌───────────────────────────────────────────────────────┐  │
│  │  Agents Table                                         │  │
│  │  ┌──────────────────────────────────────────────┐    │  │
│  │  │ Name | Worker  | Claude  | Last Seen | ...  │    │  │
│  │  ├──────────────────────────────────────────────┤    │  │
│  │  │ dev  │ online  │ running │ 2s ago    │ ... │    │  │
│  │  │ test │ online  │ idle    │ 5s ago    │ ... │    │  │
│  │  │ old  │ offline │ idle    │ 2m ago    │ ... │    │  │
│  │  └──────────────────────────────────────────────┘    │  │
│  │                                                       │  │
│  │  deriveWorkerStatus(last_seen):                      │  │
│  │    age = now - last_seen                             │  │
│  │    return age < 30s ? "online" : "offline"           │  │
│  │                                                       │  │
│  │  workerStatusBadge(last_seen):                       │  │
│  │    status = deriveWorkerStatus(last_seen)            │  │
│  │    return <badge class={green|red}>{status}</badge>  │  │
│  │                                                       │  │
│  │  claudeStatusBadge(claude_status):                   │  │
│  │    return <badge class={muted|green|yellow}>         │  │
│  │             {claude_status}                          │  │
│  │           </badge>                                   │  │
│  └───────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

## Status Flow

### Worker Status (Computed)
```
Agent Process
    │
    ├─ Heartbeat sent → last_seen updated in DB
    │                   (every 15 seconds)
    │
    ▼
Coordinator
    │
    ├─ On read: compute from last_seen
    │   if (now - last_seen) < 30s:
    │     worker_status = "online"
    │   else:
    │     worker_status = "offline"
    │
    ▼
Dashboard
    │
    └─ Display: [online] or [offline] badge
```

### Claude Status (Reported)
```
Agent Process
    │
    ├─ Check current-task.json
    │   ├─ No task → "idle"
    │   ├─ Task executing → "running"
    │   └─ Waiting for input → "waiting"
    │
    ├─ Send in heartbeat payload
    │
    ▼
Coordinator
    │
    ├─ Store claude_status in DB
    │
    ▼
Dashboard
    │
    └─ Display: [idle] or [running] or [waiting] badge
```

## Task Lifecycle Changes

### Before (Problematic)
```
API: POST /v1/tasks/claim
    ├─ Task status: queued → in_progress
    └─ Agent status: (any) → running  ❌ AUTOMATIC UPDATE

Agent Heartbeat (no task)
    └─ Agent status: running → idle   ❌ CONFLICT

Result: Inconsistent state, status flapping
```

### After (Fixed)
```
API: POST /v1/tasks/claim
    ├─ Task status: queued → in_progress
    └─ Agent status: NO AUTOMATIC UPDATE  ✅

Agent Heartbeat
    ├─ Reads current-task.json
    ├─ Determines claude_status (idle/running/waiting)
    └─ Reports in heartbeat  ✅ SINGLE SOURCE OF TRUTH

Result: Consistent state, no conflicts
```

## Example Scenarios

### Scenario 1: Worker Running, No Task
```
Agent: worker process active, tmux panel open, no task claimed
    ↓
Heartbeat: claude_status = "idle", current_task_id = ""
    ↓
Dashboard:
    Worker: [online]  ← Derived from recent last_seen
    Claude: [idle]    ← Reported by agent
```

### Scenario 2: Worker Running, Task Executing
```
Agent: worker process active, task claimed and injected into tmux
    ↓
Heartbeat: claude_status = "running", current_task_id = "task-uuid"
    ↓
Dashboard:
    Worker: [online]   ← Derived from recent last_seen
    Claude: [running]  ← Reported by agent
```

### Scenario 3: Worker Crashed
```
Agent: worker process stopped (no more heartbeats)
    ↓
Last heartbeat: 60 seconds ago
    ↓
Dashboard:
    Worker: [offline]  ← Derived from stale last_seen (> 30s)
    Claude: [running]  ← Last known state (stale)
```

### Scenario 4: Interactive Prompt
```
Agent: Claude asks user for confirmation (y/n)
    ↓
Heartbeat: claude_status = "waiting", current_task_id = "task-uuid"
    ↓
Dashboard:
    Worker: [online]   ← Derived from recent last_seen
    Claude: [waiting]  ← Reported by agent
    ↓
User provides input via dashboard
    ↓
Heartbeat: claude_status = "running", current_task_id = "task-uuid"
    ↓
Dashboard:
    Worker: [online]
    Claude: [running]
```

## Key Design Decisions

### 1. Worker Status is Computed, Not Stored
- **Why**: Avoid database writes on every status computation
- **Trade-off**: Must compute on every read (acceptable overhead)
- **Threshold**: 30 seconds (2x heartbeat interval of 15s)

### 2. Claude Status is Reported, Not Inferred
- **Why**: Only the agent knows true execution state
- **Trade-off**: Requires agent updates to change status
- **Source**: Agent reads `current-task.json` and hook state

### 3. No Automatic Status Updates on Task Operations
- **Why**: Avoid race conditions and conflicts with heartbeat
- **Trade-off**: Task claim/complete no longer updates agent status
- **Solution**: Agent heartbeat is single source of truth

### 4. Backward Compatibility Maintained
- **Why**: Avoid breaking existing agents during rollout
- **Trade-off**: Temporary duplication (`status` + `claude_status`)
- **Migration**: Old agents work, new agents send both fields

## Benefits

1. **Clarity**: Clear distinction between worker lifecycle and Claude execution
2. **Accuracy**: Status reflects actual state (no incorrect "running" for idle workers)
3. **Consistency**: Single source of truth (agent heartbeat)
4. **Observability**: Dashboard shows both worker health and task execution state
5. **Debugging**: Easy to identify crashed workers vs. idle Claude sessions

## References

- Implementation: `IMPLEMENTATION_SUMMARY.md`
- Testing: `RUNBOOK.md`
- API Docs: `CLAUDE.md`
