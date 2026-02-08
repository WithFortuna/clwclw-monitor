# Fix current_task_id Not Cleared After Task Completion

## 요구사항
- REQUIREMENTS.md 참조: Agent 상태 관리 (dual status system)

## 문제
- 작업이 완료되어도 agent의 `current_task_id`가 클리어되지 않음
- Dashboard에서 과거 task가 계속 "Current Task"로 표시됨
- Memory Store와 Postgres Store의 동작 불일치
- Postgres Store에서 `claude_status`를 자동으로 업데이트하여 dual status 원칙 위반

## 작업 목록
- [x] Memory Store의 ClaimTask에서 current_task_id 설정 추가
- [x] Memory Store의 AssignTask에서 current_task_id 설정 추가
- [x] Memory Store의 CompleteTask에서 current_task_id 클리어 추가
- [x] Memory Store의 FailTask에서 current_task_id 클리어 추가
- [x] Postgres Store의 ClaimTask에서 status 자동 업데이트 제거
- [x] Postgres Store의 AssignTask에서 status 자동 업데이트 제거
- [x] Postgres Store의 CompleteTask에서 status 자동 업데이트 제거
- [x] Postgres Store의 FailTask에서 status 자동 업데이트 제거
- [x] 빌드 검증

## 변경 파일
- `coordinator/internal/store/memory/memory.go`
- `coordinator/internal/store/postgres/postgres.go`

## 구현 원칙
1. **current_task_id**: Task 상태에 따라 자동 관리
   - Claim/Assign 시: task ID로 설정
   - Complete/Fail 시: null로 클리어

2. **claude_status**: Heartbeat만이 유일한 정보 출처
   - Task 작업 시 자동 업데이트하지 않음
   - Agent가 heartbeat에서 보고하는 값만 사용

## 결과
- 작업 완료 시 agent의 current task가 자동으로 "not assigned"로 표시됨
- Memory Store와 Postgres Store의 동작 일관성 확보
- Dual status architecture 원칙 준수
