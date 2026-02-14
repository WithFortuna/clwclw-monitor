# Fix: failed 선행 Task가 있어도 다음 Task claim 가능하도록 체인 게이트 완화

## 요구사항
- REQUIREMENTS.md 참조: 4.4.1 Chain Ownership 및 독점성
- 다음 Task를 `in_progress`로 올리는 조건을 "선행 Task 중 `queued`/`in_progress` 없음"으로 변경
- `done`/`failed` 선행 Task는 claim 블로킹 조건에서 제외
- Memory/Postgres 동작 일치

## 작업 목록
- [ ] REQUIREMENTS.md에 새 claim 게이트 규칙 명시
- [ ] Memory Store ClaimTask 선행 조건 로직 수정
- [ ] Postgres claim_task 함수 갱신(신규 migration 추가)
- [ ] Memory/Postgres 회귀 테스트 추가/수정
- [ ] 관련 테스트 실행

## 변경 파일
- `REQUIREMENTS.md`
- `coordinator/internal/store/memory/memory.go`
- `coordinator/internal/store/memory/memory_test.go`
- `supabase/migrations/0016_relax_chain_claim_gate_for_failed_predecessors.sql`
- `coordinator/internal/store/postgres/postgres_test.go`
