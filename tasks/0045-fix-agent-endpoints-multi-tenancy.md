# Fix Agent Endpoints Multi-Tenancy

## 요구사항
- Multi-tenancy 도입으로 모든 API 엔드포인트는 user_id 기반 접근 제어 필요
- Agent 조회 시 요청자의 user_id와 agent의 user_id 일치 여부 검증 필요

## 문제점
- `GET /v1/agents/{id}/current-task`: user_id 검증 없음
- `GET /v1/agents/{id}`: user_id 검증 없음
- 다른 유저의 agent 정보에 무단 접근 가능

## 작업 목록
- [x] `handleAgentCurrentTask`: user_id 검증 추가
- [x] `handleGetAgent`: user_id 검증 추가
- [ ] 에러 케이스 테스트

## 변경 파일
- `coordinator/internal/httpapi/handlers.go`
