# 0015 — Trace Centralization (Artifacts / Links)

Status: **Todo**

## Goal

터미널 트레이스/실행 로그를 중앙에서 보관하고(DB 직접 저장 대신 오브젝트 스토리지 등) 대시보드에서 쉽게 조회할 수 있게 한다.

## Acceptance Criteria

- [ ] “트레이스 저장소”를 정의한다(S3/R2/GCS 등)와 보관 기간/비용 모델을 결정한다.
- [ ] Agent/Legacy가 트레이스를 업로드하고, Coordinator events에 참조 링크(artifact URL)를 기록한다.
- [ ] 대시보드에서 event payload에 포함된 artifact 링크를 클릭/프리뷰할 수 있다.
- [ ] 개인 정보/시크릿이 트레이스에 포함될 수 있으므로 마스킹/접근제어 가이드를 문서화한다.

## Notes / References

- Legacy trace 캡처: `Claude-Code-Remote/src/utils/trace-capture.js`, `Claude-Code-Remote/src/utils/tmux-monitor.js`
- Coordinator events: `coordinator/internal/httpapi/handlers.go`

