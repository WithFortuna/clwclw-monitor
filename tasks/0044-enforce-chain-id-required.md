# Enforce chain_id Required for All Tasks

## 요구사항
- REQUIREMENTS.md 참조: 모든 태스크는 반드시 체인에 속해야 함

## 작업 목록
- [x] Store 레벨: CreateTask에 chain_id 필수 검증 추가 (memory + postgres)
- [x] ClaimTask: standalone 태스크 fallback 제거 (memory)
- [x] SQL claim_task 함수: 3번째 fallback 제거 (새 마이그레이션)
- [x] handleAgentsRequestSession: 자동 체인 생성 적용
- [x] DB 스키마: chain_id NOT NULL + ON DELETE CASCADE 마이그레이션
- [x] UI: Standalone Tasks 섹션 제거
- [x] 테스트 수정 (memory + postgres)

## 변경 파일
- `coordinator/internal/store/memory/memory.go`
- `coordinator/internal/store/postgres/postgres.go`
- `coordinator/internal/httpapi/handlers.go`
- `coordinator/internal/httpapi/ui/app.js`
- `coordinator/internal/store/memory/memory_test.go`
- `coordinator/internal/store/postgres/postgres_test.go`
- `supabase/migrations/0011_enforce_chain_id.sql`
