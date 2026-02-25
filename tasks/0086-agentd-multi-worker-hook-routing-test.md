# Agentd 멀티 워커 Hook 라우팅 통합 테스트

## 요구사항
- 별도 터미널에서 `clw-agent.js hook completed`가 실행되면 `agentd`를 통해 해당 pane_id worker로 라우팅되어야 한다.
- 복수 worker가 등록된 상황에서도 대상 pane worker만 coordinator 완료 API(`POST /v1/tasks/complete`)를 호출해야 한다.

## 작업 목록
- [x] `REQUIREMENTS.md`에 멀티 worker hook 라우팅 통합 테스트 요구사항 반영
- [x] `tasks/INDEX.md`에 신규 작업 등록
- [x] `agent/tests/integration/`에 agentd 멀티 worker hook 라우팅 테스트 추가
- [x] 테스트 실행 (`node --test agent/tests/integration/hook-agentd-multi-worker-routing.test.js`)
- [x] 작업 완료 후 체크리스트 업데이트

## 변경 파일
- `REQUIREMENTS.md`
- `tasks/INDEX.md`
- `tasks/0086-agentd-multi-worker-hook-routing-test.md`
- `agent/tests/integration/hook-agentd-multi-worker-routing.test.js`
