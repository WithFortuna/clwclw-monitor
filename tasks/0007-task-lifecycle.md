# 0007 — Task Lifecycle (Complete/Fail) API + UI

Status: **Done**

## Goal

Coordinator에서 `tasks`의 상태를 `Queued → In Progress → Done/Failed`로 끝까지 흘릴 수 있게 한다.

## Acceptance Criteria

- [x] `POST /v1/tasks/complete` 구현(최소: done 처리)
- [x] (옵션) `POST /v1/tasks/fail` 구현(실패 처리/재시도 정책은 추후)
- [x] UI에서 In Progress 작업을 Done으로 마킹 가능(폴링 기반)
- [x] UI에서 In Progress 작업을 Failed로 마킹 가능(폴링 기반)
- [x] Agent(로컬)에서 claim → 주입 → 완료 훅과 연동하여 자동 complete (`agent/clw-agent.js work` + `hook completed`)

## Notes / References

- API 구현: `coordinator/internal/httpapi/handlers.go`
- 저장소 구현: `coordinator/internal/store/*`
