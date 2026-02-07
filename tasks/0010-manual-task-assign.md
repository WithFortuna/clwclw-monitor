# 0010 — Manual Task Assign (Specific Task → Agent)

Status: **Done**

## Goal

`REQUIREMENTS.md`의 “수동 할당 가능”을 충족하기 위해,
채널의 “다음 작업을 claim”하는 수준이 아니라 **특정 task를 특정 agent에 할당(assign)** 할 수 있게 한다.

## Acceptance Criteria

- [x] `POST /v1/tasks/assign` 구현
  - 입력: `task_id`, `agent_id` (optional `idempotency_key`)
  - 조건: task가 `queued`일 때만 `in_progress`로 전이되며 `assigned_agent_id/claimed_at`가 설정된다.
  - 충돌: 이미 `in_progress/done/failed`면 409.
- [x] in-memory / Postgres store 모두 지원
- [x] 대시보드에서 queued task에 “Assign” 버튼 제공(최소: agent_id prompt로도 OK)
