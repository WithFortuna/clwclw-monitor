# UI 수정: Task 생성 시 Chain 선택 + Task Board Channel 기준 재구조화

## 요구사항
- Task 생성 시 chain 선택 드롭다운 추가
- Task Board를 Channel > Chain > Task 계층 구조로 재구조화

## 작업 목록
- [x] Task 생성 폼에 Chain 선택 드롭다운 추가 (dashboard.html)
- [x] Chain 선택 로직 구현 (app.js - fillChainSelect)
- [x] Task 생성 시 chain_id body에 포함 (app.js)
- [x] Task Board를 Channel 기준으로 재구조화 (app.js - renderTaskBoard)

## 변경 파일
- `coordinator/internal/httpapi/ui/dashboard.html`
- `coordinator/internal/httpapi/ui/app.js`
