# Tasks Index

> Source of truth: `REQUIREMENTS.md`

- `0001-bootstrap-coordinator.md` — **Done** — Go Coordinator 프로젝트 골격 + in-memory API
- `0002-supabase-schema.md` — **Done** — Supabase(Postgres) 스키마/인덱스/보관정책/idempotency 초안
- `0003-agent-bridge.md` — **Done** — 기존 `Claude-Code-Remote`를 “로컬 Agent”로 래핑하여 Coordinator로 이벤트/상태 업로드
- `0004-preserve-legacy-features.md` — **Done** — 기존 레포 기능 회귀 방지(채널/주입/보안/테스트) 체크리스트 + 스모크 테스트 루틴
- `0005-web-dashboard.md` — **Done** — 웹 대시보드(에이전트 상태/이력/태스크 보드) MVP
- `0006-migration-plan.md` — **Done** — 레거시(Node) → Go “전체 마이그레이션” 단계별 전략/원칙(회귀 방지 포함)
- `0007-task-lifecycle.md` — **Done** — Task 상태 전이(complete/fail) API + UI 반영
- `0008-agent-worker.md` — **Done** — Agent task worker(채널 claim → 주입 → 완료 훅 연동)
- `0009-multi-session-agents.md` — **Done** — 로컬 다중 Claude 세션(예: 5개) 관측/분리(Agent ID/state per tmux target)
- `0010-manual-task-assign.md` — **Done** — 특정 queued task를 특정 agent에 수동 할당(assign)
- `0011-usecases-catalog.md` — **Done** — 현재 제공 기능을 유스케이스(1문장)로 목록화(레거시 포함)
- `0012-multi-session-routing.md` — **Done** — 토큰/명령을 tmux pane(target) 단위로 정확히 라우팅(5개 세션 대응)
- `0013-hardening-basics.md` — **Todo** — 보안/신뢰성 하드닝(ratelimit, timeouts, webhook 검증 옵션 등)
- `0014-task-orchestration-enhancements.md` — **Todo** — 재큐잉/타임아웃/오프라인 재할당/우선순위 등 분배 고도화
- `0015-trace-centralization.md` — **Todo** — 트레이스/아티팩트 중앙 저장 + 대시보드 링크
- `0016-legacy-quality.md` — **Todo** — 레거시 품질/정합성 정리(기능 변경 없이)
- `0017-auth-rbac.md` — **Todo** — 정식 인증/권한(RBAC) 체계 도입
- `0018-observability.md` — **Todo** — 로그/메트릭/트레이싱 등 관측성 추가
- `0019-deployment-packaging.md` — **Todo** — 운영/배포 패키징(Docker/systemd 등, 최후순위)
- `0020-acceptance-test-runbook.md` — **Done** — 로컬 실행/화면 확인/인수테스트 절차 문서화
- `0021-interactive-prompts.md` — **Done** — 인터랙티브 선택지 감지 → 대시보드 표시 → 사용자 선택/주입
- `0022-interactive-prompt-navigation.md` — **Done** — Enter/Tab/Arrow/Esc 기반 선택지 UI 감지 + 키 주입(옵션 선택)
- `0049-request-session-token-completion.md` — **Done** — request_claude_session 완료를 전용 이벤트 + agent_session_request_token 기반으로 분리 처리
- `0050-session-request-completion-hardening.md` — **Done** — 전용 완료 이벤트 payload 정규화 + task 상태 전이 실패 명시 에러 처리
- `0054-enforce-chain-assign-subscription-check.md` — **Done** — 체인 수동 할당 시 채널 구독 검증 + UI 구독 추가 확인/재시도
- `0055-fix-subscription-dropdown-edit-refresh-race.md` — **Done** — Agent subscription 드롭다운 편집 중 auto refresh 리렌더 경합 수정
- `0061-fix-queued-popover-refresh-focus-race.md` — **Done** — 체인/Queued popover 입력 중 refresh 리렌더로 인한 포커스/드래프트 유실 방지
- `0062-fix-agent-list-order-jitter.md` — **Done** — 에이전트 목록 응답을 이름 오름차순으로 고정해 대시보드 리렌더링 순서 흔들림 제거
- `0063-offline-worker-forces-claude-not-running.md` — **Done** — Worker가 offline일 때 Claude 상태를 not running으로 강제
