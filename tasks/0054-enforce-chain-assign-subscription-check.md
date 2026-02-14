# 체인 할당 시 채널 구독 검증 + 구독 추가 확인 플로우

## 배경
- 체인에 Agent를 수동 할당할 때, Agent의 `subscriptions`에 체인 채널이 없어도 그대로 할당되는 문제가 있다.
- 운영자 UX 기준으로는 미구독 상태를 즉시 막고, 필요 시 UI에서 구독 추가를 승인한 뒤 재할당할 수 있어야 한다.

## 작업 항목
- [x] `REQUIREMENTS.md`에 체인 할당 시 구독 검증/프론트 후속 플로우 요구사항 반영
- [x] 백엔드 `POST /v1/chains/{id}/assign-agent`에 채널 구독 검증 추가 및 명시적 오류 코드 반환
- [x] 프론트 체인 할당 드롭다운에서 구독 오류 시 “채널 구독 추가 여부” 확인 UX 추가
- [x] 사용자가 승인하면 `PATCH /v1/agents/{id}/channels`로 채널 구독 추가 후 체인 할당 재시도
- [x] 핸들러 테스트 추가 및 관련 테스트 실행
- [x] 체크리스트 완료 처리

## 변경 파일
- `REQUIREMENTS.md`
- `coordinator/internal/httpapi/handlers.go`
- `coordinator/internal/httpapi/handlers_test.go`
- `coordinator/internal/httpapi/ui/app.js`
- `tasks/0054-enforce-chain-assign-subscription-check.md`
- `tasks/INDEX.md`
