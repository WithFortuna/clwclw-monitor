# 에이전트 대시보드 목록 순서 흔들림(리렌더링) 수정

## 요구사항
- REQUIREMENTS.md 참조: 4.2 웹뷰 대시보드
- 에이전트 목록 응답은 이름 오름차순으로 고정해 리렌더링 시 순서가 바뀌지 않아야 한다.

## 작업 목록
- [x] `ListAgents` 정렬 기준(메모리/포스트그레스) 점검
- [x] 메모리 스토어의 에이전트 목록 정렬을 이름 오름차순으로 수정
- [x] 포스트그레스 스토어의 에이전트 목록 쿼리를 이름 오름차순으로 수정
- [x] 정렬 회귀 테스트 추가
- [x] 체크리스트 완료 처리

## 변경 파일
- `REQUIREMENTS.md`
- `tasks/0062-fix-agent-list-order-jitter.md`
- `tasks/INDEX.md`
- `coordinator/internal/store/memory/memory.go`
- `coordinator/internal/store/postgres/postgres.go`
- `coordinator/internal/store/memory/memory_test.go`
- `coordinator/internal/store/postgres/postgres_test.go`
