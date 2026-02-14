# Notification System for Setup Waiting Agents

## 요구사항
- REQUIREMENTS.md 참조: Agent가 setup_waiting 상태일 때 대시보드에 알림 표시
- SSE 기반 실시간 알림 → 사용자에게 "Claude Code 백그라운드 실행" yes/no 프롬프트

## 작업 목록
- [x] bus.go: busEvent에 Payload 필드 추가, EventNotification 상수, PublishWithPayload 메서드
- [x] notifications.go (신규): 알림 중복 방지 tracker (에이전트별 5분 cooldown)
- [x] handlers.go: heartbeat에서 setup_waiting 감지 → 알림 발송
- [x] handlers.go: SSE에서 notification 이벤트 분리
- [x] server.go: notificationTracker 추가, dismiss 엔드포인트 등록
- [x] handlers.go: dismiss API 핸들러
- [x] dashboard.html: 알림 벨 버튼 + 알림 패널 HTML
- [x] styles.css: 플로팅 벨, 패널, 알림 아이템 스타일
- [x] app.js: SSE notification 리스너, 알림 상태관리, 렌더링, yes/no 액션

## 변경 파일
- `coordinator/internal/httpapi/bus.go`
- `coordinator/internal/httpapi/notifications.go` (신규)
- `coordinator/internal/httpapi/server.go`
- `coordinator/internal/httpapi/handlers.go`
- `coordinator/internal/httpapi/ui/dashboard.html`
- `coordinator/internal/httpapi/ui/styles.css`
- `coordinator/internal/httpapi/ui/app.js`
