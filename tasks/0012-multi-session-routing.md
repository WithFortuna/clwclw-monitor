# 0012 — Multi-Session Routing (Token → tmux target/pane)

Status: **Done**

## Goal

로컬에서 Claude Code 세션을 여러 개(예: 5개) 동시에 띄워도, 알림에 포함된 토큰으로 들어온 원격 명령이 **항상 올바른 tmux pane(대화 세션)** 으로 주입되게 한다.

## Acceptance Criteria

- [x] 알림 생성 시점에 tmux 정보를 `tmuxSession` 뿐 아니라 `tmuxTarget`(예: `session:window.pane`)까지 캡처해 세션 레코드에 저장한다.
- [x] Telegram/LINE/Email 인바운드 명령 주입은 `tmuxTarget`을 우선 사용하고, 없을 때만 `tmuxSession`으로 폴백한다(하위 호환).
- [x] `controller-injector`/tmux 주입 유틸이 `tmux send-keys -t <target>` 형태로 pane 단위 주입을 지원한다.
- [x] tmux-monitor/trace 캡처가 가능하면 pane 단위(`-t <target>`) 캡처를 지원한다(하위 호환 포함).
- [x] 문서 갱신: `USECASES.md`에 멀티 세션 라우팅 유스케이스를 추가/수정한다.
- [x] 스모크 체크 통과: `bash tasks/smoke/legacy-static-check.sh`.

## Notes / References

- 레거시 세션 파일: `Claude-Code-Remote/src/data/sessions/*.json`
- Telegram 인바운드: `Claude-Code-Remote/src/channels/telegram/webhook.js`
- LINE 인바운드: `Claude-Code-Remote/src/channels/line/webhook.js`
- Email 인바운드: `Claude-Code-Remote/src/relay/email-listener.js`, `Claude-Code-Remote/src/relay/relay-pty.js`
- 주입 스위치: `Claude-Code-Remote/src/utils/controller-injector.js`
- 기존 멀티 세션 분리(Agent 래퍼): `agent/clw-agent.js`
