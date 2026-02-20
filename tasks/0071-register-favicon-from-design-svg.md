# Register favicon from design SVG

## 요구사항
- `design/favicon/favicon_1.svg`를 웹 UI 파비콘으로 등록
- landing/dashboard/agent-auth 페이지에서 동일 파비콘 노출

## 작업 목록
- [x] `design/favicon/favicon_1.svg`를 UI 정적 파일 경로로 복사
- [x] UI HTML 헤더에 favicon link 태그 추가
- [x] SVG에서 PNG(16/32/180) 및 `favicon.ico` 파생 파일 생성
- [x] UI HTML에 `shortcut icon` + PNG + SVG + apple-touch 아이콘 동시 등록
- [x] `/dashboard`, `/agent-auth` URL을 HTML 파일로 직접 매핑
- [x] favicon link에 버전 쿼리 추가로 브라우저 캐시 무효화
- [x] 동작 확인(파일 존재/참조 경로 검증)

## 변경 파일
- `REQUIREMENTS.md`
- `tasks/0071-register-favicon-from-design-svg.md`
- `coordinator/internal/httpapi/ui/favicon_1.svg`
- `coordinator/internal/httpapi/ui/favicon.ico`
- `coordinator/internal/httpapi/ui/favicon-16x16.png`
- `coordinator/internal/httpapi/ui/favicon-32x32.png`
- `coordinator/internal/httpapi/ui/apple-touch-icon.png`
- `coordinator/internal/httpapi/ui/landing.html`
- `coordinator/internal/httpapi/ui/dashboard.html`
- `coordinator/internal/httpapi/ui/agent-auth.html`
- `coordinator/internal/httpapi/ui/landing-old.html`
- `coordinator/internal/httpapi/ui.go`
