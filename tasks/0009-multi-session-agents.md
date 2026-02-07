# 0009 — Multi-Session Agents (5 Claude Code Instances)

Status: **Done**

## Goal

로컬에서 Claude Code를 여러 개(예: 5개) 띄우는 경우에도,
각 세션을 Coordinator에서 **별도 Agent로 관측/태스크 완료 처리**할 수 있게 한다.

핵심은 “하나의 머신 = 하나의 agent id”가 아니라, **tmux target(세션/패널)** 단위로
agent identity + in-flight 상태를 분리하는 것이다.

## Acceptance Criteria

- [x] `agent/clw-agent.js`가 **tmux target 단위 state dir**를 자동 구성한다.
  - Agent ID 파일(`agent-id.txt`) 및 current task 파일이 target별로 분리되어 동시 실행 충돌이 없다.
- [x] `agent/clw-agent.js hook completed|waiting`이 현재 tmux target을 감지해 **올바른 state dir**를 사용한다.
- [x] 대시보드에서 동일 머신(동일 hostname)이라도 여러 agent가 구분 가능하다(이름/메타).
- [x] “tmux 없이 실행” 같은 케이스는 기존 behavior(단일 state dir)로 폴백한다.

## Notes

- 이 작업은 “레거시(Node) 기능 포팅”이 아니라, 현재 hybrid 구조에서 **멀티 세션 운영을 안정화**하기 위한 개선이다.
