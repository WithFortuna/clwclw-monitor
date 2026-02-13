# 0049 Request Session Token Completion

## Goal
- `request_claude_session` 완료를 일반 태스크 완료와 분리하고, `agent_session_request_token` 기반으로 정확한 세션 요청 태스크를 `done` 처리한다.

## Checklist
- [x] `REQUIREMENTS.md`에 전용 이벤트 + 토큰 기반 완료 요구사항 추가
- [x] Coordinator가 세션 요청 태스크 생성 시 `agent_session_request_token` 발급/저장
- [x] Agent가 세션 할당 완료 시 전용 이벤트(`agent.automation.session_request.completed`) 전송
- [x] 전용 이벤트 payload에 `agent_session_request_token` 포함
- [x] Coordinator가 토큰으로 세션 요청 태스크를 찾아 `done` 처리
- [x] Postgres 스키마/마이그레이션에 `agent_session_request_token` 컬럼 및 unique 인덱스 추가
- [x] 관련 핸들러 테스트 추가
