# Task 33: Implement Remote Session Startup

- **User Story**: As a user, I want the coordinator to be able to request an agent to start a new Claude Code session, with my approval, so that I don't have to manually start `tmux` and `claude` every time.

## Implementation Details

### Agent (`clw-agent.js`)

- [x] **Optional Tmux Target**: Modified the `work` command to allow it to start without a `--tmux-target`.
- [x] **Setup Mode**: Implemented a new "setup mode" in the `work` loop. If no target is set, the agent polls for a task to initiate setup.
- [x] **Interactive User Prompt**: Created a CLI prompt (`promptForRunMode`) that asks the user to choose between "Automatic Mode" and "Manual Mode".
  - [x] **Automatic Mode**: The agent creates a new detached `tmux` session and starts `claude` automatically.
  - [x] **Manual Mode**: The agent prompts the user to enter the target of their manually created `tmux` session.
- [x] **Status Reporting**: Added `emitEvent` calls to report the user's choice, `tmux` session creation status, and command injection status to the Coordinator.
- [x] **Special Task Handling**: The agent now recognizes a task with `type: "request_claude_session"` and, after setting up the session, completes this task without injecting its content.

### Coordinator (`coordinator/...`)

- [x] **Database Migration**: Added a new migration file (`0007_add_type_to_tasks.sql`) to add a `type` column to the `tasks` table.
- [x] **Model Update**: Added the `Type` field to the `Task` struct in `internal/model/model.go`.
- [x] **Store Layer Update**: Updated all relevant functions in `internal/store/postgres/postgres.go` (`CreateTask`, `ListTasks`, `ClaimTask`, etc.) to correctly handle the new `Type` field.
- [x] **New API Endpoint**: Created a new handler `handleAgentsRequestSession` and registered it to the route `POST /v1/agents/request-session`. This endpoint creates a `request_claude_session` task on a given channel.

## Testing

- [x] **Agent**: Manually tested the interactive prompt flow for the `work` command by running it without a `--tmux-target` and confirming the automatic session creation and worker loop continuation.
- [ ] **Coordinator**: (Requires running the full stack) Test the `POST /v1/agents/request-session` endpoint to ensure it correctly queues the special task.
- [ ] **End-to-End**: A full end-to-end test is required to verify a headless agent picks up the request and initiates the user prompt correctly.
