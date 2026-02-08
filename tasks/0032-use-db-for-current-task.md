# Use DB for Current Task (Remove current-task.json)

## 요구사항

Local file (`current-task.json`) 대신 Coordinator DB를 사용하여 current task 관리:
1. Agent가 task claim → Coordinator의 `agents.current_task_id` 자동 업데이트 (이미 구현됨)
2. Hook 실행 시 Coordinator API로 current task 조회
3. Local state directory 의존성 제거

## 작업 목록

### 1. Coordinator API 추가
- [x] Store interface에 `GetAgent(ctx, id)` 추가
- [x] Memory store에 `GetAgent` 구현
- [x] Postgres store에 `GetAgent` 구현
- [x] `GET /v1/agents/:agent_id/current-task` 엔드포인트 추가
  - Agent의 current_task_id 조회
  - Task 정보 반환 (title, description, assigned_at 등)
  - 404 if no current task

### 2. Agent 측 수정
- [x] `getJson()` 함수 추가 (GET 요청용)
- [x] `readCurrentTask()` → `fetchCurrentTaskFromCoordinator()` 변경
  - Coordinator API 호출
  - current-task.json 읽기 제거
- [x] `writeCurrentTask()` 호출 제거
  - Coordinator가 ClaimTask 시 자동 업데이트
- [x] `clearCurrentTask()` 호출 제거
  - Coordinator가 CompleteTask/FailTask 시 자동 업데이트
- [x] Hook completed에서 `fetchCurrentTaskFromCoordinator()` 사용
- [x] Work 루프에서 `fetchCurrentTaskFromCoordinator()` 사용

### 3. 검증 로직 단순화
- [x] Hook의 tmux target 검증 제거 (불필요)
  - Coordinator의 current_task_id 검증으로 충분
  - Agent ID만 정확하면 안전

### 4. State Directory 역할 재정의
- [x] State directory는 agent ID 저장 용도로만 사용
  - `agent/data/instances/<target>/agent-id.txt`
  - current-task 관련 파일 제거 (함수는 남겨두되 사용하지 않음)

## 변경 파일

- `coordinator/internal/httpapi/routes.go`
- `coordinator/internal/httpapi/handlers.go`
- `agent/clw-agent.js`

## 장점

1. **단일 진실 공급원 (Single Source of Truth)**: Coordinator DB가 유일한 상태 저장소
2. **State directory 문제 해결**: Local file 혼동 문제 근본 제거
3. **단순화**: Local file sync 불필요, 검증 로직 단순화
4. **확장성**: 여러 machine에서 같은 agent ID 사용 가능 (future)

## 우선순위

**High** - 0031 버그 수정의 근본적 해결책
