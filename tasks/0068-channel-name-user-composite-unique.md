# Channel Name + User ID 복합 유니크 제약 조건

## 요구사항
- 채널명의 유니크 제약을 글로벌 → (name, user_id) 복합 유니크로 변경
- 같은 유저 내에서만 채널명 중복 불가, 다른 유저는 동일 채널명 허용

## 작업 목록
- [x] DB 마이그레이션 SQL 작성 (0017)
- [x] 0000_full_init.sql 반영
- [x] Memory Store CreateChannel 로직 수정
- [x] Postgres Store 테스트 스키마 수정

## 변경 파일
- `supabase/migrations/0013_channel_name_user_composite_unique.sql`
- `supabase/migrations/0000_full_init.sql`
- `coordinator/internal/store/memory/memory.go`
- `coordinator/internal/store/postgres/postgres_test.go`
