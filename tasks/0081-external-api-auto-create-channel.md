# External API: 채널 자동 생성

## 요구사항
- 외부 API(`POST /v1/external/tasks`)에서 채널명이 존재하지 않으면 404 대신 자동 생성

## 작업 목록
- [x] `validScopes`에 `channels:write`, `chains:write` 추가
- [x] `handleExternalCreateTask` - 채널 미존재 시 자동 생성으로 변경

## 변경 파일
- `coordinator/internal/httpapi/access_token.go`
