# Subscription 인라인 편집 포커스/저장 UX 수정

## 배경
- 대시보드 Agent 테이블의 `subscriptions` 인라인 편집에서 Enter, Esc, blur 동작 후 input 포커스가 해제되지 않거나 저장 여부를 알기 어려운 문제가 있음.

## 작업 항목
- [x] 기존 인라인 편집 로직 분석 및 포커스 고착 원인 확인
- [x] Enter/blur 저장, Esc 취소, 포커스 해제 동작을 일관되게 수정
- [x] 저장 상태(saving/saved/error) 표시 추가
- [x] 문서(`REQUIREMENTS.md`) 요구사항 반영
- [x] 변경 확인 및 체크리스트 완료 처리

## 변경 파일
- `coordinator/internal/httpapi/ui/app.js`
- `REQUIREMENTS.md`
