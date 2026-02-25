# Specific Error Handling in HTTP Handlers

## 요구사항
- generic `400 "invalid_request"` 대신 구체적인 HTTP status + error code 반환
- `ErrNotFound` → 404, `ErrConflict` → 409, unexpected → 500 `internal`
- store 검증 에러(errWithCode)는 핸들러 레벨에서 조기 검증으로 처리

## 작업 목록
- [x] memory.go: chain_id_not_found errWithCode → store.ErrNotFound
- [x] handlers.go: handleAgentsHeartbeat 400 → 500
- [x] handlers.go: handleChannels POST - name 검증 + switch 전환
- [x] handlers.go: handleChains POST - 필드 검증 + switch 전환
- [x] handlers.go: handleTasks POST - 필드 검증 + switch 전환
- [x] handlers.go: handleTasksClaim default 400 → 500
- [x] handlers.go: handleTasksAssign default 400 → 500
- [x] handlers.go: handleTasksComplete default 400 → 500
- [x] handlers.go: handleTasksFail default 400 → 500
- [x] handlers.go: handleTaskInputs if-else → switch
- [x] handlers.go: handleTaskInputsClaim default 400 → 500
- [x] handlers.go: handleEvents POST - 필드 검증 + default 500
- [x] handlers.go: handleChainAssignAgent default 400 → 500
- [x] handlers_test.go: NonExistentChainID 테스트 기대값 400 → 404

## 변경 파일
- `coordinator/internal/store/memory/memory.go`
- `coordinator/internal/httpapi/handlers.go`
