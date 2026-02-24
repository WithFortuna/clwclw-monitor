# 0073 Agent Identity Lifecycle (Heartbeat issuance + Pane bind)

## Goal
- 배포 환경(Postgres)에서 agent 식별자 불일치로 `/v1/events`가 `400 invalid_request/not_found`로 실패하며 worker가 중단되는 문제를 해결한다.
- `agent_id`를 state dir 변화와 분리해 안정적으로 유지하고, pane 확보 시점에 명시적으로 bind한다.

## Checklist
- [x] `REQUIREMENTS.md`에 heartbeat 기반 `agent_id` 발급 및 pane bind 요구사항 반영
- [x] Coordinator에 `POST /v1/agents/{id}/bind-pane` API 추가
- [x] Agent heartbeat가 server-issued `agent_id`를 서버 응답으로 수신/저장하도록 수정
- [x] Agent ID를 프로세스 전역 메모리로 고정하고, `AGENT_STATE_DIR` 변경이 `agent_id`를 바꾸지 않도록 수정
- [x] skip/session 전환 시 pane 확보 후 bind 호출 + 이벤트 업로드 순서 보장
- [x] 관련 서버 테스트 추가 및 핵심 테스트 통과 확인

## Changed Files
- `REQUIREMENTS.md`
- `tasks/INDEX.md`
- `tasks/0073-agent-identity-lifecycle-heartbeat-pane-bind.md`
- `coordinator/internal/httpapi/server.go`
- `coordinator/internal/httpapi/handlers.go`
- `coordinator/internal/httpapi/handlers_test.go`
- `coordinator/README.md`
- `agent/clw-agent.js`
