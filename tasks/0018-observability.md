# 0018 — Observability (Logs / Metrics / Tracing)

Status: **Todo**

## Goal

운영 중 장애/성능 문제를 빠르게 진단할 수 있도록 관측성을 추가한다.

## Acceptance Criteria

- [ ] Coordinator에 구조화 로그(요청 ID/correlation id 포함)를 추가한다.
- [ ] 최소 메트릭(요청 수/에러율/지연/SSE 연결 수/claim 성공률 등)을 노출한다.
- [ ] Agent/Legacy 이벤트에도 correlation id(예: task_id, idempotency_key)를 일관되게 포함한다.
- [ ] 장애 시나리오(네트워크 단절/DB 장애/웹훅 실패)에 대한 진단 가이드를 문서화한다.

## Notes / References

- Coordinator 서버/핸들러: `coordinator/internal/httpapi/*`
- Agent 래퍼: `agent/clw-agent.js`

