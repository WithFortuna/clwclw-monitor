# Hook Delivery Debug Trace Logging

## 요구사항
- durable queue 기반 훅 전달의 전체 파이프라인을 `debug` 레벨에서 추적 가능해야 한다.
- enqueue부터 agentd 전달, worker 수신/ACK, pull/replay까지 단계별 로그를 남겨야 한다.

## 작업 목록
- [x] `REQUIREMENTS.md`에 Hook Delivery Trace 로그 요구사항 추가
- [x] `tasks/INDEX.md`에 신규 작업 등록
- [x] `agent/clw-agent.js`에 hook-flow debug 로깅 헬퍼 추가
- [x] queue enqueue/ack/deadletter 단계 로그 추가
- [x] IPC send/result 및 agentd forward/ack 단계 로그 추가
- [x] worker hook_forward/pull/replay 단계 로그 추가
- [x] 문법 체크 수행 (`node --check agent/clw-agent.js`)
- [x] 작업 완료 후 체크리스트 업데이트

## 변경 파일
- `REQUIREMENTS.md`
- `tasks/INDEX.md`
- `tasks/0084-hook-delivery-debug-trace-logging.md`
- `agent/clw-agent.js`
