# Chain Ownership 및 독점성 구현

## 요구사항
- REQUIREMENTS.md 참조: 4.4.1 Chain Ownership 및 독점성 (Exclusivity)

## 목표
Agent가 동시에 하나의 Chain만 소유하고, Chain을 소유한 Agent만 해당 Chain의 Task를 claim할 수 있도록 chain-level ownership과 exclusivity를 구현한다.

## 핵심 규칙
1. **Single chain ownership**: Agent는 동시에 하나의 Chain에 대해서만 소유권을 가질 수 있다
2. **Ownership 획득**: Agent가 Chain의 첫 번째 Task를 claim하면 해당 Chain의 소유권을 자동으로 획득한다
3. **Ownership 유지**: Agent가 Chain을 소유하는 동안 다른 Chain의 Task를 claim할 수 없다
4. **독점적 접근**: Chain을 소유한 Agent만 해당 Chain의 Task를 claim할 수 있다
5. **Ownership 해제**: Chain의 모든 Task가 완료/실패하면 소유권이 자동으로 해제된다

## 작업 목록

### 1. 데이터 모델 변경
- [x] chains 테이블에 `owner_agent_id` 컬럼 추가 (migration)
- [x] model.Chain에 `OwnerAgentID` 필드 추가
- [x] Agent에서 현재 소유한 chain 조회 로직 추가

### 2. claim_task 함수 수정 (Postgres)
- [x] Agent의 현재 chain ownership 체크 로직 추가 (migration에 구현)
- [x] Agent가 chain을 소유하면 그 chain의 task만 claim 가능하도록 제약
- [x] Chain의 첫 task를 claim할 때 chain.owner_agent_id 설정
- [x] 다른 agent가 소유한 chain의 task는 claim 불가

### 3. 메모리 스토어 구현
- [x] memory.go의 ClaimTask에 chain ownership 로직 추가
- [x] chain ownership 상태를 메모리에서 추적

### 4. Chain ownership 해제 로직
- [x] CompleteTask 시 chain의 모든 task 완료 여부 체크
- [x] FailTask 시 chain의 owner_agent_id 해제 여부 결정
- [x] chain 완료/실패 시 owner_agent_id를 NULL로 설정
- [x] Postgres: trigger를 통한 자동 해제
- [x] Memory: checkAndReleaseChainOwnership helper 함수

### 5. API 엔드포인트 추가 (선택사항)
- [ ] POST /v1/chains/detach - 수동 chain ownership 해제
- [ ] GET /v1/agents/:id/chain - Agent의 현재 소유 chain 조회

### 6. 테스트
- [ ] Postgres store chain ownership 테스트 추가 (TODO: postgres_test.go)
- [x] Memory store chain ownership 테스트 추가 (TestChainOwnership)
- [x] 여러 agent가 동시에 claim 시도 테스트 (TestChainOwnership)
- [x] Chain ownership 해제 테스트 (TestChainOwnership)

## 구현 완료 사항

### Migration (0012_chain_ownership.sql)
1. chains 테이블에 `owner_agent_id` 컬럼 추가
2. `claim_task()` 함수를 chain ownership 로직으로 재작성:
   - Agent가 이미 chain을 소유하면 그 chain의 task만 claim
   - Agent가 chain을 소유하지 않으면 소유되지 않은 chain의 첫 task claim 및 ownership 획득
3. `check_and_release_chain_ownership()` 헬퍼 함수 추가
4. Task 완료/실패 시 자동 ownership 해제 trigger

### Go 코드 변경
1. **model.Chain**: `OwnerAgentID` 필드 추가
2. **postgres.go**: CreateChain, GetChain, ListChains, UpdateChain에 owner_agent_id 처리 추가
3. **memory.go**:
   - ClaimTask: chain ownership 체크 로직 추가
   - checkAndReleaseChainOwnership: 헬퍼 함수 구현
   - CompleteTask/FailTask: ownership 해제 로직 호출
   - UpdateChain: OwnerAgentID 업데이트 지원
4. **memory_test.go**: TestChainOwnership 테스트 케이스 추가

## 최종 구현 상태

### ✅ 완료된 기능
1. **Chain Ownership 시스템**
   - Agent는 동시에 하나의 Chain만 소유
   - Chain 첫 task claim 시 자동 ownership 획득
   - 소유한 Chain의 task만 claim 가능
   - 다른 agent는 소유된 chain에 접근 불가

2. **Locked 상태 시스템**
   - Detach 시 chain과 in_progress task를 `locked` 상태로 전환
   - Locked task는 `queued` 또는 `done`으로만 전환 가능
   - Locked chain은 새로운 claim 차단

3. **수동 Detach API**
   - `POST /v1/chains/{id}/detach`
   - `POST /v1/tasks/{id}/status` (locked task 상태 변경)
   - `POST /v1/chains/{id}/assign-agent` (ownership 재할당)

4. **자동 해제 제거**
   - Task 완료/실패 시 ownership 유지
   - Chain status만 업데이트 (ownership 보존)

### 📊 테스트 커버리지
- ✅ TestChainOwnership: 기본 ownership 동작
- ✅ TestDetachAgentFromChain: Detach 및 locked 상태
- ✅ TestUpdateTaskStatus: Locked task 상태 전환
- ✅ TestClaimTaskBlockedByLockedTask: Locked chain claim 차단

### 🔧 검증 필요 사항
- [ ] Postgres migration 실행 및 동작 확인
- [ ] UI에서 chain owner_agent_id, locked 상태 표시 확인
- [ ] 실제 agent로 detach 시나리오 테스트

## 변경 파일
- `supabase/migrations/0012_chain_ownership.sql` (신규)
- `coordinator/internal/model/model.go`
- `coordinator/internal/store/postgres/postgres.go`
- `coordinator/internal/store/memory/memory.go`
- `coordinator/internal/store/postgres/postgres_test.go`
- `coordinator/internal/store/memory/memory_test.go`

## 설계 노트

### claim_task 함수 로직 (의사코드)
```sql
-- 1. Agent가 이미 소유한 chain이 있는지 확인
SELECT owner_chain_id FROM agents WHERE id = agent_id;

-- 2-a. 소유한 chain이 있으면, 그 chain의 다음 queued task만 claim 가능
IF owner_chain_id IS NOT NULL THEN
  SELECT task FROM tasks
  WHERE chain_id = owner_chain_id
    AND status = 'queued'
    AND sequence = (next sequential task)
  FOR UPDATE SKIP LOCKED;

-- 2-b. 소유한 chain이 없으면, queued chain의 첫 task를 claim하고 ownership 획득
ELSE
  SELECT task FROM tasks
  WHERE chain_id IN (SELECT id FROM chains WHERE status = 'queued' AND owner_agent_id IS NULL)
    AND sequence = 1
  FOR UPDATE SKIP LOCKED;

  UPDATE chains SET owner_agent_id = agent_id WHERE id = task.chain_id;
END IF;
```

### Ownership 해제 조건
- Chain의 모든 task가 `done` 상태일 때
- Chain의 어떤 task가 `failed` 상태일 때 (정책에 따라 다를 수 있음)
- Agent가 명시적으로 detach API를 호출할 때
