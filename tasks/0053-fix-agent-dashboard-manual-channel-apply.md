# 에이전트 대시보드 수동 채널 입력 적용 점검/수정

## 배경
- 에이전트 대시보드에서 subscriptions(채널) 직접 입력 시 기대한 대로 적용되지 않는 이슈가 보고됨.

## 작업 항목
- [x] 대시보드 subscriptions 인라인 편집 저장 경로(UI → API) 점검
- [x] 서버 채널 구독 업데이트 및 후속 동작(request-session/worker polling) 불일치 원인 확인
- [x] 직접 입력 채널 적용 실패 케이스 수정
- [x] 관련 테스트/검증 수행 및 결과 기록
- [x] 체크리스트 완료 처리

## 변경 파일
- `coordinator/internal/httpapi/ui/app.js`
- `agent/clw-agent.js`
- `tasks/0053-fix-agent-dashboard-manual-channel-apply.md`
- `tasks/INDEX.md`

## 구현 메모
- subscriptions 편집 UI를 채널 목록 단일 드롭다운으로 변경했다.
- 드롭다운 선택 변경(`change`) 즉시 `PATCH /v1/agents/{id}/channels` 요청을 전송하도록 수정했다.
