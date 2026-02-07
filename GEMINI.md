# CLWCLW-Monitor Project Overview

This project, `clwclw-monitor`, is designed to extend the existing `Claude-Code-Remote` functionality by introducing a centralized Coordinator, a web-based UI (webview), and a task board. The primary goal is to provide an internet-accessible dashboard for monitoring agent status, task history, and distributing tasks to multiple agents.

## Project Architecture and Components

The system comprises three main logical components:

1.  **Coordinator (Go)**:
    *   A Go-based API server responsible for central management of agent status, task history, and task distribution.
    *   It exposes various API endpoints for agents to report status, ingest events, and claim tasks, as well as endpoints for the web UI.
    *   Uses Supabase (Postgres) for persistent storage, with an in-memory option for development.
    *   Provides real-time updates to the dashboard via Server-Sent Events (SSE).

2.  **Agent (Node.js)**:
    *   Based on the `Claude-Code-Remote` codebase, this is a local execution process.
    *   It observes and controls Claude Code sessions (e.g., via tmux/pty injection).
    *   Collects logs, sends notifications via various channels (Email, Telegram, LINE, Desktop), and reports its status and events to the Coordinator API.
    *   Claims tasks from the Coordinator.

3.  **Webview Dashboard**:
    *   An internet-accessible UI for monitoring the system.
    *   Displays active agents, their statuses, current tasks, and historical timelines.
    *   Provides a task board for managing and distributing tasks across channels.

## Key Technologies

*   **Go**: For the Coordinator API server.
*   **Node.js**: For the Agent processes (inheriting from `Claude-Code-Remote`).
*   **Supabase (Postgres)**: For central data storage, managing agents, channels, tasks, and events.

## Task Distribution and Agent Communication

Agents retrieve tasks from the Coordinator by making explicit API calls to the `POST /v1/tasks/claim` endpoint. This is a request-response mechanism, where an agent requests a task from the Coordinator. This claim process is designed to be atomic and FIFO (First-In, First-Out) to prevent duplicate task assignments. The Coordinator API supports idempotency for task claiming to handle retries robustly.

The webview dashboard receives real-time updates on agent status and task changes from the Coordinator using Server-Sent Events (SSE), ensuring the UI reflects the current state of the system efficiently. Agents, however, proactively communicate with the Coordinator to report status and claim tasks.

## Building and Running

### 1. Supabase/Database Setup

The project uses Supabase (Postgres) for its database. The schema is defined in `supabase/migrations/`.
*   Review the SQL migration files (e.g., `0001_init.sql`, `0002_task_inputs.sql`, `0003_dual_status.sql`) in `supabase/migrations/` to understand the database structure.
*   You will need a running Supabase instance (or a local Postgres database) and apply these migrations.

### 2. Coordinator (Go)

The Coordinator is built with Go.

**Prerequisites:**
*   Go (version specified in `go.mod` if any, or latest stable).

**To run the Coordinator:**

1.  Navigate to the `coordinator` directory: `cd coordinator`
2.  Set necessary environment variables (e.g., `COORDINATOR_PORT`, `COORDINATOR_AUTH_TOKEN`, `COORDINATOR_DATABASE_URL`). If `COORDINATOR_DATABASE_URL` is not set, it will use an in-memory store.
    ```bash
    # Example for running with in-memory store on port 8080
    COORDINATOR_PORT=8080 COORDINATOR_AUTH_TOKEN=devtoken go run ./cmd/coordinator
    ```
    For production, you would set `COORDINATOR_DATABASE_URL` to your Supabase connection string.

### 3. Agent (Node.js - Claude-Code-Remote)

The Agent functionality is based on the `Claude-Code-Remote` Node.js project.

**Prerequisites:**
*   Node.js (>= 14.0.0).
*   `tmux` (if using tmux injection mode).

**To set up and run an Agent:**

1.  Navigate to the `Claude-Code-Remote` directory: `cd Claude-Code-Remote`
2.  Install dependencies: `npm install`
3.  Run the interactive setup (recommended to generate `.env` and configure Claude hooks): `npm run setup`
    *   Alternatively, manually configure `.env` by copying `.env.example` and editing it.
    *   Ensure Claude Code hooks are correctly configured (either globally in `~/.claude/settings.json` or project-specific via `CLAUDE_HOOKS_CONFIG`).
4.  Start Claude Code (e.g., in tmux or default PTY mode) with the configured hooks.
5.  Start the Agent's webhook services: `npm run webhooks` (this will start all enabled notification platforms and implicitly connect to the Coordinator, though specific Coordinator integration points need to be confirmed in `clwclw-monitor`'s agent codebase).

### 4. Webview UI

*   The Coordinator's `GET /` endpoint serves a static dashboard (`coordinator/internal/httpapi/ui/app.js` is mentioned).
*   The UI will poll API endpoints and use `GET /v1/stream` (SSE) for real-time updates.

## Development Conventions

*   **API-First Design**: The Coordinator defines a clear API for interaction.
*   **Idempotency**: Critical API operations (like task claiming and event ingestion) are designed with idempotency to ensure reliable retries.
*   **Modular Architecture**: Separation of concerns between Coordinator (Go), Agent (Node.js), and database (Supabase).
*   **Test-Driven (implied)**: `REQUIREMENTS.md` mentions smoke/regression tests for existing features.
*   **Database Migrations**: Supabase migrations are used for schema management.
