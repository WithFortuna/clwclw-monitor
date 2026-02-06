# 0013 — Hardening Basics (Security / Reliability)

Status: **Todo**

## Goal

Coordinator/Agent/Legacy 경로를 운영 환경에 가까운 수준으로 하드닝해 악의적/오작동 입력에도 안정적으로 동작하게 한다.

## Acceptance Criteria

- [ ] Coordinator API에 기본 방어를 추가한다(요청 바디 크기 제한, 타임아웃, 에러 메시지 표준화, 안전한 CORS 기본값 등).
- [ ] Coordinator에 rate limit(예: IP/API Key 단위)을 추가해 남용/스팸을 완화한다.
- [ ] SSE(`/v1/stream`)에 연결 제한/idle timeout 등 운영 안전장치를 둔다.
- [ ] Agent 업로드(heartbeat/events/complete/fail)가 네트워크 오류에서 재시도/백오프(최소)로 안정적으로 동작한다.
- [ ] Legacy 인바운드(특히 Telegram)에는 “웹훅 요청 진짜 여부”를 강화할 수 있는 옵션을 추가한다(기존 whitelist 기반은 유지).
- [ ] 문서에 보안/운영 기본값과 권장 설정을 정리한다.

## Notes / References

- Coordinator 핸들러: `coordinator/internal/httpapi/handlers.go`
- Coordinator 미들웨어: `coordinator/internal/httpapi/middleware.go`
- Legacy Telegram webhook: `Claude-Code-Remote/src/channels/telegram/webhook.js`
- Legacy LINE webhook(서명 검증): `Claude-Code-Remote/src/channels/line/webhook.js`

