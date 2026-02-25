# Remove AGENT_ID Bootstrap + Current Task Debug Logging

## 요구사항
- Agent는 프로세스 시작 시 `AGENT_ID` 환경변수로 `_currentAgentId`를 초기화하지 않아야 한다.
- `hook completed` 처리 중 current task 조회(`GET /v1/agents/{id}/current-task`) 요청/응답을 `debug` 레벨로 기록해야 한다.

## 작업 목록
- [x] `REQUIREMENTS.md`에 요구사항 반영
- [x] `tasks/0086-remove-agent-id-env-bootstrap-and-current-task-debug-log.md` 작업 파일 생성
- [x] `agent/clw-agent.js`에서 `_currentAgentId`의 `AGENT_ID` 초기화 제거
- [x] `agent/clw-agent.js`에서 current task 조회 요청/응답 debug 로그 추가
- [x] `agent/clw-agent.js` 문법 체크 실행 (`node --check agent/clw-agent.js`)
- [x] 작업 완료 후 체크리스트 업데이트

## 변경 파일
- `REQUIREMENTS.md`
- `tasks/0086-remove-agent-id-env-bootstrap-and-current-task-debug-log.md`
- `agent/clw-agent.js`
