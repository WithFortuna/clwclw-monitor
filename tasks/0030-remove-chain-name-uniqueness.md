# Remove Chain Name Uniqueness Constraint

## 요구사항
- Chain name과 task title은 중복 가능해야 함
- 같은 channel 내에서도 동일한 이름의 chain을 여러 개 만들 수 있어야 함

## 작업 목록
- [x] memory store의 CreateChain에서 unique 체크 제거
- [x] memory store의 UpdateChain에서 unique 체크 제거
- [x] postgres store의 CreateChain 확인 (DB constraint 없음)
- [x] postgres store의 UpdateChain 확인 (DB constraint 없음)
- [x] 코드 수정 완료 (사용자가 직접 테스트 필요)

## 변경 파일
- `coordinator/internal/store/memory/memory.go`
- `coordinator/internal/store/postgres/postgres.go`

## 관련 이슈
- 사용자 보고: "failed to create chain for task: conflict" 에러
- 원인: 같은 title로 여러 task 생성 시 chain name 충돌
