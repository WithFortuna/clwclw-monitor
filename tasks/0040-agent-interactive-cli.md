# Agent Interactive CLI Flow

## 요구사항
- REQUIREMENTS.md 참조: 4.9 에이전트 인터랙티브 CLI 플로우

## 작업 목록
- [x] `agentLogin()` 수정 — login 성공 후 work 자동 진입 프롬프트
- [x] `fetchChannels()` 함수 추가 — GET /v1/channels API 호출
- [x] `promptForWorkConfig()` 함수 추가 — 채널 선택 + tmux 모드 설정
- [x] `startWorkLoop()` 함수 추출 — work 루프 재사용 가능하도록
- [x] `cmd === 'work'` 수정 — --channel 없으면 인터랙티브 폴백
- [x] `askYesNo()` 유틸 함수 추가

## 변경 파일
- `agent/clw-agent.js`
- `REQUIREMENTS.md`
