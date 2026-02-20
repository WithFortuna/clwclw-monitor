# Worker Offline 시 Claude Not Running 강제

## 요구사항
- REQUIREMENTS.md 참조: 4.2 웹뷰 대시보드
- Agent의 worker 상태가 `offline`이면 Claude Code 상태는 반드시 `not running`으로 표시되어야 한다.

## 작업 목록
- [x] `REQUIREMENTS.md`에 상태 연동 규칙 추가
- [x] API 응답(`GET /v1/agents`, `GET /v1/dashboard`)에서 worker offline 시 `claude_status`를 `idle`로 정규화
- [x] UI 렌더링에서 worker offline 시 Claude 상태를 `not running`으로 강제하는 안전장치 추가
- [x] 상태 연동 회귀 테스트 추가

## 변경 파일
- `REQUIREMENTS.md`
- `tasks/0063-offline-worker-forces-claude-not-running.md`
- `tasks/INDEX.md`
- `coordinator/internal/httpapi/agent_status.go`
- `coordinator/internal/httpapi/agent_status_test.go`
- `coordinator/internal/httpapi/handlers.go`
- `coordinator/internal/httpapi/dashboard.go`
- `coordinator/internal/httpapi/ui/app.js`
