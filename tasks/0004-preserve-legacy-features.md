# 0004 — Preserve Legacy Features (No Regression)

Status: **Done**

## Goal

`Claude-Code-Remote`가 제공하던 기능을 동일하게 제공하는지(회귀 없는지) 기준을 문서화하고 스모크 테스트 루틴을 만든다.

## Acceptance Criteria

- [x] 기능 목록을 체크리스트로 고정(Email/IMAP, Telegram, LINE, Desktop, tmux/pty injection, trace capture, whitelist/token)
- [x] “로컬에서 5개 세션” 같은 대표 시나리오를 재현 가능한 형태로 문서화
- [x] 최소 스모크 테스트 커맨드 정리(수동/반자동)
- [x] 실패 시 진단 루틴(로그 위치/환경변수/포트/권한)

---

## Legacy Feature Checklist (Source of Truth)

> 기준: `Claude-Code-Remote/README.md` + `CLAUDE_CODE_REMOTE_USECASES.md`

### A. 알림 채널(Outbound)

- [ ] Desktop 알림/사운드(완료/대기)
- [ ] Telegram 알림(토큰 포함, 버튼/명령 포맷 안내)
- [ ] LINE 알림(토큰 포함)
- [ ] Email 알림(토큰 포함, Reply-to-command, 실행 트레이스 포함 옵션)

### B. 인바운드 명령(Inbound) + 검증

- [ ] Telegram webhook 수신(`/webhook/telegram`)
- [ ] Telegram authorize(whitelist 또는 configured chat/group)
- [ ] LINE webhook 수신(`/webhook`)
- [ ] LINE 서명 검증(`x-line-signature`) + authorize(whitelist 또는 configured user/group)
- [ ] Email IMAP 리스닝 + 답장 판별 + 토큰/세션 검증
- [ ] Email 기본 위험 커맨드 필터(예: `rm -rf`, `sudo`, `curl|sh`)

### C. 주입(Injection)

- [ ] tmux 주입(`tmux send-keys`, 세션 유지)
- [ ] (옵션) tmux 무인 주입(`src/relay/tmux-injector.js`), 세션 없으면 생성될 수 있음
- [ ] pty 모드/OS 자동화 폴백(클립보드/AppleScript/알림)

### D. 세션/보안

- [ ] 토큰(8자리) 발급 + 만료(24h)
- [ ] 세션 파일 저장/조회(`src/data/sessions/*.json`, `src/data/session-map.json`)
- [ ] self-reply loop 방지(`sent-messages.json`)

### E. 실행 트레이스/이력

- [ ] tmux 대화 캡처(`tmux-monitor.js`)
- [ ] Email 실행 트레이스 포함 옵션(기본 on)
- [ ] SubagentStop 처리/요약 옵션

### F. 진단/테스트(레거시 제공)

- [ ] `test-telegram-notification.js`
- [ ] `test-real-notification.js`
- [ ] `test-injection.js`
- [ ] `test-complete-flow.sh`
- [ ] CLI 진단/상태: `claude-remote.js status|test|diagnose`

---

## Smoke Test Guide (Manual / Semi-automated)

> 주의: 이 레포는 외부 채널 API(Email/Telegram/LINE)를 사용하므로, 실 테스트는 실제 계정/토큰/웹훅 URL이 필요합니다.

### (선행) 정적 스모크(계정/네트워크 불필요)

```bash
bash tasks/smoke/legacy-static-check.sh
```

### 0) 준비

1) 레거시 의존성 설치
```bash
cd Claude-Code-Remote
npm install
```

2) 설정 생성
```bash
cd Claude-Code-Remote
npm run setup
```

3) 훅 설치 확인
- 기본은 `agent/clw-agent.js hook ...` 훅이 설치됨(레거시 알림 유지 + Coordinator 업로드)
- 강제로 레거시 훅만 쓰려면 `CLAUDE_REMOTE_HOOK_TARGET=legacy`로 `npm run setup` 실행

### 1) Outbound 알림 스모크

- 훅 스크립트 직접 호출(완료/대기)
```bash
node Claude-Code-Remote/claude-hook-notify.js completed
node Claude-Code-Remote/claude-hook-notify.js waiting
```

- Agent 래퍼 훅 호출(권장; 레거시 + 업로드)
```bash
node agent/clw-agent.js hook completed
node agent/clw-agent.js hook waiting
```

### 2) Telegram 인바운드 명령 스모크

1) webhook 서버 실행
```bash
cd Claude-Code-Remote
node start-telegram-webhook.js
```

2) Telegram에서 토큰 포함 명령 전송
- `/cmd TOKEN1234 your command`

기대 결과:
- authorize 통과(whitelist/configured chat)
- 토큰 유효(세션 파일 존재 + 만료 전)
- 지정 tmux 세션으로 주입 성공

### 3) LINE 인바운드 명령 스모크

1) webhook 서버 실행
```bash
cd Claude-Code-Remote
node start-line-webhook.js
```

2) LINE에서 메시지 전송
- `Token TOKEN1234 your command`

기대 결과:
- 서명 검증 통과
- authorize 통과
- 주입 성공

### 4) Email 답장 명령 스모크

- 데몬(권장)
```bash
cd Claude-Code-Remote
node claude-remote.js daemon start
node claude-remote.js daemon status
```

- relay-pty(통합형, 무인 주입 우선)
```bash
cd Claude-Code-Remote
node start-relay-pty.js
```

기대 결과:
- IMAP 수신
- 답장 판별 + 토큰 매칭
- 주입 성공(환경에 따라 tmux/자동화 경로)

### 5) 멀티 세션(5개 Claude) 스모크

권장 전제: “Claude 1개 = tmux 세션 1개”.

```bash
tmux new -s a
tmux new -s b
tmux new -s c
tmux new -s d
tmux new -s e
```

각 세션에서 Claude를 실행하고 훅을 켠 뒤:
- 각 세션에서 작업 실행 → 각기 다른 토큰/알림이 와야 함
- 각 토큰으로 reply 명령 → 해당 tmux 세션으로 주입되어야 함

---

## Failure Diagnostics (What to check)

### 공통

- `.env`가 올바른지: `Claude-Code-Remote/.env`
- 포트 충돌:
  - LINE 기본 3000 (`LINE_WEBHOOK_PORT`)
  - Telegram 기본 3001 (`TELEGRAM_WEBHOOK_PORT`)
- tmux 사용 시:
  - `INJECTION_MODE=tmux`
  - 대상 tmux 세션 존재 여부

### 로그/상태 파일

- Email daemon:
  - PID: `Claude-Code-Remote/src/data/claude-code-remote.pid`
  - Log: `Claude-Code-Remote/src/data/daemon.log`
- relay state:
  - `Claude-Code-Remote/src/data/relay-state.json`
- sessions:
  - `Claude-Code-Remote/src/data/sessions/*.json`
  - `Claude-Code-Remote/src/data/session-map.json`
- relay-pty:
  - PID: `Claude-Code-Remote/relay-pty.pid`
  - processed: `Claude-Code-Remote/src/data/processed-messages.json`

### 진단 커맨드

- CLI 상태/테스트
```bash
cd Claude-Code-Remote
node claude-remote.js status
node claude-remote.js test
node claude-remote.js diagnose
```

## Notes / References

- 기존 기능 레퍼런스: `CLAUDE_CODE_REMOTE_USECASES.md`
