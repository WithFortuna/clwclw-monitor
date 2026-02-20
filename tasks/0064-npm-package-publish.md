# @clwclw/agent npm 패키지 배포

## 요구사항
- clw-agent를 `@clwclw/agent`로 npm 배포
- Claude-Code-Remote를 bundledDependencies로 포함
- npm workspaces로 모노레포 관리

## 작업 목록
- [x] 루트 package.json 생성 (workspace root)
- [x] agent/package.json 생성 (@clwclw/agent)
- [x] clw-agent.js 수정: Claude-Code-Remote 경로 해석
- [x] clw-agent.js 수정: npm 설치 시 상태 디렉토리
- [x] clw-agent.js 수정: graceful degradation
- [x] chmod +x agent/clw-agent.js
- [x] npm install (workspace linking)

## 변경 파일
- `package.json` (루트, 신규)
- `agent/package.json` (신규)
- `agent/clw-agent.js`
