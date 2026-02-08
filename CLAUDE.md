# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 작업 규칙 (Working Rules)

**중요: 모든 작업은 다음 프로세스를 따라야 합니다.**

1. **새로운 기능 추가 시**: 반드시 `REQUIREMENTS.md`에 요구사항을 먼저 기록
2. **작업 시작 시**: `./tasks/` 디렉토리에 마크다운 파일로 작업 발행
3. **작업 진행**: 체크박스(`- [ ]`, `- [x]`)로 작업 여부 트래킹
4. **문서 작성**: 사용자가 명시적으로 요청하기 전까지 별도 문서 생성 금지

**작업 파일 형식 예시** (`tasks/YYYY-MM-DD-feature-name.md`):
```markdown
# [Feature Name]

## 요구사항
- REQUIREMENTS.md 참조: [섹션명]

## 작업 목록
- [ ] 작업 1
- [x] 작업 2 (완료)
- [ ] 작업 3

## 변경 파일
- `path/to/file1.go`
- `path/to/file2.js`
```

## Project Overview

**clwclw-monitor** is a web-based task coordination system for Claude Code agents. It extends [Claude-Code-Remote](https://github.com/JessyTsui/Claude-Code-Remote) with a centralized Go API server (Coordinator), web dashboard, and multi-agent task distribution.

### Architecture

The system has three main components:

1. **Coordinator** (`coordinator/`) - Go API server that manages agents, tasks, channels, and events
2. **Agent** (`agent/`) - Node.js bridge that connects local Claude Code sessions to the Coordinator
3. **Legacy Remote** (`Claude-Code-Remote/`) - Original notification/control system (email, Telegram, LINE, desktop)

### Key Concepts

- **Agent**: A local process monitoring a Claude Code session; sends heartbeats and events to Coordinator
- **Channel**: Logical task queue (e.g., `backend-domain`, `notify`)
- **Task**: Work unit published to a channel; agents claim tasks in FIFO order
- **Event**: Timeline entry representing agent activity (linked to tasks or standalone)
- **Task Input**: Command injection into tmux sessions for remote control

### Agent Status System

Agents have a dual status system (see [Issue #1](https://github.com/WithFortuna/clwclw-monitor/issues/1)):

1. **Worker Status** (computed from `last_seen` timestamp):
   - `online`: Heartbeat within 30 seconds (worker process alive)
   - `offline`: Heartbeat stale (worker process crashed/stopped)

2. **Claude Status** (reported by agent):
   - `idle`: No task assigned or not actively executing
   - `running`: Task assigned and Claude actively executing
   - `waiting`: Task in progress, waiting for interactive user input

**Key Design:** Task claim/complete operations do NOT automatically update agent status. Agent heartbeat is the sole source of truth for Claude execution state.

## Common Commands

### Coordinator (Go)

The Coordinator must be run from the repository root (which contains `go.work`):

```bash
# Run with in-memory store (default)
go run ./coordinator/cmd/coordinator

# Run with Postgres/Supabase
export COORDINATOR_DATABASE_URL='postgres://user:pass@host/db'
go run ./coordinator/cmd/coordinator

# Build binary
cd coordinator
go build -o coordinator ./cmd/coordinator

# Run tests
cd coordinator
go test ./...
```

**Environment variables:**
- `COORDINATOR_PORT` - Server port (default: `8080`)
- `COORDINATOR_AUTH_TOKEN` - Optional bearer token for API auth
- `COORDINATOR_DATABASE_URL` - Postgres connection string (falls back to `DATABASE_URL`)
- `COORDINATOR_EVENT_RETENTION_DAYS` - Event retention period (default: `30`, `0` to disable)
- `COORDINATOR_RETENTION_INTERVAL_HOURS` - Purge interval (default: `24`)

### Agent (Node.js)

Agent commands assume `Claude-Code-Remote/` dependencies are installed:

```bash
# Initial setup (from Claude-Code-Remote/)
cd Claude-Code-Remote
npm install
node setup.js  # Interactive wizard for .env and hooks

# Agent operations (from repo root)
COORDINATOR_URL=http://localhost:8080 node agent/clw-agent.js heartbeat
COORDINATOR_URL=http://localhost:8080 node agent/clw-agent.js hook completed
COORDINATOR_URL=http://localhost:8080 node agent/clw-agent.js work --channel backend-domain --tmux-target claude-code:1.0

# Run legacy services with heartbeat
COORDINATOR_URL=http://localhost:8080 node agent/clw-agent.js run
```

**Agent environment variables:**
- `COORDINATOR_URL` - Coordinator API endpoint (default: `http://localhost:8080`)
- `COORDINATOR_AUTH_TOKEN` - Optional API token
- `AGENT_ID` - Agent identifier (auto-generated to `agent/data/agent-id.txt`)
- `AGENT_NAME` - Human-readable name (default: hostname)
- `AGENT_CHANNELS` - Comma-separated channel subscriptions
- `AGENT_STATE_DIR` - State directory override for multi-session support
- `AGENT_HEARTBEAT_INTERVAL_SEC` - Heartbeat frequency (default: `15`)
- `AGENT_WORK_POLL_INTERVAL_SEC` - Task polling interval (default: `5`)

### Legacy Remote (Claude-Code-Remote)

```bash
cd Claude-Code-Remote

# Setup
npm run setup              # Interactive configuration wizard
npm install               # Install dependencies

# Start services
npm run webhooks          # All enabled platforms (Telegram, Email, LINE)
npm run telegram          # Telegram webhook only
npm run line             # LINE webhook only
npm run daemon:start     # Email IMAP listener

# Testing
node claude-hook-notify.js completed    # Test all notification channels
node test-telegram-notification.js       # Test Telegram
node test-injection.js                   # Test tmux injection
node claude-remote.js diagnose          # System diagnostics
```

### Development Workflow

```bash
# 1. Start Coordinator
go run ./coordinator/cmd/coordinator

# 2. Access dashboard
open http://localhost:8080/

# 3. Create channel and tasks via UI or API
curl -X POST http://localhost:8080/v1/channels \
  -H 'Content-Type: application/json' \
  -d '{"name":"test-channel","description":"Test"}'

# 4. Start worker agent
COORDINATOR_URL=http://localhost:8080 \
node agent/clw-agent.js work --channel test-channel --tmux-target claude-code:1.0

# 5. Test notification flow
node Claude-Code-Remote/claude-hook-notify.js completed
```

## Architecture Details

### Coordinator API Design

The Coordinator (`coordinator/internal/httpapi/`) exposes RESTful endpoints and an SSE stream for real-time updates:

**Core Endpoints:**
- `GET /` - Dashboard UI (static HTML)
- `GET /health` - Health check
- `GET /v1/stream` - SSE stream for real-time dashboard updates

**Agent Management:**
- `POST /v1/agents/heartbeat` - Update agent last_seen and status
- `GET /v1/agents` - List all agents

**Channel & Task Management:**
- `POST /v1/channels`, `GET /v1/channels`
- `POST /v1/tasks`, `GET /v1/tasks`
- `POST /v1/tasks/claim` - FIFO task claiming (atomic)
- `POST /v1/tasks/assign` - Manual task assignment
- `POST /v1/tasks/complete`, `POST /v1/tasks/fail`

**Event Timeline:**
- `POST /v1/events` - Create event with optional idempotency
- `GET /v1/events` - List events (filterable by agent/task)

**Task Inputs (Remote Injection):**
- `POST /v1/task-inputs` - Create command injection request
- `POST /v1/task-inputs/claim` - Agent claims next pending input

**Dashboard:**
- `GET /v1/dashboard` - Aggregated state snapshot (cached)

### Storage Layer

The `coordinator/internal/store/` package defines a `Store` interface with two implementations:

1. **Memory Store** (`store/memory/`) - Default; ephemeral, no persistence
2. **Postgres Store** (`store/postgres/`) - Production; uses Supabase/Postgres with schema in `supabase/migrations/`

**FIFO Task Claiming:**
The Postgres implementation uses `FOR UPDATE SKIP LOCKED` to prevent race conditions:
```sql
-- See supabase/migrations/0001_init.sql: claim_task() function
SELECT ... WHERE status='queued' ORDER BY created_at ASC FOR UPDATE SKIP LOCKED LIMIT 1
```

### Multi-Session Agent Support

Agents support multiple concurrent Claude Code sessions via `AGENT_STATE_DIR`:

```bash
# Session 1
AGENT_STATE_DIR=agent/data/instances/session1 \
node agent/clw-agent.js work --channel backend --tmux-target claude-code:1.0

# Session 2
AGENT_STATE_DIR=agent/data/instances/session2 \
node agent/clw-agent.js work --channel frontend --tmux-target claude-code:2.0
```

Each instance maintains isolated state (agent ID, claimed tasks, idempotency keys).

### Legacy Integration

The agent bridge (`agent/clw-agent.js`) preserves **all** Claude-Code-Remote functionality:
- Email notifications with execution traces (SMTP/IMAP)
- Telegram bot with interactive buttons
- LINE messaging with token-based commands
- Desktop notifications with sound alerts
- tmux/PTY command injection
- Session token management and 24-hour expiration

**Hook Integration:**
Claude Code hooks call `agent/clw-agent.js hook completed|waiting`, which:
1. Executes original `Claude-Code-Remote/claude-hook-notify.js` (preserves notifications)
2. Uploads completion event to Coordinator API (best-effort; failures don't block hooks)

### Idempotency Strategy

- **Events**: `POST /v1/events` with `idempotency_key` prevents duplicate timeline entries (200 with `{"deduped": true}`)
- **Task Claims**: `POST /v1/tasks/claim` with `idempotency_key` returns same task on retry
- **Task Completion**: Transition guards prevent duplicate state changes (idempotent by design)

## File Structure

```
.
├── coordinator/              # Go API server
│   ├── cmd/coordinator/      # Main entry point
│   ├── internal/
│   │   ├── config/           # Environment config loading
│   │   ├── httpapi/          # HTTP handlers, SSE bus, dashboard
│   │   ├── model/            # Domain models (Agent, Task, Event, etc.)
│   │   └── store/            # Storage interface + implementations
│   │       ├── memory/       # In-memory store (default)
│   │       └── postgres/     # Postgres/Supabase store
│   └── go.mod
├── agent/                    # Local agent bridge
│   ├── clw-agent.js          # Main agent script
│   └── data/                 # State directory (agent ID, instances)
├── Claude-Code-Remote/       # Legacy remote control system
│   ├── src/                  # Notification channels, relay, automation
│   ├── setup.js              # Interactive configuration wizard
│   ├── start-all-webhooks.js # Multi-platform webhook server
│   └── claude-hook-notify.js # Hook entry point for notifications
├── supabase/
│   └── migrations/           # Postgres schema definitions
│       ├── 0001_init.sql     # Core tables (agents, channels, tasks, events)
│       └── 0002_task_inputs.sql # Remote injection table
├── tasks/                    # Development task tracking (markdown)
├── go.work                   # Go workspace (includes coordinator/)
├── REQUIREMENTS.md           # Full system requirements (Korean)
└── RUNBOOK.md                # Local acceptance test guide
```

## Important Patterns

### Task Lifecycle

1. **Creation**: Task created with `status=queued`, `priority=0` (lower = higher priority)
2. **Claiming**: Agent claims via FIFO (oldest `created_at` first); task transitions to `in_progress`
3. **Execution**: Agent injects task into tmux session; Claude Code processes command
4. **Completion**: Hook detects Claude stop event → agent calls complete/fail → task transitions to `done`/`failed`

### Event Timeline

Events capture agent activity with optional task association:
- `task_id` is **nullable** - events can exist without tasks (e.g., user commands, heartbeats)
- Common event types: `task.claimed`, `task.completed`, `task.failed`, `heartbeat`, `command.executed`
- Payloads are arbitrary JSON (e.g., `{"msg": "...", "trace": "..."}`)

### Real-Time Updates

Dashboard uses SSE (`GET /v1/stream`) to receive state changes:
```javascript
const eventSource = new EventSource('/v1/stream');
eventSource.addEventListener('update', e => {
  const data = JSON.parse(e.data);
  // data.type: 'agent_updated', 'task_updated', 'event_created', etc.
});
```

The Coordinator broadcasts updates via an in-memory event bus (`internal/httpapi/bus.go`).

## Testing & Diagnostics

### Smoke Tests

```bash
# Test Coordinator health
curl -s http://localhost:8080/health | cat

# Create channel
curl -X POST http://localhost:8080/v1/channels \
  -H 'Content-Type: application/json' \
  -d '{"name":"test","description":"Test channel"}'

# Create task
curl -X POST http://localhost:8080/v1/tasks \
  -H 'Content-Type: application/json' \
  -d '{"channel_id":"<uuid>","title":"Test task","description":"Details"}'

# Agent heartbeat
COORDINATOR_URL=http://localhost:8080 node agent/clw-agent.js heartbeat

# Upload event
AGENT_ID="$(cat agent/data/agent-id.txt | tr -d '\n')"
curl -X POST http://localhost:8080/v1/events \
  -H 'Content-Type: application/json' \
  -d "{\"agent_id\":\"${AGENT_ID}\",\"type\":\"test\",\"payload\":{\"msg\":\"hello\"},\"idempotency_key\":\"test:1\"}"
```

### Legacy System Tests

```bash
cd Claude-Code-Remote

# Test all notification channels
node claude-hook-notify.js completed

# Test Telegram bot
node test-telegram-notification.js

# Test email flow
node test-real-notification.js

# Diagnose system state
node claude-remote.js diagnose
```

### Full Integration Test

See `RUNBOOK.md` for complete acceptance test procedures including tmux injection and Claude Code hook verification.

## Database Migrations

Schema changes should be added as new files in `supabase/migrations/`:

```bash
# Example: Add new column
cat > supabase/migrations/0003_add_task_metadata.sql <<'EOF'
alter table public.tasks add column metadata jsonb not null default '{}'::jsonb;
EOF
```

**Schema Source of Truth**: `supabase/migrations/0001_init.sql` and `0002_task_inputs.sql`

**Retention Policy**: Events older than 30 days are purged automatically (configurable via `COORDINATOR_EVENT_RETENTION_DAYS`).

## Security Considerations

### Authentication

- Coordinator supports optional shared token via `COORDINATOR_AUTH_TOKEN`
- Agents/UI must provide `Authorization: Bearer <token>` or `X-Api-Key: <token>` header
- **Production**: Use strong random tokens; rotate regularly

### Legacy Remote Security

- Email: Sender whitelist (`ALLOWED_SENDERS` in `.env`)
- Telegram: Bot token + chat ID verification
- LINE: Channel secret + access token validation
- Session tokens: 8-character alphanumeric, 24-hour expiration
- Hooks: Validate session tokens before injecting commands

### Tmux Injection

- Agents inject commands using `tmux send-keys`
- **Risk**: Malicious commands can execute arbitrary code in Claude sessions
- **Mitigation**: Validate task sources, use session token whitelists, audit event logs

## Development Notes

- **Go Version**: 1.22+ required (see `go.work`)
- **Node.js**: 14+ required for agent and legacy remote
- **Dependencies**: Coordinator uses minimal deps (`jackc/pgx/v5` for Postgres); agent has zero npm dependencies
- **Concurrency**: Coordinator handles concurrent API requests; FIFO claims use database-level locking
- **Error Handling**: Agent failures are best-effort; hook failures don't block Claude Code operation

## References

- **Requirements**: See `REQUIREMENTS.md` for full system specification (Korean)
- **Runbook**: See `RUNBOOK.md` for local testing procedures
- **Task Tracker**: See `tasks/*.md` for development roadmap and completed work
- **Legacy Docs**: See `Claude-Code-Remote/README.md` for original remote control documentation
