# Setup: CWD 기준으로 settings.local.json 작성

## 요구사항
- `clw-agent setup`을 실행한 디렉토리의 `.claude/settings.local.json`에 훅 작성
- `.claude/` 없으면 에러 메시지 출력 후 종료 (디렉토리 생성 안 함)

## 작업 목록
- [x] `agent/clw-agent.js`: `.claude/` 존재 확인 사전 검사 추가, spawn cwd를 `process.cwd()`로 변경
- [x] `Claude-Code-Remote/setup.js`: `ensureHooksFile()`에서 `__dirname` 기반 → `process.cwd()` 기반으로 변경, `mkdirSync` 제거

## 변경 파일
- `agent/clw-agent.js`
- `Claude-Code-Remote/setup.js`
