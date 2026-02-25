# Hook 전달 보장: Durable Queue + agentd 복구 + worker pull

## 요구사항
- `hook completed|waiting` 이벤트 전달 경로를 best-effort IPC에서 at-least-once 전달 보장으로 강화한다.
- `agentd`/worker 연결 단절 상황에서도 이벤트 유실 없이 복구 가능해야 한다.
- 모노레포 실행 및 `npm` 설치 실행 모두에서 일관된 queue 경로를 사용해야 한다.

## 작업 목록
- [x] `REQUIREMENTS.md`에 durable hook 전달 보장 요구사항 추가
- [x] `agent/clw-agent.js`에 디스크 기반 hook durable queue(pending/ack/deadletter) 추가
- [x] hook 커맨드에서 queue 선저장 후 IPC 전달 시도하도록 변경
- [x] `agentd`에 backlog replay/ACK 처리/재시도 및 복구 로직 추가
- [x] worker에 agentd 자동 재연결(backoff) + queue pull 처리 로직 추가
- [x] 최소 검증(문법 체크/실행 가능한 범위 테스트) 수행 (`node --check agent/clw-agent.js`)
- [x] 작업 완료 후 체크리스트 상태 업데이트

## 변경 파일
- `REQUIREMENTS.md`
- `tasks/0082-durable-hook-delivery-queue-and-recovery.md`
- `tasks/INDEX.md`
- `agent/clw-agent.js`
