# 0001 — Bootstrap Coordinator (Go)

Status: **Done**

## Goal

Go 기반 Coordinator(API 서버) 골격을 만들고, 에이전트/이벤트/태스크 최소 API를 **in-memory**로 먼저 동작시키며 이후 Supabase로 교체할 수 있게 한다.

## Acceptance Criteria

- [x] `coordinator/` 디렉토리 및 기본 구조 생성
- [x] `GET /health` 동작
- [x] (선택) 공유 토큰 기반 인증 미들웨어(`Authorization: Bearer ...` 또는 `X-Api-Key`)
- [x] `POST /v1/agents/heartbeat` (에이전트 last_seen 갱신)
- [x] `POST /v1/events` (작업 이력 ingest)
- [x] `POST /v1/channels` / `GET /v1/channels`
- [x] `POST /v1/tasks` / `GET /v1/tasks`
- [x] `POST /v1/tasks/claim` (FIFO claim, 원자적 동작을 메모리에서 보장)
- [x] Supabase(Postgres) 저장소 구현으로 교체(코드 추가; 실행/검증은 Go 설치 필요)
- [x] SSE/WS 또는 Supabase Realtime 연동(SSE: `GET /v1/stream`)

## Notes / References

- 요구사항 원문: `REQUIREMENTS.md`
- 구현 위치: `coordinator/`
- 핵심 API 스펙은 추후 `0002-supabase-schema.md`와 함께 고정(스키마/인덱스/원자 claim 쿼리).
