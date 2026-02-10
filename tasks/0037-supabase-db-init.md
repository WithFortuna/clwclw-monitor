# Supabase DB 초기화 통합 스크립트

## 요구사항
- Supabase DB에 테이블이 없는 상태에서 한 번에 모든 테이블 생성
- 기존 마이그레이션 0001~0009의 버그 수정 반영

## 작업 목록
- [x] 기존 마이그레이션 분석 및 버그 식별
- [x] `supabase/migrations/0000_full_init.sql` 통합 스크립트 생성

## 변경 파일
- `supabase/migrations/0000_full_init.sql`
