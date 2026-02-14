# 체인 소유자 claim 규칙 보정 (detach/lease 전 소유 고정)

## 배경
- 체인 소유자가 `detach` 전까지(또는 lease 만료 전까지) 유지되어야 하는데, 상태(`in_progress`) 기반 필터로 인해 소유 체인 인식이 깨지는 문제가 있음.
- 소유 체인에 새 Task가 추가됐을 때 기존 소유자가 독점 claim해야 하는데, 체인 상태 필터 때문에 claim되지 않는 경우가 발생함.

## 작업 항목
- [x] `REQUIREMENTS.md`에 완료/실패 후 ownership 유지 및 신규 Task claim 규칙 명시
- [x] 메모리 스토어 `ClaimTask`의 체인 소유 인식/후보 선정 조건 수정
- [x] Postgres `claim_task()` 함수(마이그레이션)에서 동일 규칙 반영
- [x] 회귀 테스트 추가/수정 (소유 체인 신규 Task claim 시나리오)
- [x] 체크리스트 완료 처리

## 변경 파일
- `REQUIREMENTS.md`
- `coordinator/internal/store/memory/memory.go`
- `coordinator/internal/store/memory/memory_test.go`
- `supabase/migrations/0012_chain_ownership.sql`
- `supabase/migrations/0015_fix_claim_task_owner_persistence.sql`
- `tasks/INDEX.md`
