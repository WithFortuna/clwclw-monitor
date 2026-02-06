# 0003 — Agent Bridge (Claude-Code-Remote → Coordinator)

Status: **In Progress**

## Goal

기존 `Claude-Code-Remote`의 모든 기능(알림/웹훅/주입/보안)을 “로컬 Agent”로 유지하면서, 그 실행 로그/상태/이력을 Coordinator로 업로드한다.

## Acceptance Criteria

- [x] Agent 스크립트 골격(`agent/clw-agent.js`) 추가
- [x] `heartbeat` 커맨드로 `POST /v1/agents/heartbeat`
- [x] `hook completed|waiting` 래퍼로 (1) 기존 알림 실행 + (2) `POST /v1/events` 업로드(베스트 에포트)
- [x] `Claude-Code-Remote/setup.js`가 가능하면 agent 훅(`clw-agent.js hook ...`)을 설치하도록 변경(중복 훅 방지 포함)
- [x] Agent 프로세스가 주기적으로 `POST /v1/agents/heartbeat` (옵션: `node agent/clw-agent.js run`)
- [x] Telegram/LINE/Email 인바운드 명령 처리 결과(성공/실패)도 이벤트로 업로드(옵션: `COORDINATOR_URL` 설정 시)
- [x] Agent 식별자/이름/채널 구독(논리 채널) 개념을 최소로 추가(Agent ID 파일 + heartbeat meta `subscriptions`)
- [x] 기존 기능 기본 동작(회귀 없음) — 정적 스모크(`tasks/smoke/legacy-static-check.sh`) 기준

## Notes / References

- 기존 코드: `Claude-Code-Remote/` (회귀 금지)
- 구현 후보:
  - (A) `Claude-Code-Remote`에 “옵션”으로 Coordinator 업로드를 추가(환경변수로 enable)
  - (B) 별도 `agent/` 래퍼 프로세스가 `Claude-Code-Remote` 로그/훅을 구독해 업로드
- 결정해야 할 것:
  - Agent ID 생성/보관(고정 UUID vs 머신 고유값)
  - 이벤트 idempotency 전략
