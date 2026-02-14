# Fix: Memory Store Chain Ownership Bypass After Completion

## 요구사항
- 에이전트는 최대 하나의 체인만 소유 가능
- 체인의 모든 태스크 완료 후에도 명시적 detach 전까지 소유권 유지
- Memory Store와 Postgres Store 동작 일치

## 작업 목록
- [x] Memory Store ClaimTask 소유권 체크에서 status 필터 제거
- [x] eligible task 필터링에서 소유 체인 done 상태 처리
- [x] reevaluateChainStatus에서 체인 완료 시 OwnerAgentID 자동 해제 제거
- [x] CreateTask에서 reevaluateChainStatus 호출 추가
- [x] 빌드 확인

## 변경 파일
- `coordinator/internal/store/memory/memory.go`
