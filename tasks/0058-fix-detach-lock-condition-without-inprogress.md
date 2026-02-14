# Fix: Detach 시 in_progress 없음에도 Chain이 locked 되는 문제

## 요구사항
- REQUIREMENTS.md 참조: 4.4.2 Detach 및 Locked Task 처리
- detach 시 `in_progress` Task가 `locked`로 전환된 경우에만 Chain을 `locked`로 둔다
- detach 시 `in_progress` Task가 없으면 Chain 상태를 Task 집합 기준으로 재평가한다

## 작업 목록
- [x] REQUIREMENTS.md 조건 문구 보강
- [x] Memory Store detach 상태 전환 로직 수정
- [x] Postgres Store detach 상태 전환 로직 수정
- [x] Memory/Postgres 회귀 테스트 추가
- [x] 관련 테스트 실행

## 변경 파일
- `REQUIREMENTS.md`
- `coordinator/internal/store/memory/memory.go`
- `coordinator/internal/store/memory/memory_test.go`
- `coordinator/internal/store/postgres/postgres.go`
- `coordinator/internal/store/postgres/postgres_test.go`
