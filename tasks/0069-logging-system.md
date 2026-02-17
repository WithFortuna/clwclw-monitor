# Logging System (slog)

## 요구사항
- REQUIREMENTS.md 참조: 관측성/로깅

## 작업 목록
- [x] Config에 LogLevel, LogFile, LogFormat 필드 추가
- [x] logger 패키지 생성 (coordinator/internal/logger/logger.go)
- [x] main.go에서 로거 초기화 + log.Printf → slog 교체
- [x] middleware.go의 log.Printf → slog 교체
- [x] memory/memory.go, postgres/postgres.go의 log.Printf → slog 교체
- [x] auth.go 핸들러에 에러 로깅 추가
- [x] handlers.go 500 응답에 에러 로깅 추가
- [x] go build + go vet 통과 확인

## 변경 파일
- `coordinator/internal/config/config.go`
- `coordinator/internal/logger/logger.go` (신규)
- `coordinator/cmd/coordinator/main.go`
- `coordinator/internal/httpapi/middleware.go`
- `coordinator/internal/httpapi/auth.go`
- `coordinator/internal/httpapi/handlers.go`
- `coordinator/internal/store/memory/memory.go`
- `coordinator/internal/store/postgres/postgres.go`
