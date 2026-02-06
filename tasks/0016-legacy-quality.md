# 0016 — Legacy Quality Improvements (Non-functional)

Status: **Todo**

## Goal

레거시 `Claude-Code-Remote/`의 품질/정합성을 정리해 유지보수/회귀 방지를 쉽게 만든다(기능 변경 없이).

## Acceptance Criteria

- [ ] `Claude-Code-Remote/package.json` 스크립트/엔트리 포인트 불일치를 점검하고 문서/코드 중 하나로 정합성을 맞춘다.
- [ ] “현재 스냅샷에서 실제로 쓰이는 경로”를 기준으로 README/Quickstart를 최소 수정해 혼란을 줄인다.
- [ ] 테스트/진단 스크립트가 기본 Node 버전에서 동작하도록 문법/경로 문제를 정리한다.
- [ ] 레거시 변경이 Coordinator/agent 래퍼 동작에 영향이 없는지 스모크 체크로 보장한다.

## Notes / References

- 레거시 CLI: `Claude-Code-Remote/claude-remote.js`
- 레거시 문서: `Claude-Code-Remote/README.md`
- 스모크: `tasks/smoke/legacy-static-check.sh`

