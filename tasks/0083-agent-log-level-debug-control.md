# Agent 로그 레벨 제어 (debug/info/warn/error)

## 요구사항
- `agent/clw-agent.js`에서 로그 레벨을 사용자가 설정할 수 있어야 한다.
- agentd 연결/등록/복구 및 hook 전달 관련 로그를 `debug` 레벨로 제어 가능해야 한다.

## 작업 목록
- [x] `REQUIREMENTS.md`에 agent 로그 레벨 제어 요구사항 추가
- [x] `tasks/INDEX.md`에 신규 작업 등록
- [x] `agent/clw-agent.js`에 로그 레벨 유틸 추가 (`AGENT_LOG_LEVEL`)
- [x] agentd 연결/등록 로그를 `debug` 레벨로 전환
- [x] 사용법(환경변수) 안내 메시지 반영
- [x] 문법 체크 수행 (`node --check agent/clw-agent.js`)
- [x] 작업 완료 후 체크리스트 업데이트

## 변경 파일
- `REQUIREMENTS.md`
- `tasks/INDEX.md`
- `tasks/0083-agent-log-level-debug-control.md`
- `agent/clw-agent.js`
