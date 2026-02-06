# 0021 — Interactive Prompts (Detect → Show → Respond)

Status: **Done**

## Goal

Claude Code가 “선택지(1/2, y/n, Enter 등)”를 요구하는 **인터랙티브 프롬프트** 상태에 들어가면 이를 감지해 대시보드에서 해당 태스크에 옵션을 보여주고, 사용자가 선택/입력하면 같은 tmux target으로 주입되게 한다.

## Acceptance Criteria

- [x] Agent(work 모드)가 in-flight task의 tmux pane output을 주기적으로 캡처해 “인터랙티브 프롬프트”를 best-effort로 감지한다.
- [x] 감지 시 Coordinator events에 `task.prompt` 이벤트를 `task_id`와 함께 업로드하고, 중복 감지는 idempotency로 억제한다.
- [x] 대시보드는 `in_progress` 태스크에서 프롬프트가 감지되면 해당 태스크에서 옵션/입력을 표시한다.
- [x] 사용자가 선택/입력하면 Coordinator에 “태스크 입력”을 생성하고, Agent가 이를 claim/poll해서 tmux target에 주입한다.
- [x] 주입 성공/실패를 event로 남겨서 대시보드 타임라인에서 확인 가능하다.
- [x] 스모크 체크 통과: `bash tasks/smoke/legacy-static-check.sh`.

## Output

- Backend: `/v1/tasks/inputs`, `/v1/tasks/inputs/claim` (Coordinator)
- UI: in-progress task에 `Prompt…` 버튼 + modal 입력/옵션 전송
- Agent(worker): prompt 감지 + 입력 claim/inject + 이벤트 업로드

## Notes / References

- 훅 설치(Stop/SubagentStop): `Claude-Code-Remote/setup.js`
- 레거시 자동 confirm 패턴(참고): `Claude-Code-Remote/src/relay/tmux-injector.js`
- 워커: `agent/clw-agent.js`
- 대시보드 UI: `coordinator/internal/httpapi/ui/*`
