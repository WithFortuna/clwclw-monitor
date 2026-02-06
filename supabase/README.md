# Supabase (Postgres) Schema

이 디렉토리는 `REQUIREMENTS.md`의 중앙 저장소(Supabase/Postgres) 스키마 초안을 담습니다.

- 마이그레이션: `supabase/migrations/0001_init.sql`
- 핵심 테이블: `agents`, `channels`, `tasks`, `events`

## FIFO claim (원자적) 설계

Coordinator는 `tasks`에서 `status='queued'`인 레코드를 **생성 시간 순(FIFO)** 으로 1개 집어 `in_progress`로 바꾸고 `assigned_agent_id`를 세팅해야 합니다.

Postgres에서는 다음 패턴을 사용합니다.

- `SELECT ... FOR UPDATE SKIP LOCKED` + `UPDATE ... RETURNING`
- 또는 `claim_task()` 같은 DB 함수로 캡슐화

마이그레이션 파일에 예시 SQL이 포함되어 있습니다.

