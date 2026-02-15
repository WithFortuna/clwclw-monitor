# UI: Queued/Chain popover 입력 중 refresh 리렌더 경합 수정

## 요구사항
- REQUIREMENTS.md 참조: 4.3 작업 태스크 보드
- 체인 생성 popover와 Queued task 생성 popover 입력 중 auto refresh(SSE/poll)로 인한 리렌더로 포커스/입력 draft가 끊기지 않아야 한다.

## 작업 목록
- [x] 현재 refresh/rerender 흐름 분석 및 원인 식별
- [x] popover 입력 중에는 백그라운드 refresh에서 task board 리렌더를 건너뛰도록 수정
- [x] 체인/queued popover 생성 완료 경로는 강제 리렌더로 즉시 반영되도록 분기 추가
- [ ] 변경 동작 수동 검증

## 변경 파일
- `REQUIREMENTS.md`
- `tasks/0061-fix-queued-popover-refresh-focus-race.md`
- `coordinator/internal/httpapi/ui/app.js`
