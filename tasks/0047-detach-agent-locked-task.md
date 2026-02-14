# Detach Agent & Locked Task Feature

## 요구사항
- REQUIREMENTS.md 참조: Agent Detach & Locked Task Resolution

## 작업 목록
- [x] Add `locked` status constants to model
- [x] Add store interface methods (DetachAgentFromChain, UpdateTaskStatus)
- [x] Implement memory store methods
- [x] Add memory store tests
- [x] Implement postgres store methods
- [x] Add postgres store tests
- [x] Create DB migration
- [x] Add HTTP handlers (detach, task status update, assign-agent)
- [x] Register routes
- [x] Update dashboard UI (app.js)
- [x] Add CSS styles
- [x] Run tests

## 변경 파일
- `coordinator/internal/model/model.go`
- `coordinator/internal/store/store.go`
- `coordinator/internal/store/memory/memory.go`
- `coordinator/internal/store/memory/memory_test.go`
- `coordinator/internal/store/postgres/postgres.go`
- `coordinator/internal/store/postgres/postgres_test.go`
- `coordinator/internal/httpapi/handlers.go`
- `coordinator/internal/httpapi/server.go`
- `coordinator/internal/httpapi/ui/app.js`
- `coordinator/internal/httpapi/ui/styles.css`
- `supabase/migrations/0013_locked_status.sql`
