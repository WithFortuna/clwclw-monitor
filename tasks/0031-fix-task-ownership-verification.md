# Fix Task Ownership Verification

## 문제 상황

다른 agent가 소유한 task를 완료 처리할 수 있는 버그 발생:
- Agent A가 Task X를 claim
- Agent B가 작업 완료 후 hook 실행
- Task X가 Done으로 처리됨 (소유권 검증 실패)

## 근본 원인

Hook 실행 시 `configureStateDirForHook()`가 **잘못된 state directory**를 선택하면:
1. 다른 agent의 `current-task.json` 읽음
2. 다른 agent의 `agent-id.txt` 읽음 (같은 디렉토리)
3. `completeTask(other_task_id, other_agent_id)` 호출
4. Coordinator 검증: `req.AgentID == task.AssignedAgentID` → **통과!**

## 작업 목록

### 1. Coordinator 측 강화된 검증
- [x] CompleteTask에서 agent의 current_task_id 검증 추가
  - Agent가 실제로 해당 task를 진행 중인지 확인
  - `agent.CurrentTaskID != req.TaskID` → ErrConflict 반환
  - Memory store: memory.go:495-506
  - Postgres store: postgres.go:668-688

### 2. Agent 측 검증 강화
- [x] current-task.json에 agent_id 필드 추가
  - Task claim 시 자신의 agent ID 저장 (clw-agent.js:1413)
  - Hook 실행 시 stored agent ID와 current agent ID 비교 (clw-agent.js:1118-1120)
  - 불일치 시 complete 시도하지 않음

더이상 agent <-> tmux session <-> task 연결 정보는 로컬 .json이 아니라 db에서 관리됩니다
~### 3. State Directory 선택 로직 개선~
~- [ ] configureStateDirForHook() 로직 검토 (선택 사항)~
~- 현재 agent_id 검증으로 충분히 보호됨~
~- 필요 시 추가 개선 가능~

### 4. 테스트 케이스 추가
~- [ ] State directory 혼동 상황 재현 및 검증~
- [ ] 잘못된 소유권으로 complete 시도 시 에러 확인

## 변경 파일

- `coordinator/internal/store/memory/memory.go`
- `coordinator/internal/store/postgres/postgres.go`
- `agent/clw-agent.js`

## 우선순위

**High** - 다른 agent의 task를 잘못 완료하는 것은 심각한 데이터 무결성 문제
