# Agent 모드 분리 (local/prod) + Hook COORDINATOR_URL 해결

## 요구사항
- Hook에서 COORDINATOR_URL 미상속 문제 해결 (파일로 persist)
- Local/Prod 에이전트 분리 (데이터 경로 완전 분리)

## 작업 목록
- [x] getAgentMode() / saveAgentMode() 함수 추가
- [x] agentDataDir() 모드 기반 경로 변경
- [x] stateInstancesRoot() 모드 기반 경로 변경
- [x] getAgentToken() agentDataDir() 기반으로 변경
- [x] coordinatorBaseUrl() persist된 URL fallback 추가
- [x] Login 플로우에 모드 선택 프롬프트 추가
- [x] Worker 시작 시 URL persist
- [x] .gitignore 업데이트

## 변경 파일
- `agent/clw-agent.js`
- `.gitignore`
