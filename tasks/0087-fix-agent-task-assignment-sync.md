# Agent-Task 할당 동기화 정합성 보강

## 요구사항
- `tasks.assigned_agent_id`와 `agents.current_task_id`는 어느 한쪽만 변경되지 않도록 항상 함께 동기화되어야 한다.
- `done`/`failed` Task는 과거 이력 보존을 위해 `assigned_agent_id`를 유지할 수 있다.
- `in_progress` 상태에서는 하나의 Agent가 동시에 하나의 Task만 담당할 수 있어야 한다.

## 작업 목록
- [x] `REQUIREMENTS.md`에 동기화 정합성 요구사항 반영
- [x] `tasks/0087-fix-agent-task-assignment-sync.md` 작업 파일 생성
- [x] `tasks/INDEX.md`에 신규 작업 등록
- [x] Memory Store에 Agent-Task 동기화 편의 메서드 추가 및 claim/assign/complete/fail/detach 경로 적용
- [x] Postgres Store에 Agent-Task 동기화 편의 메서드 추가 및 claim/assign/complete/fail/detach 경로 적용
- [x] Postgres 스키마에 `in_progress` 기준 Agent 단일 task 제약(부분 unique index) 반영
- [x] 관련 테스트 추가/수정
- [x] 변경 파일 `gofmt` 적용
- [x] 테스트 실행 및 결과 확인
- [x] 작업 완료 후 체크리스트 업데이트

## 변경 파일
- `REQUIREMENTS.md`
- `tasks/INDEX.md`
- `tasks/0087-fix-agent-task-assignment-sync.md`
- `coordinator/internal/store/memory/memory.go`
- `coordinator/internal/store/memory/memory_test.go`
- `coordinator/internal/store/postgres/postgres.go`
- `coordinator/internal/store/postgres/postgres_test.go`
- `supabase/migrations/0000_full_init.sql`
- `supabase/migrations/0019_single_in_progress_task_per_agent.sql`
