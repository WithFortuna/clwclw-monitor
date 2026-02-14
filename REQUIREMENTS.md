# Claude Code Remote Webview 확장 요구사항

## 1. 목적
- 기존 Claude-Code-Remote 기능을 참고해, 원격 제어/알림 기반 시스템에 **웹뷰 UI**와 **작업 태스크 보드**를 추가한다.
- 최종 목표는 인터넷에서 접속 가능한 대시보드를 통해 **에이전트 상태/작업 이력/태스크 분배**를 최소 기능으로 확인·운영하는 것이다.
- 클론한 `Claude-Code-Remote` 레포의 기능은 **전부 포함**되어야 한다. (회귀 금지)

## 2. 범위 (MVP 기준)
- Coordinator(API 서버)는 **Go**로 구현한다.
- 저장소는 **Supabase(Postgres)** 를 사용한다. (로컬 파일은 중앙 저장소로 사용하지 않음)
- 로그인/인증은 **후순위**로 미룬다. (초기에는 임시 키/토큰 방식 가능)
- 에이전트 수: 최대 10개
- 동시 접속 사용자: 최대 1,000명
- 로그/이벤트 보관: **30일**

## 3. 용어/구성요소
- **Coordinator**: 인터넷에 배포되는 Go 기반 API 서버. 에이전트 상태/이력/태스크를 중앙에서 관리한다.
- **Agent**: 로컬에서 Claude Code 세션을 관찰/제어하는 실행 프로세스. (tmux/pty 주입, 로그 수집, 채널 알림 등)
- **Channel(태스크 채널)**: 태스크를 발행/구독하는 논리적 작업 영역(예: `backend-domain`, `notify`).
- **Task**: 채널에 발행되는 작업 단위. 에이전트가 FIFO로 가져가 수행한다.
- **Event(작업 이력)**: 에이전트가 “지금까지 한 일”을 나타내는 레코드. (task_id는 NULL 가능)

## 4. 핵심 기능 요구사항

### 4.1 기존 기능 유지 (필수)
클론한 `Claude-Code-Remote` 레포에 포함된 기능은 모두 유지되어야 한다.
- 이메일 알림 + IMAP 리스너/응답
- 텔레그램 봇 + 버튼/커맨드
- LINE 메시징
- 데스크톱 알림/사운드
- tmux/pty 주입
- 실행 로그 캡처(터미널 트레이스)
- 세션 토큰/화이트리스트 보안
- CLI/스크립트 기반 테스트 & 진단

### 4.2 웹뷰 대시보드 (배포형)
- 인터넷에서 접근 가능한 웹 UI 제공
- 현재 활성 에이전트 수 표시
- 에이전트별 상태 및 현재 작업 표시
- 에이전트별 작업 이력(타임라인) 표시

### 4.3 작업 태스크 보드 (Jira 유사, 최소 기능)
- 채널(도메인) 단위로 태스크 발행
- 채널별 태스크 목록/상태 표시
- 태스크 상태 최소 단계: `Queued / In Progress / Done`

### 4.4 태스크 분배 모델
- 분배 방식: **FIFO**
- 에이전트는 채널을 구독하고, FIFO 순서대로 태스크를 가져감
- 수동 할당 가능 (자동 분배는 보조)
- FIFO claim은 **원자적(atomic)** 으로 수행되어 중복 할당이 발생하지 않아야 함
- **Chain 기반 태스크 관리**: 기존 Task 단위로 관리되던 큐잉 시스템을 Chain 단위로 변경한다. 이는 여러 Task가 논리적으로 연결된 경우 이를 하나의 작업 흐름(Chain)으로 간주하여 관리하고 처리하는 것을 의미한다. Chain 내의 Task들은 순차적으로 처리될 수 있으며, Chain 전체의 상태를 추적할 수 있다.

#### 4.4.1 Chain Ownership 및 독점성 (Exclusivity)
- **Chain-level ownership**: Agent는 동시에 하나의 Chain에 대해서만 소유권을 가질 수 있다
- **Ownership 획득**: Agent가 Chain의 첫 번째 Task를 claim하면 해당 Chain의 소유권을 자동으로 획득한다
- **Ownership 유지**: Agent가 Chain을 소유하는 동안 다른 Chain의 Task를 claim할 수 없다
- **독점적 접근**: Chain을 소유한 Agent만 해당 Chain의 Task를 claim할 수 있다. 같은 채널을 구독하는 다른 Agent는 소유된 Chain의 Task를 claim할 수 없다
- **Ownership 해제**: Chain의 모든 Task가 완료되거나 실패하면 소유권이 자동으로 해제된다
- **Detach 메커니즘**: Agent는 명시적으로 Chain ownership을 해제할 수 있다 (수동 detach)

#### 4.4.2 Detach 및 Locked Task 처리
- **Detach 동작**: Agent를 Chain에서 분리하면 현재 `in_progress` 상태인 Task는 `locked` 상태로 전환된다
  - Chain의 `owner_agent_id`가 초기화된다
  - Chain 상태가 `locked`로 변경된다
  - Agent의 `current_task_id`가 초기화된다
- **Locked 상태**: Task/Chain이 Agent 분리로 인해 중단된 상태를 나타내는 새로운 상태값
  - Task 상태: `queued` / `in_progress` / `done` / `failed` / **`locked`**
  - Chain 상태: `queued` / `in_progress` / `done` / `failed` / **`locked`**
- **Locked Task 해결**: UI에서 locked 상태의 Task를 수동으로 해결할 수 있다
  - `locked → queued`: Task를 다시 대기열로 되돌린다 (`assigned_agent_id`, `claimed_at` 초기화)
  - `locked → done`: Task를 완료 처리한다 (`done_at` 설정)
  - 해결 후 Chain 상태는 남은 Task들의 상태에 따라 자동 재평가된다
- **Claim 제한**: Chain에 `locked` 상태 Task가 존재하면 해당 Chain의 Task를 claim할 수 없다
- **UI 요소**:
  - Chain 카드 헤더에 소유 Agent 이름 표시 및 Detach 버튼 제공
  - Locked Task는 In Progress 컬럼에 시각적으로 구분되어 표시 (dashed border, amber 색상)
  - Locked Task에 "→ Queued" / "→ Done" 버튼으로 상태 전환
  - Agent 미할당 또는 locked 상태의 Chain에 Agent 할당 드롭다운 제공
- **체인 수동 할당 구독 검증**:
  - `POST /v1/chains/{id}/assign-agent` 호출 시 대상 Agent가 해당 Chain의 채널을 `subscriptions`에 포함하고 있는지 서버에서 검증해야 한다.
  - 미구독 상태면 서버는 명시적 오류 코드를 반환해야 한다(프론트 분기 가능).
  - 대시보드는 이 오류를 받으면 사용자에게 채널 구독 추가 여부를 확인하고, 승인 시 `PATCH /v1/agents/{id}/channels`로 채널을 추가한 뒤 할당을 재시도해야 한다.
- **API 엔드포인트**:
  - `POST /v1/chains/{id}/detach` — Agent를 Chain에서 분리
  - `POST /v1/tasks/{id}/status` — Locked Task 상태 전환 (locked→queued 또는 locked→done만 허용)
  - `POST /v1/chains/{id}/assign-agent` — Chain에 Agent 할당

### 4.5 작업 이력 정의
- 태스크를 수행한 경우: 태스크 수행 로그를 작업 이력으로 기록
- 태스크 미할당 상태에서 실행된 명령: **사용자 명령 자체가 작업 이력**

### 4.6 웹뷰/에이전트 통신
- 로컬 에이전트는 실행 로그/상태를 Coordinator API로 전송
- 웹뷰는 실시간 상태 변화를 수신해 반영 (SSE/WS 또는 Supabase Realtime)
- 이벤트 중복 업로드를 방지하기 위한 최소한의 **idempotency** 가 필요 (구현 상세는 설계에서 결정)

### 4.7 태스크별 실행 모드 지정
- 태스크마다 **Claude Code 실행 모드(execution mode)** 를 지정할 수 있다
- 실행 모드 종류:
  - `accept-edits`: 편집 자동 승인 모드
  - `plan-mode`: 계획 수립 모드 (코드 실행 없이 탐색/계획만)
  - `bypass-permission`: 모든 권한 자동 승인 (optional, 사용자 설정에 따라 사용 가능 여부 다름)
- 에이전트가 태스크를 claim하면 **현재 모드를 감지**하고, 필요 시 **목표 모드로 자동 전환**
- 모드 감지: `tmux capture-pane`으로 화면 캡처 후 현재 모드 표시 텍스트 파싱
- 모드 전환: `Shift+Tab` 키를 이용한 순환 방식으로 목표 모드까지 이동
- 모드 전환 실패 시 에이전트는 경고 이벤트를 생성하고 태스크를 fail 처리

### 4.8 원격 세션 시작 요청
- Coordinator는 특정 채널의 "헤드리스" 에이전트(특정 tmux 세션에 연결되지 않은 에이전트)에게 새로운 Claude Code 세션을 시작하도록 요청할 수 있다.
- **흐름:**
  1. Coordinator가 `request_claude_session` 타입의 특수 태스크를 생성한다.
  2. 헤드리스 상태의 에이전트가 이 태스크를 claim한다.
  3. 에이전트는 사용자에게 CLI 프롬프트를 통해 세션 시작 방법을 묻는다.
     - **자동 모드:** 에이전트가 백그라운드에서 `tmux` 세션을 직접 생성하고 `claude` 명령어를 실행한다. 사용자는 `tmux attach` 명령어로 세션에 접속할 수 있다.
     - **수동 모드:** 사용자가 직접 `tmux` 세션을 시작하고, 해당 세션 정보를 에이전트에게 입력하여 연결한다.
  4. 세션이 성공적으로 설정되면 에이전트는 일반 작업 모드로 전환되고, `request_claude_session` 태스크는 자동으로 완료 처리된다.
  5. `request_claude_session` 완료 처리는 일반 `task.completed` 이벤트와 분리된 전용 이벤트(`agent.automation.session_request.completed`)로 수행한다.
     - Coordinator는 세션 요청 태스크 생성 시 `agent_session_request_token`을 함께 발급한다.
     - Agent는 세션 할당/자동 실행이 완료되면 전용 이벤트 payload에 canonical 키 `agent_session_request_token`만 포함해 전송한다.
     - Coordinator는 `agent_session_request_token`(및 선택적으로 `task_id`)으로 대상 세션 요청 태스크를 식별하여 `done`으로 전이한다.
     - 전용 완료 이벤트 처리 시 task 상태 전이가 실패하면 성공으로 무시하지 않고 명시적으로 에러를 반환한다.

### 4.9 에이전트 인터랙티브 CLI 플로우
- `login` 완료 후 자동으로 work 설정 진입 여부를 프롬프트로 물어봄
- `work` 명령을 `--channel` 없이 실행하면 인터랙티브 프롬프트 진입
- 인터랙티브 프롬프트에서 채널 목록을 API로 조회하여 번호로 선택 (복수 선택 가능)
- tmux 모드는 기존 자동/수동 선택 재활용
- 기존 `--channel`, `--tmux-target` 플래그 기반 동작은 하위 호환 유지

### 4.10 UI 기반 에이전트 채널 할당 (추후 구현)
- Agent 모델에 `subscriptions []string` 필드 추가
- `PATCH /v1/agents/:id/channels` API 추가
- 대시보드 UI에서 에이전트별 채널 할당/해제 기능
- agent work 루프에서 서버 채널 구독 정보를 polling하여 동적 채널 변경
- 구독 채널 인라인 편집 UX는 `Enter`/입력창 외부 클릭(blur) 시 저장 요청, `Esc` 시 취소되어야 하며 저장/취소 후 포커스가 해제되어야 한다.
- 구독 채널 인라인 편집 시 저장 요청 상태(예: saving/saved/error)를 UI에서 확인할 수 있어야 한다.

### 4.11 Agent IPC (Unix Domain Socket)
- 유저당 1개의 `agentd` 데몬이 UDS(Unix Domain Socket)를 listen
- worker(work 커맨드)는 시작 시 agentd에 register (paneId, agentId, mode, coordinatorUrl)
- hook 프로세스는 agentd에 `hook_request`를 전송, agentd가 paneId 기준으로 올바른 worker에게 라우팅
- worker가 coordinator API 호출 후 `hook_ack`를 agentd에 전송, agentd가 hook에게 결과 전달
- agentd 미실행 시 hook은 기존 파일 기반 fallback (hookDirectCoordinator) 사용
- worker는 시작 시 agentd가 없으면 detached child로 자동 시작
- 프로토콜: NDJSON (newline-delimited JSON), 모든 메시지에 `type`과 `id` 포함
- 소켓 경로: `$XDG_RUNTIME_DIR/clwclw/agentd.sock` 또는 `~/.clwclw/run/agentd.sock`
- 싱글턴 보장: `agentd.pid` lock file로 중복 실행 방지

## 5. 비기능 요구사항
- **경량 인프라** 지향 (최소 비용)
- 실시간성: 웹뷰에서 상태 변화가 빠르게 반영될 것
- 안정성: 에이전트/서버 재시작 시 상태 복구 가능
- 확장성: 동접 1,000명 UI 조회를 감당할 수 있어야 함 (캐시/실시간 방식은 구현에서 결정)

## 6. 데이터 모델 (초안)

### 6.1 Agents
- `id`
- `name`
- `status` (idle/running/waiting 등)
- `current_task_id`
- `last_seen`

### 6.2 Channels
- `id`
- `name`
- `description`

### 6.3 Tasks
- `id`
- `channel_id`
- `title`
- `description`
- `status`
- `priority`
- `assigned_agent_id`
- `execution_mode` (NULL 가능: `accept-edits`, `plan-mode`, `bypass-permission`)
- `created_at`
- `claimed_at`

### 6.4 Events (작업 이력)
- `id`
- `agent_id`
- `task_id` (NULL 가능)
- `type`
- `payload`
- `created_at`

## 7. 처리 흐름 (요약)
1) 에이전트는 상태 및 로그를 Coordinator API에 전송
2) Coordinator는 Supabase에 저장
3) 웹 UI는 실시간으로 상태/태스크/이력을 표시
4) 태스크는 채널 구독 에이전트가 FIFO로 claim

## 8. 개발 계획 (권장 순서)
요구사항 문서의 나열 순서와 실제 개발 순서는 다를 수 있다. 구현 리스크/의존성 기준으로 아래 순서를 권장한다.
1) 기존 레포 기능 스모크 테스트/회귀 테스트 기준 확보 (기능 유지 보장용)
2) Supabase 스키마/인덱스 확정 + 30일 보관 정책(정리 작업) 결정
3) Coordinator(Go) 최소 API 구현: agent heartbeat, events ingest, channels, tasks CRUD, FIFO claim
4) 로컬 에이전트에서 Coordinator로 상태/이력 업로드 연결 (기존 알림/주입 기능 유지)
5) 웹뷰 최소 UI 구현: 에이전트 리스트/상태, 채널별 태스크, 에이전트 작업 이력
6) 하드닝: 임시 인증(공유 키), rate limit, 관측성(logging/metrics), 장애/재시도 정책

## 9. 보류/추후 결정 사항
- 로그인/인증 체계(정식)
- 구체적 UI 프레임워크 및 배포 스택
- ~상세 태스크 분배 정책(재시도, 실패 처리, 우선순위 규칙)~ => 체인 및 lease 기반 소유권 관리 방식으로 결정 
- 실행 로그(터미널 트레이스) 저장 방식: DB 직접 저장 vs 파일/오브젝트 스토리지(요금/용량 고려)
