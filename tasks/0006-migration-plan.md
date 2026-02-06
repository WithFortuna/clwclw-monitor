# 0006 — Migration Plan (Legacy Node → Go, Strangler Pattern)

Status: **Done**

## Goal

현재는 **레거시(`Claude-Code-Remote`, Node.js)를 “그대로 사용”**하면서 Go Coordinator/Supabase로 확장하고 있다.  
이 문서는 나중에 **Go로 전체 마이그레이션**할 것을 전제로, 지금의 구조/경계/결정사항과 단계별 이행 계획을 고정한다.

## Current Architecture (Today)

### What is migrated to Go now?

- Coordinator(API 서버): `coordinator/`
- 중앙 DB 스키마 초안(Supabase/Postgres): `supabase/migrations/0001_init.sql`
- Coordinator MVP UI(SSE + 폴링 fallback): `coordinator/internal/httpapi/ui/*`
  - 단일 스냅샷 API: `GET /v1/dashboard` (서버 TTL cache)
  - 실시간 트리거: `GET /v1/stream` (SSE)

### What remains in legacy Node now? (No regression)

기존 레포의 기능은 그대로 유지한다.

- Claude Code hooks 기반 “완료/대기” 감지 및 채널 알림: `Claude-Code-Remote/claude-hook-notify.js`
- Telegram/LINE webhook 인바운드(명령 수신) 및 인증(whitelist/서명): `Claude-Code-Remote/src/channels/*/webhook.js`
- Email(발송/IMAP 리스너/답장 명령 추출), tmux/자동화 주입, 실행 트레이스 캡처 등: `Claude-Code-Remote/src/**`

### Bridge layer (glue)

레거시 동작을 깨지 않으면서 중앙 관측을 붙이기 위한 “브릿지”를 도입했다.

- 훅 래퍼(권장): `agent/clw-agent.js hook completed|waiting`
  - 1) 레거시 알림 실행(기존 behavior 유지)
  - 2) Coordinator로 heartbeat/event 업로드(best-effort)
- 레거시 내부 업로더(옵션): `Claude-Code-Remote/src/core/coordinator-client.js`
  - 인바운드 명령 성공/실패 이벤트를 Coordinator에 업로드(설정 시만 동작)

즉, **지금은 “Go로 통째로 포팅”이 아니라 “레거시 활용 + 점진적 치환(strangler)”** 방식이다.

## Why this approach?

- 최우선 요구사항이 “기존 레포 기능 전부 포함(회귀 금지)”이기 때문
- Go로 한 번에 포팅하면 채널/자동화/플랫폼 권한/엣지케이스에서 회귀가 발생하기 쉬움
- 중앙 관측/태스크 보드(Coordinator)는 레거시와 독립적으로 먼저 만들 수 있음

## Migration Principles (to keep in mind)

- **Contract-first**: Coordinator API/이벤트 스키마를 먼저 고정하고 구현체(레거시/Go)를 교체한다.
- **Feature parity gates**: “레거시 vs Go”를 동일 입력에 대해 비교 가능한 형태로 만든 뒤 스위칭한다.
- **Strangler boundaries**: 기능 단위로 “구성요소 교체”가 가능하도록 경계를 유지한다.
- **No silent regressions**: 최소 스모크 테스트/시나리오를 `tasks/0004-preserve-legacy-features.md`에 고정한다.
- **Operational safety**: 실패 시 항상 레거시 경로로 폴백 가능하게 유지한다.

## Target End State (All-Go)

### Coordinator (Go)

- Agents/Tasks/Events 저장/분배 + 실시간 업데이트(SSE/WS or Supabase Realtime)

### Agent (Go)

- Claude hooks 처리, 로그 수집, 상태 업로드
- Telegram/LINE/Email 인바운드 수신 + 명령 검증 + 주입
- 주입(tmux/pty/OS automation)은 OS별 분기(권한/호환성) 포함

### Deprecation

- `Claude-Code-Remote`는 완전히 제거하거나, “플러그인/호환 모드”로만 유지

## Suggested Phases

### Phase 0 — Observability-first (현재)

- 레거시를 운영하며 Coordinator에 event/heartbeat만 업로드
- 대시보드/태스크 보드 기반 운영 확립

Exit criteria:
- 레거시 사용 중에도 모든 핵심 이벤트가 Coordinator에 기록됨

### Phase 1 — Inbound command path migration (Go)

- Telegram/LINE webhook 수신 및 인증을 Go로 구현(레거시와 병행 가능)
- 토큰/세션 매핑을 DB 중심으로 옮김(파일 기반 제거)

Exit criteria:
- “명령 수신→검증→주입”이 Go 경로로도 안정적으로 동작(레거시 대비 기능 동등)

### Phase 2 — Notification channels migration (Go)

- Telegram/LINE/Email/desktop 채널 발송을 Go로 구현(레거시 대비 메시지/템플릿 동등)

Exit criteria:
- 훅 트리거 시 알림이 Go 경로로도 동일하게 전달됨(템플릿/추적 포함)

### Phase 3 — Injection/automation migration (Go)

- tmux 주입, fallback 자동화, 권한 안내 등을 Go로 이식
- 가장 회귀 가능성이 높은 영역이므로 마지막에 수행(그리고 OS별로 나눠 진행)

Exit criteria:
- “세션 유지/새 세션 생성” 정책이 명확하고, 실패시 복구/폴백이 정의됨

### Phase 4 — Remove legacy

- 레거시 프로세스/파일 기반 세션 저장 제거
- 문서/테스트/운영 런북에서 레거시 제거

## Open Questions (to decide early)

- (1) 토큰/세션 매핑의 “정답 원천(source of truth)”을 DB로 언제 완전히 옮길지
- (2) 멀티 세션(5개 Claude) 라우팅 기준: tmux 세션명 vs PTY 핸들 vs 별도 세션 식별자
- (3) 보안 모델: 임시 토큰/whitelist에서 정식 인증으로 확장 경로
- (4) 실행 로그 저장: DB 직접 저장 vs object storage(용량/요금)

## Notes / References

- 현재 구현 요약: `tasks/WORKLOG.md`
- 레거시 동작/유스케이스: `CLAUDE_CODE_REMOTE_USECASES.md`
- 회귀 방지 체크리스트(작성 예정): `tasks/0004-preserve-legacy-features.md`
