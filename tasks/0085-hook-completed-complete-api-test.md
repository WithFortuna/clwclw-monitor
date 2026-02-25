# Hook Completed Complete API 테스트

## 요구사항
- `clw-agent.js hook completed` 실행 시, 에이전트가 현재 task를 조회한 뒤 `POST /v1/tasks/complete`를 호출해야 한다.
- 테스트는 BDD(`Given-When-Then`) 시나리오 이름으로 작성한다.

## 작업 목록
- [x] `REQUIREMENTS.md`에 해당 테스트 요구사항 반영
- [x] `tasks/INDEX.md`에 신규 작업 등록
- [x] `agent/tests/integration/`에 hook completed complete API 호출 검증 테스트 추가
- [x] 테스트 실행 (`node --test agent/tests/integration/hook-completed-complete-api.test.js`)
- [x] 작업 완료 후 체크리스트 업데이트

## 변경 파일
- `REQUIREMENTS.md`
- `tasks/INDEX.md`
- `tasks/0085-hook-completed-complete-api-test.md`
- `agent/tests/integration/hook-completed-complete-api.test.js`
