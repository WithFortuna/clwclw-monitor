# 0050 Session Request Completion Hardening

## Goal
- `agent.automation.session_request.completed` 처리에서 토큰 키 중복을 제거하고, 서버가 이벤트를 받았을 때 세션 요청 task를 확실히 `done` 처리하도록 보강한다.

## Checklist
- [x] `REQUIREMENTS.md`에 canonical 토큰 키/오류 반환 정책 반영
- [x] Agent 전송 payload에서 토큰 키를 `agent_session_request_token` 하나로 정리
- [x] Coordinator가 `task_id` + 토큰 조합으로 대상 task 식별 가능하도록 보강
- [x] 전용 완료 처리에서 상태 전이 실패를 조용히 무시하지 않도록 수정
- [x] 관련 핸들러 테스트 보강
