# Dual Status Testing Checklist

## Pre-Deployment

### Build Verification
- [x] Go code compiles: `cd coordinator && go build ./cmd/coordinator`
- [ ] No Go lint warnings: `cd coordinator && go vet ./...`
- [ ] Database migration syntax valid: Check `supabase/migrations/0003_dual_status.sql`

### Code Review
- [x] Model changes: `ClaudeStatus`, `WorkerStatus`, `Agent.ClaudeStatus` added
- [x] Store changes: Automatic status updates removed from claim/complete/fail
- [x] API changes: Heartbeat accepts `claude_status`, agent list returns `worker_status`
- [x] Agent changes: Heartbeat sends both `claude_status` and `status`
- [x] UI changes: Dashboard renders two status columns

## Deployment Testing

### Phase 1: Deploy Coordinator

#### 1. Database Migration
```bash
# If using Supabase, apply migration via dashboard or CLI
supabase db push

# If using local Postgres
psql $DATABASE_URL -f supabase/migrations/0003_dual_status.sql
```

**Verify:**
- [ ] Migration completes without errors
- [ ] `agents` table has `claude_status` column
- [ ] Existing agents migrated: `status` values copied to `claude_status`
- [ ] Index created: `idx_agents_claude_status`

#### 2. Start Coordinator
```bash
# In-memory mode
go run ./coordinator/cmd/coordinator

# Postgres mode
COORDINATOR_DATABASE_URL='postgres://...' go run ./coordinator/cmd/coordinator
```

**Verify:**
- [ ] Server starts on port 8080
- [ ] Health check passes: `curl http://localhost:8080/health`
- [ ] Dashboard loads: `open http://localhost:8080/`

#### 3. Test API with Old Agent (Backward Compatibility)
```bash
# Simulate old agent sending only 'status'
curl -X POST http://localhost:8080/v1/agents/heartbeat \
  -H 'Content-Type: application/json' \
  -d '{
    "agent_id": "test-old-agent",
    "name": "Old Agent",
    "status": "idle",
    "current_task_id": ""
  }'
```

**Verify:**
- [ ] Request succeeds (200 OK)
- [ ] Agent appears in dashboard
- [ ] Both `status` and `claude_status` set to "idle"
- [ ] Worker status shows "online"

#### 4. Test API with New Agent
```bash
# Simulate new agent sending both fields
curl -X POST http://localhost:8080/v1/agents/heartbeat \
  -H 'Content-Type: application/json' \
  -d '{
    "agent_id": "test-new-agent",
    "name": "New Agent",
    "status": "idle",
    "claude_status": "idle",
    "current_task_id": ""
  }'
```

**Verify:**
- [ ] Request succeeds (200 OK)
- [ ] Agent appears in dashboard
- [ ] `claude_status` field respected
- [ ] Worker status shows "online"

### Phase 2: Deploy Agent

#### 5. Update Agent Code
```bash
# Pull latest agent code
git pull origin main

# Verify heartbeat function updated
grep -A 10 "function heartbeat" agent/clw-agent.js | grep claude_status
```

**Verify:**
- [ ] `claude_status` field present in heartbeat payload
- [ ] `status` field kept for backward compatibility

#### 6. Restart Agent Worker
```bash
# Stop existing worker (Ctrl+C or pkill)
pkill -f "clw-agent.js work"

# Start with new code
COORDINATOR_URL=http://localhost:8080 \
node agent/clw-agent.js work --channel test-channel --tmux-target claude-code:1.0
```

**Verify:**
- [ ] Worker starts without errors
- [ ] Heartbeat logs show successful POST to coordinator
- [ ] Dashboard shows agent as "online"

### Phase 3: Functional Testing

#### Test Case 1: Worker Online, No Task
**Setup:**
1. Start worker agent
2. Don't claim any tasks
3. Wait for heartbeat (15 seconds)

**Expected:**
- [ ] Dashboard shows:
  - Worker: `online` (green badge)
  - Claude: `idle` (muted badge)
  - Current Task: (empty)
  - Last Seen: < 30 seconds ago

**Verify:**
```bash
curl -s http://localhost:8080/v1/agents | jq '.agents[] | {name, worker_status, claude_status, current_task_id}'
```

#### Test Case 2: Worker Online, Task Executing
**Setup:**
1. Create task via dashboard or API
2. Agent claims task (via work loop or manual claim)
3. Task injected into tmux session
4. Wait for heartbeat (15 seconds)

**Expected:**
- [ ] Dashboard shows:
  - Worker: `online` (green badge)
  - Claude: `running` (green badge)
  - Current Task: task UUID
  - Last Seen: < 30 seconds ago

**Verify:**
```bash
# Check current-task.json exists
cat agent/data/current-task.json

# Check dashboard
curl -s http://localhost:8080/v1/dashboard | jq '.agents[] | select(.name=="test-agent")'
```

#### Test Case 3: Worker Offline
**Setup:**
1. Stop worker process (Ctrl+C or `pkill`)
2. Wait 35 seconds (> 30s threshold)
3. Refresh dashboard

**Expected:**
- [ ] Dashboard shows:
  - Worker: `offline` (red badge)
  - Claude: (last known state)
  - Last Seen: > 30 seconds ago

**Verify:**
```bash
# Check agent status
curl -s http://localhost:8080/v1/agents | jq '.agents[] | select(.name=="test-agent") | {worker_status, last_seen}'
```

#### Test Case 4: Task Completion
**Setup:**
1. Worker online, task executing
2. Task completes (Claude hook fires)
3. Agent detects completion via hook
4. Wait for heartbeat (15 seconds)

**Expected:**
- [ ] Dashboard shows:
  - Worker: `online` (green badge)
  - Claude: `idle` (muted badge)
  - Current Task: (empty)
  - Task status: `done`

**Verify:**
```bash
# Check current-task.json cleared
cat agent/data/current-task.json  # Should be empty or deleted

# Check agent status
curl -s http://localhost:8080/v1/agents | jq '.agents[] | select(.name=="test-agent")'

# Check task status
curl -s http://localhost:8080/v1/tasks | jq '.tasks[] | select(.id=="<task-uuid>") | {status, done_at}'
```

#### Test Case 5: Task Claim (No Auto Status Update)
**Setup:**
1. Worker online, Claude idle
2. Manually claim task via API
3. Immediately check agent status (before next heartbeat)

**Expected:**
- [ ] Task status changes to `in_progress`
- [ ] Agent `claude_status` remains `idle` (not auto-updated to `running`)
- [ ] After next heartbeat (when task injected): `claude_status` changes to `running`

**Verify:**
```bash
# Claim task
curl -X POST http://localhost:8080/v1/tasks/claim \
  -H 'Content-Type: application/json' \
  -d '{
    "agent_id": "test-agent-id",
    "channel_id": "channel-uuid"
  }'

# Check agent status immediately (should still be idle)
curl -s http://localhost:8080/v1/agents | jq '.agents[] | select(.name=="test-agent") | {claude_status}'

# Wait 15 seconds for heartbeat
sleep 16

# Check agent status again (should now be running)
curl -s http://localhost:8080/v1/agents | jq '.agents[] | select(.name=="test-agent") | {claude_status}'
```

#### Test Case 6: Dashboard SSE Updates
**Setup:**
1. Open dashboard in browser
2. Open browser DevTools → Network tab
3. Start/stop agent worker

**Expected:**
- [ ] SSE connection established to `/v1/stream`
- [ ] "update" events received when agent status changes
- [ ] Dashboard auto-refreshes without manual reload
- [ ] Worker status badge updates (online ↔ offline)

**Manual Verify:**
- [ ] Watch Network tab for SSE events
- [ ] Observe real-time badge color changes

#### Test Case 7: Multiple Agents
**Setup:**
1. Start 2-3 agent workers with different `AGENT_STATE_DIR`
2. Assign tasks to different channels
3. Observe dashboard

**Expected:**
- [ ] All agents visible in dashboard
- [ ] Each shows independent worker/Claude status
- [ ] Worker online/offline reflects actual process state
- [ ] Claude status reflects individual task execution

**Verify:**
```bash
# Start multiple workers
AGENT_STATE_DIR=agent/data/instance1 COORDINATOR_URL=http://localhost:8080 \
node agent/clw-agent.js work --channel backend --tmux-target claude-code:1.0 &

AGENT_STATE_DIR=agent/data/instance2 COORDINATOR_URL=http://localhost:8080 \
node agent/clw-agent.js work --channel frontend --tmux-target claude-code:2.0 &

# Check dashboard
curl -s http://localhost:8080/v1/agents | jq '.agents[] | {name, worker_status, claude_status}'
```

### Phase 4: Edge Cases

#### Edge Case 1: Stale Heartbeat Recovery
**Setup:**
1. Worker online, Claude running
2. Simulate network partition (firewall or coordinator down)
3. Wait 35 seconds
4. Restore network

**Expected:**
- [ ] During partition: Worker status becomes `offline`
- [ ] After recovery: First heartbeat updates `last_seen`
- [ ] Worker status returns to `online`

#### Edge Case 2: Rapid Status Changes
**Setup:**
1. Create task, claim immediately, complete immediately
2. Observe status during rapid transitions

**Expected:**
- [ ] No race conditions or status flapping
- [ ] Final state consistent: Task done, Claude idle

#### Edge Case 3: Database Restart
**Setup:**
1. Agents running
2. Restart database (if using Postgres)
3. Coordinator reconnects

**Expected:**
- [ ] Agents continue heartbeating
- [ ] Status data persisted (Postgres) or reset (memory)
- [ ] Dashboard recovers gracefully

### Phase 5: Performance

#### Performance Test 1: Many Agents
**Setup:**
1. Simulate 10+ agents via concurrent heartbeat requests
2. Monitor coordinator CPU/memory

**Expected:**
- [ ] Coordinator handles load without degradation
- [ ] Dashboard API responses < 500ms
- [ ] SSE connections stable

**Verify:**
```bash
# Simulate 20 agents
for i in {1..20}; do
  curl -X POST http://localhost:8080/v1/agents/heartbeat \
    -H 'Content-Type: application/json' \
    -d "{
      \"agent_id\": \"load-test-agent-$i\",
      \"name\": \"Load Test Agent $i\",
      \"claude_status\": \"idle\"
    }" &
done
wait

# Check dashboard load time
time curl -s http://localhost:8080/v1/dashboard > /dev/null
```

#### Performance Test 2: Dashboard Cache
**Setup:**
1. Clear cache (restart coordinator)
2. Call dashboard API twice rapidly

**Expected:**
- [ ] First call: Computes from scratch
- [ ] Second call: Returns cached response (within 1 second TTL)
- [ ] Cache invalidated on agent heartbeat

**Verify:**
```bash
# Check cache behavior (should be fast)
time curl -s http://localhost:8080/v1/dashboard > /dev/null
time curl -s http://localhost:8080/v1/dashboard > /dev/null  # Should be faster

# Send heartbeat (invalidates cache)
curl -X POST http://localhost:8080/v1/agents/heartbeat \
  -H 'Content-Type: application/json' \
  -d '{"agent_id": "test", "name": "Test", "claude_status": "idle"}'

# Dashboard call should recompute
time curl -s http://localhost:8080/v1/dashboard > /dev/null
```

## Rollback Plan

If issues arise, rollback in reverse order:

### Rollback Phase 3: Revert Agent
1. Stop updated agent: `pkill -f "clw-agent.js"`
2. Checkout previous agent code: `git checkout HEAD~1 agent/`
3. Restart old agent: Old code sends only `status`, coordinator maps to `claude_status`

### Rollback Phase 2: Revert Coordinator
1. Stop coordinator: `Ctrl+C` or `pkill -f coordinator`
2. Checkout previous coordinator code: `git checkout HEAD~1 coordinator/`
3. Rebuild and restart: `cd coordinator && go build ./cmd/coordinator && ./coordinator`

### Rollback Phase 1: Revert Database (if needed)
```sql
-- If migration causes issues, revert schema:
ALTER TABLE public.agents DROP COLUMN IF EXISTS claude_status;
DROP INDEX IF EXISTS idx_agents_claude_status;

-- Note: This breaks new coordinator. Only use if blocking production.
```

## Success Criteria

All checkboxes checked:
- [ ] Coordinator builds and runs without errors
- [ ] Database migration applies successfully
- [ ] Old agents work (backward compatibility)
- [ ] New agents send dual status
- [ ] Dashboard displays two status columns
- [ ] Worker online/offline accurate (30s threshold)
- [ ] Claude idle/running/waiting accurate
- [ ] No automatic status updates on task claim/complete
- [ ] SSE real-time updates work
- [ ] No race conditions or status flapping
- [ ] Performance acceptable (< 500ms dashboard API)

## Failure Scenarios

If any test fails:
1. **Check logs**: Coordinator stdout, agent stdout, browser console
2. **Verify database**: `SELECT * FROM agents;` (check `claude_status` column)
3. **Test API**: Use curl to isolate frontend vs. backend issues
4. **Rollback**: Use rollback plan above
5. **Report**: Open issue with logs, test case, and environment details

## References

- Implementation: `IMPLEMENTATION_SUMMARY.md`
- Architecture: `docs/dual-status-architecture.md`
- Runbook: `RUNBOOK.md`
