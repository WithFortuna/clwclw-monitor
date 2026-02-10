# Multi-tenancy: 사용자별 리소스 분리

## 요구사항
- 모든 리소스(agents, channels, chains, tasks, events, task_inputs)에 user_id 스코핑
- 에이전트 인증 플로우 (인가 코드 방식)
- SSE 스트림 사용자 필터링

## 작업 목록
- [x] DB 마이그레이션 (0010_multi_tenancy.sql)
- [x] 모델 변경 (UserID 필드 추가, AuthCode 구조체)
- [x] 인증 미들웨어 변경 (context에 user_id 주입)
- [x] Store 인터페이스 변경 (userID 파라미터 추가)
- [x] Memory Store 구현 변경
- [x] Postgres Store 구현 변경
- [x] 핸들러 변경 (userID 전달)
- [x] 에이전트 인증 엔드포인트 (auth code flow)
- [x] 에이전트 인증 페이지 (agent-auth.html)
- [x] 에이전트 프로세스 변경 (login 커맨드)
- [x] SSE 스트림 사용자 필터링
- [x] 테스트 수정 및 빌드 검증
- [x] 0000_full_init.sql 업데이트

## 변경 파일
- `supabase/migrations/0010_multi_tenancy.sql`
- `coordinator/internal/model/model.go`
- `coordinator/internal/model/user.go`
- `coordinator/internal/httpapi/middleware.go`
- `coordinator/internal/httpapi/auth.go`
- `coordinator/internal/httpapi/jwt.go`
- `coordinator/internal/httpapi/server.go`
- `coordinator/internal/httpapi/handlers.go`
- `coordinator/internal/httpapi/dashboard.go`
- `coordinator/internal/httpapi/bus.go`
- `coordinator/internal/httpapi/ui/agent-auth.html`
- `coordinator/internal/store/store.go`
- `coordinator/internal/store/memory/memory.go`
- `coordinator/internal/store/memory/auth_code.go`
- `coordinator/internal/store/postgres/postgres.go`
- `coordinator/internal/store/postgres/auth_code.go`
- `agent/clw-agent.js`
- `supabase/migrations/0000_full_init.sql`
- `coordinator/internal/store/memory/memory_test.go`
- `coordinator/internal/store/postgres/postgres_test.go`
- `coordinator/internal/httpapi/handlers_test.go`
