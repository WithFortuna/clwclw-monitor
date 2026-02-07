# 0002 — Supabase Schema (Agents / Channels / Tasks / Events)

Status: **In Progress**

## Goal

중앙 저장소를 Supabase(Postgres)로 확정하기 위해 테이블/인덱스/보관 정책(30일)을 정의한다.

## Acceptance Criteria

- [x] `agents`, `channels`, `tasks`, `events` 테이블 정의
- [x] 필수 인덱스 정의(조회/claim 성능)
- [x] FIFO claim을 위한 원자적 쿼리(예: `FOR UPDATE SKIP LOCKED`) 설계 문서화
- [x] 30일 보관 정책(이벤트/로그) 운영 방식 결정(초기: Coordinator가 주기적으로 purge)
- [x] idempotency 키 전략 확정(중복 업로드 방지)

## Idempotency Strategy (MVP)

### Events (`POST /v1/events`)

- 클라이언트는 `idempotency_key`를 **선택적으로** 제공합니다.
- DB는 `(agent_id, idempotency_key)`를 unique로 강제해 중복 업로드를 막습니다.
- Coordinator는 중복(idempotency conflict)을 **에러로 취급하지 않고**, “이미 처리됨(deduped)”으로 응답합니다.

권장 키 예시:
- `hook:completed:<ts_or_nonce>`
- `task.injected:<task_id>`
- `telegram.webhook:<update_id>`

### Task claim (`POST /v1/tasks/claim`)

- 네트워크 재시도/타임아웃으로 인해 “같은 요청이 두 번 처리되어 다른 task를 가져가는” 문제를 막기 위해,
  클라이언트는 `idempotency_key`를 제공할 수 있습니다.
- Coordinator는 `(agent_id, idempotency_key) → task_id` 매핑을 저장하고, 동일 키로 재요청 시 **동일 task를 반환**합니다.

### Task complete/fail (`POST /v1/tasks/complete`, `POST /v1/tasks/fail`)

- 상태 전이를 **idempotent** 하게 처리합니다.
  - 이미 `done`인 task에 대해 `complete`를 재호출해도 `done_at`은 변경하지 않습니다.
  - 이미 `failed`인 task에 대해 `fail`을 재호출해도 추가 변화 없이 현재 상태를 반환합니다.

## Notes / References

- 스키마 파일: `supabase/migrations/0001_init.sql`
- “중앙 저장소로 로컬 파일 사용 금지”는 Coordinator DB를 의미(Agent 로컬 캐시/임시파일은 허용).
