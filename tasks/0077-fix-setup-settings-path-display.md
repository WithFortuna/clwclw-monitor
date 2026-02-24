# Fix: setup 질문의 outdated settings 경로 수정

## 요구사항
- commit 88b10b6 이후 실제 쓰기 경로는 `<cwd>/.claude/settings.local.json`
- 사용자에게 보여주는 질문 문구가 여전히 `~/.claude/settings.json`(옛날 경로)를 표시

## 작업 목록
- [x] `setup-i18n.json`의 `updateHooks` 문자열에 `{settingsPath}` placeholder 적용
- [x] `setup.js` line 516에서 실제 경로를 계산하여 placeholder 치환

## 변경 파일
- `Claude-Code-Remote/setup-i18n.json`
- `Claude-Code-Remote/setup.js`
