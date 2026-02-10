# Landing Page + User Authentication

## 요구사항
- 랜딩페이지: Toss 스타일의 서비스 소개 페이지
- 로그인/회원가입: ID/PW 기반 (비밀번호: 대문자 포함, 특수문자 포함, 최소 6자)
- JWT 토큰 기반 세션 관리
- 오픈 회원가입
- 기존 대시보드는 로그인 후에만 접근 가능

## 작업 목록
- [x] User 모델 추가 (model/user.go)
- [x] Store 인터페이스에 User 메서드 추가
- [x] Memory Store User 구현
- [x] Postgres Store User 구현 + 마이그레이션
- [x] JWT 유틸리티 (생성/검증)
- [x] Auth 핸들러 (register, login, verify)
- [x] 미들웨어 업데이트 (JWT 지원)
- [x] 랜딩 페이지 UI (Toss 스타일)
- [x] 대시보드 인증 가드

## 변경 파일
- `coordinator/internal/model/user.go` (신규)
- `coordinator/internal/store/store.go`
- `coordinator/internal/store/memory/user.go` (신규)
- `coordinator/internal/store/memory/memory.go`
- `coordinator/internal/store/postgres/user.go` (신규)
- `coordinator/internal/httpapi/auth.go` (신규)
- `coordinator/internal/httpapi/jwt.go` (신규)
- `coordinator/internal/httpapi/middleware.go`
- `coordinator/internal/httpapi/server.go`
- `coordinator/internal/httpapi/ui.go`
- `coordinator/internal/httpapi/ui/landing.html` (신규)
- `coordinator/internal/httpapi/ui/dashboard.html` (index.html에서 이름 변경)
- `coordinator/internal/httpapi/ui/app.js`
- `coordinator/internal/config/config.go`
- `supabase/migrations/0009_users.sql` (신규)
