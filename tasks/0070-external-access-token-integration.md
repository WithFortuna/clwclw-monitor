# External Access Token Integration

## 요구사항
- CoinScope(CSPG) Slack 봇이 에러 감지 시 자동으로 task 생성
- 사용자가 설정 화면에서 access token 발급 → CSPG 환경변수 등록 → API 호출

## 작업 목록
- [x] `supabase/migrations/0018_access_tokens.sql` — DB 마이그레이션
- [x] `coordinator/internal/model/user.go` — AccessToken 모델 추가
- [x] `coordinator/internal/store/store.go` — Store 인터페이스 6개 메서드 추가
- [x] `coordinator/internal/store/memory/memory.go` — 메모리 구현
- [x] `coordinator/internal/store/postgres/access_token.go` — Postgres 구현
- [x] `coordinator/internal/store/postgres/postgres.go` — GetChannelByNameAndUserID, GetChainByNameAndChannelID
- [x] `coordinator/internal/httpapi/middleware.go` — externalAuthMiddleware + bypass
- [x] `coordinator/internal/httpapi/access_token.go` — 핸들러
- [x] `coordinator/internal/httpapi/server.go` — 라우트 등록
- [x] `coordinator/internal/httpapi/ui/dashboard.html` — Settings 섹션

## 변경 파일
- `supabase/migrations/0018_access_tokens.sql`
- `coordinator/internal/model/user.go`
- `coordinator/internal/store/store.go`
- `coordinator/internal/store/memory/memory.go`
- `coordinator/internal/store/postgres/access_token.go` (신규)
- `coordinator/internal/store/postgres/postgres.go`
- `coordinator/internal/httpapi/middleware.go`
- `coordinator/internal/httpapi/access_token.go` (신규)
- `coordinator/internal/httpapi/server.go`
- `coordinator/internal/httpapi/ui/dashboard.html`
