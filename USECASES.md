# Use Cases (Living)

> Last updated: **2026-02-03**  
> 규칙: 각 항목은 **1문장**, 기능 추가/변경 시 항목을 추가/수정한다.

## 0) Scope / Architecture

1. 이 레포는 레거시 `Claude-Code-Remote/`를 로컬 런타임으로 유지하면서 Go Coordinator/대시보드/agent 래퍼를 옆에 붙이는 strangler(hybrid) 방식으로 동작한다.
1. Coordinator/agent 래퍼가 없어도 레거시만으로 알림/원격 명령(인바운드) 기능이 동작하고, Coordinator 연동은 설정 시에만 추가로 동작한다.

## A) Coordinator (Go) / Web Dashboard

1. 운영자가 Coordinator를 실행하면 `/health`로 헬스 체크를 확인할 수 있다.
1. 사용자가 `/`로 접속하면 Coordinator에 내장된 웹 대시보드(UI)를 확인할 수 있다.
1. 운영자가 `COORDINATOR_AUTH_TOKEN`을 설정하면 Coordinator API는 토큰 기반 인증으로 보호된다.
1. 운영자가 인증을 켠 상태에서도 대시보드(UI)는 무인증으로 로드되고, API 호출은 API Key 입력으로 수행된다.
1. 대시보드가 인증이 필요한 환경에서는 사용자가 UI에서 API Key를 저장(localStorage)해 이후 요청에 `X-Api-Key`를 포함한다.
1. 대시보드는 실시간 트리거를 위해 `GET /v1/stream`(SSE)을 구독하고 변경 이벤트가 오면 화면을 갱신한다.
1. 인증이 켜진 환경에서 대시보드는 SSE(EventSource) 제약 때문에 `api_key` 쿼리로 키를 전달해 스트림 연결을 유지할 수 있다.
1. 대시보드는 동접 조회 부하를 줄이기 위해 `GET /v1/dashboard` 단일 스냅샷 API로 agents/channels/tasks/events를 한 번에 조회한다.
1. Coordinator는 `GET /v1/dashboard` 응답을 짧은 TTL로 캐시해 1,000 동접 조회를 견딜 수 있게 한다.
1. 에이전트가 `POST /v1/agents/heartbeat`를 보내면 Coordinator는 agent의 `last_seen/status/current_task`를 업데이트한다.
1. 운영자가 `GET /v1/agents`로 에이전트 목록을 조회하면 현재 등록된 agent들을 확인할 수 있다.
1. 사용자는 대시보드에서 활성 에이전트(상태/마지막 접속/현재 작업)를 확인할 수 있다.
1. 사용자는 대시보드에서 에이전트의 채널 구독 정보(`meta.subscriptions`)를 확인할 수 있다.
1. 사용자는 대시보드에서 에이전트의 tmux 정보(`meta.tmux_target` 또는 `meta.tmux_session`)를 확인할 수 있다.
1. 운영자가 `POST /v1/channels`로 채널을 생성하면 작업 영역(예: `backend-domain`)을 논리적으로 구분할 수 있다.
1. 운영자가 `GET /v1/channels`로 채널 목록을 조회하면 등록된 작업 영역을 확인할 수 있다.
1. 사용자는 대시보드에서 채널 목록을 보고 채널별 Task 보드를 확인할 수 있다.
1. 사용자는 대시보드에서 “Create Channel”로 채널을 생성할 수 있다.
1. 운영자가 `POST /v1/tasks`로 task를 생성하면 해당 채널의 `queued` 목록에 작업이 쌓인다.
1. 운영자가 `GET /v1/tasks?channel_id=...&status=...`로 task를 조회하면 채널/상태별로 필터링할 수 있다.
1. Coordinator는 task 상태를 `queued → in_progress → done/failed`로 관리한다.
1. 에이전트가 `POST /v1/tasks/claim`으로 FIFO claim을 하면 Coordinator는 원자적으로 다음 `queued` task를 `in_progress`로 전이한다.
1. 운영자가 “수동 할당”으로 `POST /v1/tasks/assign`을 호출하면 특정 `queued` task를 특정 `agent_id`에 강제로 할당할 수 있다.
1. 사용자는 대시보드에서 `queued` task를 “Assign” 버튼으로 특정 에이전트에 수동 할당할 수 있다.
1. 사용자는 대시보드에서 “Create Task”로 특정 채널에 task를 생성할 수 있다.
1. 사용자는 대시보드에서 `in_progress` task를 “Mark Done” 버튼으로 `done` 처리할 수 있다.
1. 사용자는 대시보드에서 `in_progress` task를 “Fail” 버튼으로 `failed` 처리할 수 있다.
1. 사용자가 `POST /v1/tasks/inputs`로 태스크 입력(응답)을 생성하면 in-flight task에 대한 사용자 선택/입력을 에이전트에게 전달할 수 있다.
1. 에이전트가 `POST /v1/tasks/inputs/claim`으로 입력을 claim하면 해당 입력이 소비되고 중복 주입을 방지할 수 있다.
1. 에이전트/레거시가 `POST /v1/events`로 작업 이력(event)을 업로드하면 대시보드의 타임라인에 기록된다.
1. 운영자가 `GET /v1/events?agent_id=...&task_id=...&limit=...`로 event를 조회하면 필터/limit 기반 조회가 가능하다.
1. Coordinator는 event 중복 업로드를 `(agent_id, idempotency_key)` 기준으로 dedupe하고 중복은 에러 대신 `deduped`로 처리한다.
1. Coordinator는 claim 재시도에서 `idempotency_key`가 같으면 동일 task를 반환해 “재시도마다 다른 task를 가져가는” 문제를 방지한다.
1. Coordinator는 complete/fail 재시도에서 이미 완료/실패 상태면 시간 값 등을 바꾸지 않고 현재 상태를 반환한다.
1. Coordinator는 이벤트 보관 정책을 위해 30일 지난 events를 주기적으로 purge한다.
1. Coordinator는 저장소를 in-memory 또는 Supabase(Postgres)로 선택할 수 있다.

## B) Local Agent (Bridge / Worker)

1. 사용자가 `node agent/clw-agent.js heartbeat`를 실행하면 로컬 에이전트가 Coordinator에 heartbeat를 전송한다.
1. Claude 훅이 `node agent/clw-agent.js hook completed|waiting`을 실행하면 레거시 알림을 먼저 실행한 뒤 Coordinator에 이벤트/heartbeat를 best-effort로 업로드한다.
1. 로컬 에이전트는 `Claude-Code-Remote/.env`를 best-effort로 읽어 `COORDINATOR_URL` 같은 설정을 자동으로 로드할 수 있다.
1. 로컬 에이전트는 `agent/data/agent-id.txt`(또는 tmux target별 state dir)에 고정 UUID를 저장해 재시작해도 동일 agent_id로 동작한다.
1. 사용자가 `node agent/clw-agent.js run`을 실행하면 레거시 웹훅/데몬 서비스를 실행하면서 주기적으로 heartbeat를 전송한다.
1. 사용자가 `node agent/clw-agent.js work --channel <name> --tmux-target <target>`를 실행하면 해당 채널에서 FIFO claim → tmux 주입 → 훅 완료 시 자동 complete까지 수행한다.
1. 사용자가 `--channel "a,b,c"`처럼 여러 채널을 지정하면 워커는 채널을 순회하며 작업을 claim한다(단일 in-flight 가정).
1. 워커는 claim한 task를 `agent/data/current-task.json`(또는 tmux target별 state dir)로 기록해 훅과 연동한다.
1. 훅 `completed`가 실행되면 워커가 기록한 in-flight task를 `POST /v1/tasks/complete`로 자동 완료 처리한다.
1. 워커는 tmux target(`session`, `session:window`, `session:window.pane`)에 `tmux send-keys`로 task를 주입할 수 있다.
1. 워커는 tmux target별 state dir(`agent/data/instances/*`)로 agent id/current-task를 분리해 한 머신에서 여러 Claude 세션(예: 5개)을 각각 별도 agent로 관측할 수 있다.
1. 훅은 현재 tmux target을 감지해 동일 state dir를 사용함으로써 “작업 완료 → 올바른 task complete”를 보장한다.
1. 에이전트는 heartbeat `meta.subscriptions`로 자신이 작업 가능한 채널(구독)을 Coordinator에 알릴 수 있다.
1. 워커는 in-flight task 수행 중 tmux output을 캡처해 Claude Code의 인터랙티브 선택지(prompt)를 best-effort로 감지하고 `task.prompt` 이벤트로 업로드할 수 있다.
1. 워커는 `Enter to select · Tab/Arrow keys to navigate · Esc to cancel` 형태의 선택지 UI를 감지하면 `input_mode=arrows`와 `selected_key`를 함께 업로드할 수 있다.
1. 워커는 대시보드에서 생성된 태스크 입력을 claim해 동일 tmux target에 주입하고 `task.input.injected/failed` 이벤트로 결과를 남길 수 있다.
1. 대시보드에서 옵션을 클릭하면 워커는 필요 시 숫자 입력 대신 `Up/Down/Enter` 키 시퀀스를 주입해 선택을 진행할 수 있다.

## C) Legacy: Claude-Code-Remote (Hooks / Notifications / Inbound Commands)

### C1. 설치/설정

1. 사용자가 `Claude-Code-Remote/setup.js`를 실행하면 `.env`를 생성/갱신하고 Claude Code 훅 설정을 자동으로 구성한다.
1. 설치 스크립트는 `~/.claude/settings.json`에 Stop/SubagentStop 훅을 merge하여 작업 완료/대기 이벤트를 로컬 커맨드로 트리거한다.
1. 설치 환경에 `agent/clw-agent.js`가 있으면 훅 커맨드는 레거시 스크립트 대신 agent 래퍼를 우선 사용해 중복 알림을 방지한다.
1. 사용자가 `Claude-Code-Remote/setup-telegram.sh`를 실행하면 Telegram 설정과 함께 Claude hooks 설정 파일을 생성한다.
1. 사용자가 `CLAUDE_HOOKS_CONFIG`를 지정하면 전역 설정 대신 프로젝트 단위 훅 설정을 적용할 수 있다.

### C2. 실행/운영

1. 사용자가 `Claude-Code-Remote/start-all-webhooks.js`를 실행하면 Telegram/LINE 웹훅 서버와(옵션) 이메일 데몬을 함께 기동한다.
1. 사용자가 `Claude-Code-Remote/start-telegram-webhook.js`를 실행하면 Telegram 인바운드 명령을 받는 Express 서버가 기동한다.
1. 운영자가 Telegram 웹훅 서버의 `/health`를 호출하면 서버가 살아있는지 확인할 수 있다.
1. 사용자가 `Claude-Code-Remote/start-line-webhook.js`를 실행하면 LINE 인바운드 명령을 받는 Express 서버가 기동한다.
1. 운영자가 LINE 웹훅 서버의 `/health`를 호출하면 서버가 살아있는지 확인할 수 있다.
1. 사용자가 `Claude-Code-Remote/start-relay-pty.js`를 실행하면 이메일 명령 relay(PTY 기반) 프로세스를 기동한다.
1. 사용자가 `node Claude-Code-Remote/claude-remote.js daemon start`를 실행하면 백그라운드 이메일 데몬이 기동한다.

### C3. 작업 완료/대기 감지 → 알림(Outbound)

1. Claude Code가 작업을 완료하거나 입력 대기 상태가 되면 Stop/SubagentStop 훅이 실행되어 알림 전송이 트리거된다.
1. 레거시 훅 스크립트(`claude-hook-notify.js`)는 활성화된 채널(Email/Telegram/LINE/Desktop)에 동시에 알림을 보낼 수 있다.
1. Desktop 채널은 작업 완료/대기 시 OS 알림과 사운드로 즉시 알려준다.
1. Telegram 채널은 작업 완료/대기 시 봇 메시지로 알리고 개인/그룹 채팅에 맞는 명령 포맷을 제공한다.
1. LINE 채널은 작업 완료/대기 시 메시지로 알리고 토큰 기반 명령 포맷을 제공한다.
1. Email 채널은 작업 완료/대기 시 이메일을 보내고(옵션) 전체 실행 트레이스를 포함할 수 있다.
1. 알림에는 원격 명령 라우팅을 위한 토큰이 포함되어 사용자가 “답장/명령”을 보낼 수 있다.
1. 토큰은 24시간 등 만료 시간이 있어 오래된 알림 토큰으로는 명령 실행이 차단된다.
1. 알림 시 tmux가 감지되면 tmux-monitor가 최근 대화/응답을 캡처해 메시지에 포함하려고 시도한다.
1. Email 알림은 tmux-monitor를 통해 실행 트레이스(터미널 출력)를 캡처해 포함할 수 있다.
1. 설정에 따라 SubagentStop(서브에이전트) 알림을 켜거나 끌 수 있다.
1. SubagentStop 알림이 비활성화되어도 시스템은 subagent 활동을 로컬 tracker에 기록해 “완료 알림에 합쳐 보내기”를 준비할 수 있다.
1. 완료 알림(특히 Email)에서는 설정에 따라 tracker에 쌓인 subagent 활동 요약을 포함한 뒤 해당 기록을 정리할 수 있다.

### C4. Telegram 인바운드(명령 수신 → 주입)

1. Telegram이 웹훅으로 update 이벤트를 보내면 Telegram webhook 서버가 이를 수신해 메시지/버튼 이벤트를 처리한다.
1. Telegram 인바운드 명령은 whitelist(유저/채팅 ID) 또는 지정된 chat/group ID로 권한을 검증한다.
1. 사용자가 Telegram에서 `/start`를 보내면 봇이 사용 목적과 기본 사용법을 안내한다.
1. 사용자가 Telegram에서 `/help`를 보내면 토큰 기반 명령 포맷과 예시를 안내한다.
1. 사용자가 Telegram 알림의 버튼을 누르면 callback query로 개인/그룹 채팅용 명령 포맷을 자동 안내한다.
1. 사용자가 `/cmd <TOKEN> <command>` 또는 `<TOKEN> <command>` 형식으로 보내면 서버가 토큰과 명령을 파싱한다.
1. 서버는 토큰으로 세션 파일을 조회해 만료 여부를 확인한 뒤 주입 대상(tmuxTarget/tmuxSession/PTY)을 결정한다.
1. 서버는 ControllerInjector로 명령을 주입해 로컬 Claude 세션에서 실행되게 한다.
1. 서버는 명령 주입 성공/실패를 Telegram 메시지로 사용자에게 회신한다.

### C5. LINE 인바운드(명령 수신 → 주입)

1. LINE이 웹훅으로 callback 이벤트를 보내면 LINE webhook 서버가 이를 수신해 메시지 이벤트를 처리한다.
1. LINE 인바운드 요청은 `x-line-signature` 서명 검증으로 위조 요청을 차단한다.
1. 사용자가 토큰 기반 명령 포맷으로 보내면 서버가 토큰과 명령을 파싱한다.
1. 서버는 토큰으로 세션 파일을 조회해 만료 여부를 확인한 뒤 주입 대상(tmuxTarget/tmuxSession/PTY)을 결정한다.
1. 서버는 ControllerInjector로 명령을 주입해 로컬 Claude 세션에서 실행되게 한다.
1. 서버는 명령 주입 성공/실패를 LINE 메시지로 사용자에게 회신한다.

### C6. Email 인바운드(답장 → 명령 수신 → 주입)

1. 이메일 데몬은 IMAP 폴링으로 신규 메일/답장을 감지해 명령을 추출한다.
1. Email 인바운드 명령은 `ALLOWED_SENDERS` whitelist로 발신자를 검증한다.
1. Email 인바운드 명령은 제목의 토큰(예: `[Claude-Code-Remote #TOKEN]`)으로 세션을 식별한다.
1. Email 인바운드 명령은 세션 매핑(session-map.json 등)으로 주입 대상(tmuxTarget/tmuxSession/PTY)을 결정한다.
1. Email 인바운드 명령은 tmux-injector로 무인 주입을 시도하고 필요 시 tmux 세션을 생성할 수 있다.
1. Email 인바운드 경로는 tmux 주입 실패 시 smart-injector로 OS 자동화(클립보드/AppleScript 등) 폴백을 시도할 수 있다.
1. Email 인바운드 경로는 중복 처리 방지를 위해 처리된 메시지를 로컬 기록에 저장한다.
1. Email 인바운드 경로는 시스템이 보낸 메일을 `sent-messages.json` 등으로 추적해 self-reply loop를 방지한다.
1. Email 인바운드 경로는 세션별 `maxCommands/commandCount`로 과도한 원격 명령 실행을 제한할 수 있다.
1. Email 인바운드 경로는 `rm -rf`, `sudo`, `curl | sh` 등 위험 커맨드를 간단 블랙리스트로 차단할 수 있다.
1. relay-pty 경로는 메일 본문을 정리해 인용/서명/인사말을 제외하고 멀티라인 명령을 추출해 주입할 수 있다.
1. relay-pty 경로는 같은 메시지의 재처리를 막기 위해 `processed-messages.json` 기반 dedupe를 수행한다.
1. relay-pty 경로는 같은 명령이 반복되는 메일을 감지해 중복 텍스트를 줄여(deduplicate) 주입할 수 있다.

### C7. 주입(Injection) 모드/정책

1. ControllerInjector는 설정에 따라 tmux 모드 또는 PTY 모드로 명령을 주입할 수 있다.
1. tmux 모드 주입은 기존 tmux 세션에 send-keys로 입력하여 동일 대화 세션을 유지한다.
1. tmuxTarget이 있으면 tmux 모드 주입은 pane 단위 target으로 주입해 동일 tmux 세션 안의 여러 Claude 세션도 정확히 구분한다.
1. PTY 모드 주입은 세션 맵 파일에 기록된 PTY 경로에 명령을 기록해 실행되게 한다.
1. tmux-injector(무인 주입)는 Claude permission dialog 등을 자동 처리하며 “완전 자동 실행”을 목표로 한다.
1. tmux-injector(특히 relay-pty 경로)는 대상 세션이 없을 때 새 tmux 세션을 생성할 수 있어 새 대화 세션이 열릴 수 있다.
1. smart-monitor는 tmux 화면을 모니터링해 Claude 응답 상태를 감지하고 알림/이력에 활용될 수 있다.

### C8. 레거시 CLI/진단/테스트

1. 사용자가 `claude-remote status`를 실행하면 시스템 설정/채널/서비스 상태를 요약해서 볼 수 있다.
1. 사용자가 `claude-remote notify --type completed|waiting`을 실행하면 훅 없이도 수동으로 알림을 보낼 수 있다.
1. 사용자가 `claude-remote test`를 실행하면 모든 알림 채널을 테스트할 수 있다.
1. 사용자가 `claude-remote config`를 실행하면 인터랙티브 설정 관리자를 통해 채널/키를 구성할 수 있다.
1. 사용자가 `claude-remote setup-email`을 실행하면 이메일 설정을 위한 Quick Setup Wizard를 실행할 수 있다.
1. 사용자가 `claude-remote edit-config <type>`을 실행하면 구성 파일을 직접 편집할 수 있다.
1. 사용자가 `claude-remote install`을 실행하면 Claude hooks 설치/구성을 진행할 수 있다.
1. 사용자가 `claude-remote relay start|stop|status|cleanup`을 실행하면 이메일 명령 relay 서비스를 관리할 수 있다.
1. 사용자가 `claude-remote daemon start|stop|restart|status`를 실행하면 백그라운드 데몬을 관리할 수 있다.
1. 사용자가 `claude-remote commands list|status|cleanup|clear`를 실행하면 이메일 command bridge 큐를 조회/정리할 수 있다.
1. 사용자가 `claude-remote test-paste`를 실행하면 자동 붙여넣기 기능을 테스트할 수 있다.
1. 사용자가 `claude-remote test-simple`를 실행하면 간단 자동화 주입 경로를 테스트할 수 있다.
1. 사용자가 `claude-remote test-claude`를 실행하면 Claude Code 전용 자동화(권한/주입 포함)를 통합 테스트할 수 있다.
1. 사용자가 `claude-remote setup-permissions`를 실행하면 macOS 자동화 권한 설정 안내/점검을 수행할 수 있다.
1. 사용자가 `claude-remote diagnose`를 실행하면 자동화 이슈 진단을 수행할 수 있다.
1. 사용자가 `fix-telegram.sh`를 실행하면 Telegram 연결/웹훅 관련 문제를 점검/수정할 수 있다.
1. 개발자는 `test-complete-flow.sh`로 주요 플로우를 스모크 테스트할 수 있다.
1. 개발자는 `test-telegram-notification.js`로 Telegram 알림 발송을 테스트할 수 있다.
1. 개발자는 `test-telegram-setup.sh`로 Telegram 설정을 점검할 수 있다.
1. 개발자는 `test-injection.js`로 주입 경로를 테스트할 수 있다.
1. 개발자는 `test-long-email.js`로 이메일(긴 내용/트레이스 포함) 알림 경로를 테스트할 수 있다.
1. 개발자는 `test-real-notification.js`로 실제 채널로의 알림 발송을 테스트할 수 있다.

## D) Legacy ↔ Coordinator Bridge (Optional)

1. 레거시 프로세스가 `COORDINATOR_URL`이 설정된 경우 인바운드 명령 성공/실패를 Coordinator events로 업로드할 수 있다.
1. Telegram/LINE/Email 인바운드 명령 처리 결과는 Coordinator에 `telegram.command.sent/failed` 등 이벤트 타입으로 기록될 수 있다.
1. 레거시 업로더는 agent ID 파일(`agent/data/agent-id.txt`)이 있으면 이를 재사용해 같은 agent_id로 관측되게 한다.

## E) Repo-level Smoke/Regression Guardrails

1. 개발자는 `tasks/smoke/legacy-static-check.sh`로 레거시/브릿지 스크립트의 파일 존재 및 JS 문법을 빠르게 검증할 수 있다.
