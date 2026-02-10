# UI 기반 에이전트 채널 할당

## 요구사항
- REQUIREMENTS.md 참조: 4.10 UI 기반 에이전트 채널 할당

## 작업 목록
- [x] `PATCH /v1/agents/{id}/channels` API 핸들러 추가
- [x] `GET /v1/agents/{id}` API 핸들러 추가 (agent polling용)
- [x] 라우트 등록 (server.go)
- [x] 대시보드 UI에서 에이전트별 채널 인라인 편집 기능
- [x] agent work 루프에서 서버 채널 구독 정보 polling (~30초 간격)

## 설계 결정
- 기존 `meta.subscriptions` JSONB 필드 활용 (새 DB 컬럼/마이그레이션 불필요)
- Store 인터페이스 변경 없음 (기존 `GetAgent` + `UpsertAgent` 재사용)

## 변경 파일
- `coordinator/internal/httpapi/handlers.go` — `handleGetAgent`, `handleAgentUpdateChannels` 추가
- `coordinator/internal/httpapi/server.go` — 라우트 등록
- `coordinator/internal/httpapi/ui/app.js` — Subscriptions 셀 인라인 편집
- `agent/clw-agent.js` — `startWorkLoop()` 내 서버 채널 polling
