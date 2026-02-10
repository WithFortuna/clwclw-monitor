# Agent IPC (Unix Domain Socket) — agentd

## 요구사항
- REQUIREMENTS.md 참조: 섹션 4.11 Agent IPC

## 작업 목록
- [x] 공유 UDS 유틸리티 (resolveSocketDir, sendMsg, createNdjsonParser 등)
- [x] agentd 서버 구현 (startAgentd)
- [x] main()에 agentd 커맨드 추가
- [x] 기존 hook coordinator 로직을 hookDirectCoordinator()로 추출
- [x] tryHookViaAgentd() 구현
- [x] hook 커맨드 수정 (agentd 경유 + fallback)
- [x] work 클라이언트 통합 (ensureAgentdRunning, connectToAgentd 등)
- [x] REQUIREMENTS.md 섹션 4.11 추가

## 변경 파일
- `agent/clw-agent.js`
- `REQUIREMENTS.md`
- `tasks/0043-agent-ipc-uds.md`
