# Agent 대시보드 Subscription 드롭다운 편집 안정화

## 배경
- Agent 대시보드 Subscription 드롭다운이 편집 중 자동 갱신과 충돌해 정상 동작하지 않는 문제가 보고됨.
- 본 작업은 `REQUIREMENTS.md`의 `4.10 UI 기반 에이전트 채널 할당` 요구사항 이행 품질을 보강한다.

## 작업 항목
- [x] Subscription 드롭다운 편집 중 리렌더 경합 원인 확인
- [x] 편집 중(`select`) 상태에서는 Agent 테이블 리렌더를 건너뛰도록 수정
- [x] 편집 진입 가드(중복 편집 방지) 보강
- [x] 동작 검증 및 task 체크리스트 완료 처리

## 변경 파일
- `coordinator/internal/httpapi/ui/app.js`
- `tasks/0055-fix-subscription-dropdown-edit-refresh-race.md`
- `tasks/INDEX.md`

## 구현 메모
- 원인: Agent 테이블 리렌더 스킵 조건이 `input` 편집만 고려하고 있어, 실제 `select` 드롭다운 편집 중에는 auto refresh/SSE 리렌더로 편집 UI가 끊겼다.
- 수정: `renderAgents()`의 편집 감지를 `.subs-cell select, .subs-cell input`으로 확장해 드롭다운 편집 중 리렌더를 건너뛰도록 했다.
- 보강: Subscription 셀 클릭 시 중복 편집 진입 가드를 `select`까지 포함하도록 변경했다.
