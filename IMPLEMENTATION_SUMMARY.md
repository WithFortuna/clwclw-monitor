# Implementation Summary: Dual Status System

## Issue
[#1 - [fix] 클로드 코드 에이전트 상태관리](https://github.com/WithFortuna/clwclw-monitor/issues/1)

## Problem
The `Agent.Status` field conflated two different concepts:
1. Whether the agent worker process is running (worker lifecycle)
2. Whether Claude Code is actively executing a task (execution state)

This caused the status to incorrectly show `running` when only the tmux panel was open (worker running, no task).

## Solution Implemented
Separated concerns into two distinct status fields:
- **Worker Status** (computed from `last_seen` timestamp): online/offline
- **Claude Status** (reported by agent): idle/running/waiting

## Changes Made

### 1. Backend (Go - Coordinator)

#### Model (`coordinator/internal/model/model.go`)
- Added `ClaudeStatus` type with constants: `idle`, `running`, `waiting`
- Added `WorkerStatus` type with constants: `online`, `offline`
- Added `ClaudeStatus` field to `Agent` struct
- Added `DerivedWorkerStatus(threshold)` helper method to compute worker status from `last_seen`
- Kept `Status` field for backward compatibility (deprecated)

#### Database Migration (`supabase/migrations/0003_dual_status.sql`)
- Added `claude_status` column to `agents` table with check constraint
- Migrated existing `status` values to `claude_status`
- Added index on `claude_status` for filtering
- Updated `claim_task()` stored procedure to **remove automatic agent status update**
- Agent heartbeat is now sole source of truth for Claude status

#### Memory Store (`coordinator/internal/store/memory/memory.go`)
- Updated `UpsertAgent()` to accept and store `claude_status` field
- **Removed automatic status updates** from:
  - `ClaimTask()` - no longer sets agent status to `running`
  - `CompleteTask()` - no longer sets agent status to `idle`
  - `FailTask()` - no longer sets agent status to `idle`
- Agent heartbeat becomes sole source of truth for Claude status

#### Postgres Store (`coordinator/internal/store/postgres/postgres.go`)
- Updated `UpsertAgent()` to handle `claude_status` column in SQL queries
- Updated `ListAgents()` to fetch `claude_status` from database

#### API Handlers (`coordinator/internal/httpapi/handlers.go`)
- Added `ClaudeStatus` field to `agentsHeartbeatRequest` struct
- Updated `handleAgentsHeartbeat()` to accept `claude_status` field (falls back to `status` for backward compatibility)
- Created `agentResponse` struct that includes computed `worker_status`
- Updated `handleAgentsList()` to compute and return `worker_status` for each agent (30-second threshold)

#### Dashboard API (`coordinator/internal/httpapi/dashboard.go`)
- Updated `handleDashboard()` to compute `worker_status` for each agent before caching
- Dashboard responses now include both `worker_status` and `claude_status`

### 2. Agent (Node.js)

#### Heartbeat (`agent/clw-agent.js`)
- Updated `heartbeat()` function to send both:
  - `claude_status` (new field) - explicit Claude execution state
  - `status` (legacy field) - kept for backward compatibility with same value
- No changes needed to call sites - all heartbeat calls remain unchanged

### 3. Dashboard (Frontend)

#### UI Logic (`coordinator/internal/httpapi/ui/app.js`)
- Added `deriveWorkerStatus(lastSeen)` function - computes online/offline from timestamp (30s threshold)
- Added `workerStatusBadge(lastSeen)` function - renders worker status badge (online=green, offline=red)
- Added `claudeStatusBadge(claudeStatus)` function - renders Claude status badge (idle=muted, running=green, waiting=yellow)
- Updated `renderAgents()` to display two status columns:
  - Worker status (online/offline)
  - Claude status (idle/running/waiting)
- Updated colspan from 6 to 7 for empty state

#### HTML (`coordinator/internal/httpapi/ui/index.html`)
- Added two column headers: "Worker" and "Claude" (replaced single "Status" column)
- Updated empty state colspan from 6 to 7

#### Styles (`coordinator/internal/httpapi/ui/styles.css`)
- Added `.badge.err` class for offline status (red)
- Added `.badge.muted-badge` class for idle status (muted gray)

## Status Semantics

| Scenario | Worker Status | Claude Status | Current Task ID |
|----------|--------------|---------------|-----------------|
| Worker running, no task | `online` | `idle` | `""` (empty) |
| Worker running, task executing | `online` | `running` | `"<uuid>"` |
| Worker running, waiting for user input | `online` | `waiting` | `"<uuid>"` |
| Worker crashed/stopped | `offline` | (last known) | (last known) |

## Key Behavioral Changes

### Before (Problematic)
1. Task claimed via API → Coordinator auto-sets `agent.status = "running"`
2. Agent sends heartbeat → May override to `"idle"` if no task in `current-task.json`
3. Tmux panel open, no task → Status incorrectly shows `"running"`

### After (Fixed)
1. Task claimed via API → **No automatic agent status update**
2. Agent sends heartbeat with `claude_status`:
   - No task claimed → `"idle"`
   - Task claimed and executing → `"running"`
   - Interactive prompt detected → `"waiting"`
3. Worker status derived from `last_seen`:
   - Recent heartbeat (< 30s) → `"online"`
   - Stale heartbeat (> 30s) → `"offline"`
4. Dashboard clearly shows: Worker=Online, Claude=Idle (correct state)

## Migration Path

### Phase 1: Deploy Coordinator (Non-Breaking)
1. Run database migration: `supabase/migrations/0003_dual_status.sql`
2. Deploy updated Coordinator with dual status support
3. Old agents continue working (send `status`, Coordinator stores both `status` and `claude_status`)

### Phase 2: Update Agents
1. Deploy updated agent code (sends both `claude_status` and `status`)
2. Agents report accurate Claude execution state via heartbeat

### Phase 3: Dashboard
1. Dashboard automatically shows dual status for all agents
2. Users see clear distinction between worker lifecycle and Claude execution state

## Testing

### Build Verification
- ✅ Go code compiles successfully: `cd coordinator && go build ./cmd/coordinator`

### Manual Testing Scenarios
See RUNBOOK.md for detailed testing procedures:

1. **Worker online, no task**: Start worker, verify Worker=online, Claude=idle
2. **Worker online, task executing**: Create/claim task, verify Worker=online, Claude=running
3. **Worker online, waiting**: Trigger interactive prompt, verify Worker=online, Claude=waiting
4. **Worker offline**: Stop worker, wait 30s, verify Worker=offline
5. **Task completion**: Complete task, verify Claude returns to idle

### API Verification Commands
```bash
# Check agent status
curl -s http://localhost:8080/v1/agents | jq '.[] | {name, worker_status, claude_status, current_task_id, last_seen}'

# Send heartbeat with claude_status
AGENT_ID="$(cat agent/data/agent-id.txt | tr -d '\n')"
curl -X POST http://localhost:8080/v1/agents/heartbeat \
  -H 'Content-Type: application/json' \
  -d "{\"agent_id\":\"${AGENT_ID}\",\"name\":\"test-agent\",\"claude_status\":\"idle\",\"current_task_id\":\"\"}"
```

## Files Modified

### Backend (Go)
1. `coordinator/internal/model/model.go` - Status types and Agent struct
2. `coordinator/internal/store/memory/memory.go` - Memory store logic
3. `coordinator/internal/store/postgres/postgres.go` - Postgres queries
4. `coordinator/internal/httpapi/handlers.go` - API request/response handling
5. `coordinator/internal/httpapi/dashboard.go` - Dashboard aggregation
6. `supabase/migrations/0003_dual_status.sql` - Database schema (NEW)

### Agent (Node.js)
7. `agent/clw-agent.js` - Heartbeat payload

### Dashboard (Frontend)
8. `coordinator/internal/httpapi/ui/app.js` - UI rendering logic
9. `coordinator/internal/httpapi/ui/index.html` - HTML structure
10. `coordinator/internal/httpapi/ui/styles.css` - CSS styling

## Backward Compatibility

- ✅ Old agents sending only `status` continue to work (mapped to `claude_status`)
- ✅ `status` field kept in model for legacy compatibility
- ✅ API accepts both `status` and `claude_status` fields
- ✅ No breaking changes to existing integrations

## Future Improvements

1. **Agent Detection**: Automatically detect interactive prompts and set `claude_status=waiting`
2. **Alerting**: Notify when worker goes offline
3. **Metrics**: Track average task execution time per Claude status transition
4. **UI Filters**: Filter agents by worker status or Claude status in dashboard

## References

- Issue: https://github.com/WithFortuna/clwclw-monitor/issues/1
- Requirements: `REQUIREMENTS.md`
- Testing Guide: `RUNBOOK.md`
- Project Guide: `CLAUDE.md`
