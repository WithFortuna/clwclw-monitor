# 0005 — Web Dashboard + Task Board (MVP)

Status: **In Progress**

## Goal

인터넷에서 접근 가능한 대시보드로 “활성 에이전트/상태/이력/태스크(Queued/In Progress/Done)”를 확인/운영한다.

## Acceptance Criteria

- [x] 에이전트 리스트(상태, last_seen, current_task)
- [x] 에이전트 작업 이력 타임라인(events) (최근 이벤트 리스트; per-agent 필터링은 추후)
- [x] 채널별 태스크 보드(Queued/In Progress/Done)
- [x] 실시간 반영(SSE: `GET /v1/stream` + EventSource)
- [x] 1,000 동접 조회를 고려한 최소 캐싱/폴링 전략(`/v1/dashboard` 단일 호출 + 서버 TTL cache)

## Notes / References

- Coordinator API가 먼저 준비돼야 함: `tasks/0001-bootstrap-coordinator.md`
- 구현(정적 UI): `coordinator/internal/httpapi/ui/index.html`
