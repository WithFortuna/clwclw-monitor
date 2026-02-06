# Runbook — Local UI / Acceptance Test

이 문서는 `clwclw-monitor`를 로컬에서 실행해 **대시보드 화면을 띄우고**, 핵심 플로우를 사용자가 직접 인수테스트하는 절차를 정리합니다.

## 0) 사전 준비

- Go: `coordinator/go.mod` 기준 **Go 1.22+** 필요
- Node.js: 레거시/agent 실행용(현재 환경에는 Node가 있음)
- (선택) tmux: task 주입/멀티 세션 라우팅 테스트용

> 참고: 이 워크스페이스 환경에서는 `go`가 설치되어 있지 않습니다(`go version`이 실패).

## 1) Coordinator(대시보드) 실행

### 1.1 메모리 저장소(가장 간단)

레포 루트에서(이 레포는 `go.work`로 서브모듈 `./coordinator`를 포함):

```bash
go run ./coordinator/cmd/coordinator
```

또는 `coordinator/`로 이동해서:

```bash
cd coordinator
go run ./cmd/coordinator
```

기본 포트는 `8080` 입니다.

헬스 체크:

```bash
curl -s http://localhost:8080/health | cat
```

대시보드 접속:

- 브라우저에서 `http://localhost:8080/` 열기

### 1.2 Supabase(Postgres) 저장소(선택)

```bash
export COORDINATOR_DATABASE_URL='postgres://...'
go run ./coordinator/cmd/coordinator
```

> 스키마는 `supabase/migrations/0001_init.sql` 기준입니다.

## 2) “화면”에서 기본 인수테스트(권장 순서)

### 2.1 UI에서 채널/태스크 생성

1) `http://localhost:8080/` 접속  
2) **Create Channel** 버튼으로 채널 생성(예: `backend-domain`)  
3) **Create Task** 버튼으로 태스크 생성  
4) 보드에서 `queued / in_progress / done / failed` 상태 변화가 보이는지 확인

### 2.2 Agent(heartbeat)로 “에이전트 등장” 확인

다른 터미널에서:

```bash
COORDINATOR_URL=http://localhost:8080 node agent/clw-agent.js heartbeat
```

UI의 Agents 표에 agent가 나타나고 `last_seen`이 갱신되는지 확인합니다.

### 2.3 Event 업로드로 “타임라인” 확인(최소 테스트)

agent id는 기본적으로 `agent/data/agent-id.txt`에 생성됩니다.

```bash
AGENT_ID="$(cat agent/data/agent-id.txt | tr -d '\n')"
curl -sS -X POST http://localhost:8080/v1/events \
  -H 'Content-Type: application/json' \
  -d "{\"agent_id\":\"${AGENT_ID}\",\"type\":\"acceptance.test\",\"payload\":{\"msg\":\"hello\"},\"idempotency_key\":\"acceptance.test:1\"}" | cat
```

UI의 Timeline에 이벤트가 보이는지 확인합니다.

## 3) (옵션) tmux 주입 기반 end-to-end 테스트

> 이 단계는 “task claim → tmux send-keys 주입 → (훅) 완료 처리”까지 확인하고 싶을 때만 수행하세요.

### 3.1 tmux target 확인

tmux pane 안에서:

```bash
tmux display-message -p '#S:#I.#P'
```

출력 예: `claude-code:1.0`

### 3.2 Worker 실행(채널 claim → 주입)

```bash
COORDINATOR_URL=http://localhost:8080 node agent/clw-agent.js work --channel backend-domain --tmux-target claude-code:1.0
```

UI에서 `queued` task가 `in_progress`로 넘어가고, 해당 tmux pane에 `[TASK] ...`가 입력되는지 확인합니다.

### 3.3 (선택) Claude hooks 연동(자동 complete)

Claude hooks가 `agent/clw-agent.js hook completed|waiting`을 호출하도록 설치되어 있어야 자동 complete가 됩니다.

- 레거시 설치(훅 설정 merge + `.env` 생성):  
  ```bash
  cd Claude-Code-Remote
  npm install
  node setup.js
  ```

> 주의: 위 설치는 사용자 홈의 `~/.claude/settings.json`을 변경할 수 있으니, 필요하면 백업 후 진행하세요.

완료 후 Claude Code가 Stop/SubagentStop 이벤트를 발생시키면:
- 훅이 실행되고
- in-flight task가 자동으로 `done` 처리되는지(UI에서) 확인합니다.

## 4) Troubleshooting (자주 겪는 문제)

- UI가 안 뜸: Coordinator가 떠있는지(`curl http://localhost:8080/health`) 확인.
- 이벤트가 UI에 안 보임: `COORDINATOR_AUTH_TOKEN`을 켰다면 UI에 API Key를 입력했는지 확인(또는 auth를 끄고 재시도).
- tmux 주입이 안 됨: `--tmux-target` 값이 실제 존재하는지(`tmux list-panes -a`) 확인.
- 훅이 실패함: `Claude-Code-Remote/.env`가 존재하는지 확인(없으면 레거시 훅 스크립트가 종료할 수 있음).
