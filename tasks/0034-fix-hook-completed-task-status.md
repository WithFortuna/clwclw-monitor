# Fix Hook Completed Events Not Updating Task Status

## 요구사항
- REQUIREMENTS.md 참조: 5.5 Event Logging and Timeline (이벤트 로깅 및 타임라인)

## 문제 상황
- Claude Code 완료 시 `claude.hook` 이벤트는 발생하지만 태스크 상태가 `in_progress`에서 `done`으로 변경되지 않음
- Agent의 `current_task_id`가 클리어되지 않음
- 원인: CompleteTask의 새로운 검증 로직이 완료를 차단 (current_task_id 불일치 시 409 Conflict 반환)

## 작업 목록
- [x] Phase 1: Agent에 상세한 로깅 추가
  - [x] completeTask() 함수에 HTTP 응답 로깅
  - [x] Hook completed 핸들러에 진단 로깅
- [x] Phase 2: Coordinator Store에 로깅 추가
  - [x] Memory store CompleteTask에 검증 실패 로그
  - [x] Postgres store CompleteTask에 검증 실패 로그
- [ ] Phase 3: 진단 실행 및 근본 원인 파악
- [ ] Phase 4: 근본 원인에 따른 수정
  - [ ] 검증 로직 조정 또는
  - [ ] 상태 동기화 수정

## 변경 파일
- `agent/clw-agent.js` - 로깅 개선
- `coordinator/internal/store/memory/memory.go` - 검증 실패 로깅
- `coordinator/internal/store/postgres/postgres.go` - 검증 실패 로깅
