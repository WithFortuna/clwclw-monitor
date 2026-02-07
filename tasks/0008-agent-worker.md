# 0008 — Agent Worker (Claim → Inject → Complete)

Status: **Done**

## Goal

Coordinator의 `tasks`를 Agent가 FIFO로 claim하고, 로컬 Claude 세션(tmux)에 주입한 뒤, 완료 훅(`Stop`)과 연동해 `Done`까지 자동으로 마무리한다.

## Acceptance Criteria

- [x] `agent/clw-agent.js work --channel <name> --tmux-target <target>` 동작(폴링 기반 claim)
- [x] claim 성공 시 task를 tmux target으로 주입(`session` 또는 `session:window.pane`)
- [x] claim한 task를 `agent/data/current-task.json`에 기록(단일 in-flight 작업 가정)
- [x] `agent/clw-agent.js hook completed` 시 current-task가 있으면 `POST /v1/tasks/complete` 호출 후 상태 초기화
- [x] tmux pane/target 지정 지원(`--tmux-target`로 `session:window.pane` 가능)
- [x] 멀티 채널 워커 지원(`--channel "a,b,c"`로 순차 claim; 단일 in-flight 가정)

## Future (Post-MVP)

- 다중 동시 작업(멀티 in-flight) 지원
- 멀티 tmux target(세션/패널) 라우팅 정책 고도화

## Notes / References

- Coordinator claim API: `POST /v1/tasks/claim`
- Coordinator complete API: `POST /v1/tasks/complete`
