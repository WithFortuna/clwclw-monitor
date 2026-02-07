# Coordinator (Go)

`REQUIREMENTS.md`의 **Coordinator(API 서버)** 초안 구현입니다.

현재 단계에서는 DB(Supabase) 없이 **in-memory 저장소**로 먼저 API 형태/흐름을 고정합니다.  
추후 `supabase/migrations/0001_init.sql` 스키마에 맞춰 저장소를 교체합니다.

## 목표

- Agent heartbeat / Events ingest / Channels / Tasks / FIFO claim
- (옵션) 공유 토큰 인증

## Database Migrations

새로운 체인 기반 태스크 관리 기능을 위해 데이터베이스 스키마가 업데이트되었습니다. `supabase/migrations/` 디렉토리에 있는 최신 SQL 마이그레이션 파일들을 적용해야 합니다.

```bash
# Supabase CLI를 사용하여 마이그레이션을 적용합니다.
# 로컬 개발 환경에서 Supabase CLI를 설치하고 `supabase start`를 실행한 후,
# 다음 명령어를 실행하여 마이그레이션을 적용할 수 있습니다.
supabase db push

# 또는 psql과 같은 PostgreSQL 클라이언트를 사용하여 수동으로 SQL 파일을 실행합니다.
# 예시: psql -d "your_database_url" -f supabase/migrations/0004_add_chains_and_chain_to_tasks.sql
# 예시: psql -d "your_database_url" -f supabase/migrations/0005_update_claim_task_function.sql
```

## 실행(예시)

> 이 개발 환경에는 Go가 설치되어 있지 않을 수 있습니다. Go 설치 후 아래 커맨드를 사용하세요.

```bash
cd coordinator
COORDINATOR_PORT=8080 COORDINATOR_AUTH_TOKEN=devtoken go run ./cmd/coordinator
```

### 테스트 실행

PostgreSQL 저장소 테스트를 실행하려면 `DATABASE_URL` 환경변수를 설정해야 합니다.

```bash
# PostgreSQL 인스턴스에 대한 연결 문자열로 변경하세요.
# 예시: export DATABASE_URL="postgresql://postgres:password@localhost:5432/test_db?sslmode=disable"
export DATABASE_URL="your_postgres_connection_string"
go test -v ./internal/store/postgres/...

# 인메모리 저장소 테스트는 DATABASE_URL 없이 실행할 수 있습니다.
go test -v ./internal/store/memory/...
```

## 환경변수

- `COORDINATOR_PORT` (default: `8080`)
- `COORDINATOR_AUTH_TOKEN` (optional)
  - 설정하면 `Authorization: Bearer <token>` 또는 `X-Api-Key: <token>`가 필요합니다.
- `COORDINATOR_DATABASE_URL` (optional)
  - 설정하면 Postgres(Supabase) 저장소를 사용합니다.
  - 미설정 시 in-memory 저장소를 사용합니다.
  - `DATABASE_URL`도 fallback으로 지원합니다.
- `COORDINATOR_EVENT_RETENTION_DAYS` (default: `30`)
  - `events` 30일 보관을 위해, Coordinator가 주기적으로 오래된 이벤트를 삭제(purge)합니다.
  - `0`으로 설정하면 비활성화됩니다.
- `COORDINATOR_RETENTION_INTERVAL_HOURS` (default: `24`)
  - purge 주기(시간).

## API (초안)

- UI: `GET /` (static dashboard; polls API endpoints)
- `GET /health`
- `POST /v1/agents/heartbeat`
- `GET /v1/agents`
- `POST /v1/channels`
- `GET /v1/channels`
- `POST /v1/chains`
- `GET /v1/chains`
- `GET /v1/chains/{id}`
- `PUT /v1/chains/{id}`
- `DELETE /v1/chains/{id}`
- `POST /v1/tasks`
- `GET /v1/tasks`
- `POST /v1/tasks/claim` (FIFO)
- `POST /v1/tasks/assign` (manual assign)
- `POST /v1/tasks/complete`
- `POST /v1/tasks/fail`
- `POST /v1/events`
- `GET /v1/events`
- `GET /v1/dashboard` (aggregated snapshot; cached)
- `GET /v1/stream` (SSE; dashboard real-time updates)

요청/응답 스키마는 `coordinator/internal/httpapi/handlers.go`의 DTO를 기준으로 합니다.

## Idempotency (MVP)

- `POST /v1/events`: `idempotency_key` 중복 업로드는 `200 {"deduped": true}`로 처리합니다.
- `POST /v1/tasks/claim`: `idempotency_key`를 제공하면 동일 키 재시도 시 동일 task를 반환합니다.
- `POST /v1/tasks/complete|fail`: `in_progress → done/failed` 전이만 수행하며, 이미 완료/실패 상태면 값(시간 등)을 바꾸지 않습니다.
