# 0022 — Interactive Prompt Navigation (Arrow/Tab UI)

Status: **Done**

## Goal

Claude Code의 “선택지 UI(Enter/Tab/Arrow/Esc)”가 tmux pane에 표시될 때:

- 프롬프트/옵션/현재 선택 상태를 best-effort로 감지하고
- 대시보드에서 해당 태스크의 옵션을 사용자에게 보여주며
- 사용자가 선택하면 **키 입력(Up/Down/Tab/Enter/Escape)** 을 tmux pane에 주입해 선택을 진행한다.

## Background

기존 MVP(0021)는 숫자 입력(예: `1` + Enter) 중심이어서, 아래 형태의 UI에서 동작이 제한될 수 있다:

- “Enter to select · Tab/Arrow keys to navigate · Esc to cancel”
- 선택된 라인에 `>`/`❯` 등의 marker가 붙는 리스트

## Acceptance Criteria

- [x] Agent가 tmux capture 텍스트에서 아래를 best-effort로 판별한다:
  - [x] `Enter to select` / `Esc to cancel` 힌트 라인 유무(= navigation prompt)
  - [x] 옵션 목록(`1. ...`)과 현재 선택 marker(`>`/`❯` 등)
  - [x] payload에 `input_mode=arrows` 및 `selected_key`를 포함해 `task.prompt` 업로드
- [x] UI modal에서:
  - [x] 옵션 클릭 시 `arrows` 모드면 Up/Down + Enter로 이동/선택하도록 `kind=keys` 입력 생성
  - [x] Up/Down/Tab/Enter/Escape 버튼을 제공해 사용자가 수동으로 제어 가능
- [x] Agent(work)가 `kind=keys` 입력을 claim하면 tmux로 키 시퀀스를 주입한다(예: `Down Down Enter`).
- [x] 스모크 체크 통과: `bash tasks/smoke/legacy-static-check.sh`.

## Notes

- “감지”는 best-effort이며, tmux capture 기반이라 100% 보장을 목표로 하지 않는다.
- 키 주입은 기존 태스크의 대화 세션을 유지한 채 같은 tmux target으로 전달된다.
