# Setup Guide: 필수 사전 요구조건 및 지원 환경 스펙 명시

## 요구사항
- 현재 서비스 실행 필수 조건인 macOS, tmux, Claude Code를 화면에서 명확히 안내
- 서비스 실행에 필요한 기본 실행 정보(로그인/worker 실행 명령)를 Setup Guide에서 확인 가능해야 함

## 작업 목록
- [x] `REQUIREMENTS.md`에 Setup Guide 환경 요구사항/실행 정보 표기 요구 반영
- [x] `coordinator/internal/httpapi/ui/dashboard.html` Setup Guide에 사전 요구조건, 지원 환경, 실행 명령 섹션 추가
- [x] `coordinator/internal/httpapi/ui/styles.css` 신규 섹션 스타일 추가 (desktop/mobile 대응)
- [x] 작업 완료 후 체크리스트 상태 업데이트

## 변경 파일
- `REQUIREMENTS.md`
- `tasks/0078-setup-prerequisites-and-supported-env-spec.md`
- `coordinator/internal/httpapi/ui/dashboard.html`
- `coordinator/internal/httpapi/ui/styles.css`
