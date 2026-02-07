# Local Agent (bridge)

목표: 기존 `Claude-Code-Remote/`의 기능(알림/인바운드/주입)을 **그대로 유지**하면서,
Coordinator(Go)로 **상태/이벤트를 업로드**하는 “브릿지” 역할을 하는 로컬 에이전트 스크립트를 제공한다.

> 이 단계에서는 기존 레포(`Claude-Code-Remote/`)를 수정하지 않고, 별도 래퍼로 감싸는 방식부터 시작합니다.

## 구성

- `agent/clw-agent.js`
  - `heartbeat`: Coordinator에 heartbeat 전송
  - `hook <completed|waiting>`: Claude 훅에서 호출될 수 있는 래퍼
    - 먼저 `Claude-Code-Remote/claude-hook-notify.js`를 실행(기존 알림 유지)
    - 이후 Coordinator에 이벤트 업로드(실패해도 훅 자체는 실패시키지 않음)
  - `run`: 레거시 서비스(`Claude-Code-Remote/start-all-webhooks.js`)를 실행하고 주기적으로 heartbeat 전송
  - `work`: Coordinator task를 claim(FIFO) → tmux로 주입 → 훅 완료 시 자동 complete
    - `node agent/clw-agent.js work --channel <name> --tmux-target <target>`
    - 멀티 채널: `--channel "backend-domain,notify"`
    - `--tmux-target` 예: `claude-code`, `claude-code:1`, `claude-code:1.0`

## 환경변수

- `COORDINATOR_URL` (default: `http://localhost:8080`)
- `COORDINATOR_AUTH_TOKEN` (optional)
- `AGENT_ID` (optional; 없으면 `agent/data/agent-id.txt`에 생성/저장)
- `AGENT_NAME` (optional; default: hostname)
- `AGENT_CHANNELS` (optional; comma-separated subscriptions; e.g. `backend-domain,notify`)
- `AGENT_STATE_DIR` (optional)
  - 에이전트 state 저장 디렉토리 override.
  - 기본은 `agent/data/` 이지만, `work`/`hook`은 tmux target 기반으로 `agent/data/instances/*`를 자동 사용합니다(멀티 세션 분리).
- `AGENT_HEARTBEAT_INTERVAL_SEC` (default: `15`)
- `AGENT_WORK_POLL_INTERVAL_SEC` (default: `5`)

> 참고: `agent/clw-agent.js`는 실행 시 `Claude-Code-Remote/.env`가 있으면 이를 **best-effort로 읽어** `COORDINATOR_URL` 같은 값을 가져옵니다.

## 사용 예시

```bash
# 1) Heartbeat
COORDINATOR_URL=http://localhost:8080 node agent/clw-agent.js heartbeat

# 2) Claude hook wrapper
COORDINATOR_URL=http://localhost:8080 node agent/clw-agent.js hook completed
COORDINATOR_URL=http://localhost:8080 node agent/clw-agent.js hook waiting

# 3) Run legacy services + periodic heartbeat
COORDINATOR_URL=http://localhost:8080 node agent/clw-agent.js run

# 4) Worker (claim → inject → hook complete)
COORDINATOR_URL=http://localhost:8080 node agent/clw-agent.js work --channel backend-domain --tmux-target claude-code:1.0
```

## TODO

- 훅 설치(`~/.claude/settings.json`)를 `setup` 단계에서 이 래퍼로 자동 전환(옵션)
- Telegram/LINE/Email 인바운드 결과도 이벤트로 업로드
