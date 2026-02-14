# Fix: 기존 체인에 태스크 추가 시 sequence 미설정으로 claim 불가 버그

## 요구사항
- REQUIREMENTS.md 참조: 체인 태스크 관리

## 작업 목록
- [x] 서버 핸들러에서 sequence 자동 계산 로직 추가
- [x] Postgres Store CreateTask 후 done 체인 상태를 queued로 복원

## 변경 파일
- `coordinator/internal/httpapi/handlers.go` - sequence 자동 계산 로직
- `coordinator/internal/store/postgres/postgres.go` - CreateTask 후 chain status 업데이트
