# clwclw-monitor

Agent CLI for connecting local Claude Code sessions to a clwclw-monitor Coordinator server.

## Installation

```bash
npm install -g clwclw-monitor
```

## Quick Start

1. **Setup** (interactive wizard):
```bash
clwclw setup
```

This will create `~/.clwclw/.env` with your Coordinator URL, authentication token, and notification settings (Telegram, Email, LINE).

2. **Install Claude Code hooks** (optional, but recommended):
```bash
clwclw install-hooks
```

This adds hooks to `~/.claude/settings.json` so the agent automatically tracks task completion.

3. **Test connection**:
```bash
clwclw heartbeat
```

4. **Start a worker** (in a Claude Code session running in tmux):
```bash
clwclw work --channel backend-domain --tmux-target claude-code:1.0
```

The worker will poll the Coordinator for tasks and inject them into your tmux session.

## Commands

| Command | Description |
|---------|-------------|
| `clwclw setup` | Interactive configuration wizard |
| `clwclw install-hooks` | Install Claude Code hooks |
| `clwclw version` | Show version |
| `clwclw heartbeat` | Send heartbeat to Coordinator |
| `clwclw hook <type>` | Run hook (completed\|waiting) |
| `clwclw work [options]` | Start task worker |
| `clwclw run` | Start legacy services with heartbeat |

### Worker Options

```bash
clwclw work --channel <name> --tmux-target <target>
```

- `--channel`: Channel(s) to poll (comma-separated, e.g., `backend-domain,notify`)
- `--tmux-target`: tmux target for injection (e.g., `claude-code:1.0`)

## Configuration

Settings are stored in `~/.clwclw/.env`:

```bash
# Coordinator
COORDINATOR_URL=http://localhost:8080
COORDINATOR_AUTH_TOKEN=your-secret-token

# Agent
AGENT_NAME=my-agent

# Telegram (optional)
TELEGRAM_ENABLED=true
TELEGRAM_BOT_TOKEN=your-bot-token
TELEGRAM_CHAT_ID=your-chat-id

# Email (optional)
EMAIL_ENABLED=true
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=your-email@gmail.com
SMTP_PASS=your-app-password
EMAIL_TO=your-email@gmail.com

# LINE (optional)
LINE_ENABLED=true
LINE_CHANNEL_ACCESS_TOKEN=your-access-token
LINE_CHANNEL_SECRET=your-secret
LINE_USER_ID=your-user-id
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `COORDINATOR_URL` | `http://localhost:8080` | Coordinator server URL |
| `COORDINATOR_AUTH_TOKEN` | - | API authentication token |
| `CLWCLW_HOME` | `~/.clwclw` | Config directory |
| `AGENT_ID` | auto-generated | Agent UUID |
| `AGENT_NAME` | hostname | Human-readable agent name |
| `AGENT_CHANNELS` | - | Comma-separated channel subscriptions |
| `AGENT_STATE_DIR` | `~/.clwclw/data` | State directory |
| `AGENT_HEARTBEAT_INTERVAL_SEC` | `15` | Heartbeat frequency |
| `AGENT_WORK_POLL_INTERVAL_SEC` | `5` | Task polling interval |

## Multi-Session Support

Run multiple agents for different tmux sessions:

```bash
# Session 1 (backend)
AGENT_STATE_DIR=~/.clwclw/data/instances/backend \
clwclw work --channel backend-domain --tmux-target backend:1.0

# Session 2 (frontend)
AGENT_STATE_DIR=~/.clwclw/data/instances/frontend \
clwclw work --channel frontend-domain --tmux-target frontend:1.0
```

Each instance maintains isolated state (agent ID, claimed tasks).

## Coordinator Setup

The Coordinator is a separate Go server. See the [main repository](https://github.com/WithFortuna/clwclw-monitor) for deployment instructions.

Quick start (local development):

```bash
git clone https://github.com/WithFortuna/clwclw-monitor.git
cd clwclw-monitor
go run ./coordinator/cmd/coordinator
```

The dashboard will be available at http://localhost:8080/

## Legacy Features

This package includes full [Claude-Code-Remote](https://github.com/JessyTsui/Claude-Code-Remote) functionality:

- Desktop notifications with sound alerts
- Telegram bot with interactive buttons
- Email notifications with execution traces
- LINE messaging
- tmux/PTY command injection

Run legacy services with:

```bash
clwclw run
```

## Development

For contributors working from the repository checkout:

```bash
git clone https://github.com/WithFortuna/clwclw-monitor.git
cd clwclw-monitor

# Install dependencies for legacy remote
cd Claude-Code-Remote
npm install
cd ..

# Run agent directly (backward compatible)
node agent/clw-agent.js heartbeat
```

## License

MIT
