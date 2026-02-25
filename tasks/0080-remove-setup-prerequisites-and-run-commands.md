# Setup Guide: 사전 요구조건/기본 실행 명령 블록 제거

## 요구사항
- Setup Guide 화면에서 아래 항목을 제거한다.
  - `필수 사전 요구조건` 블록
  - `기본 실행 명령` 블록
- Setup Guide에는 `현재 지원 환경 스펙` 정보만 유지한다.

## 작업 목록
- [x] `REQUIREMENTS.md` Setup Guide 항목을 현재 정책(지원 환경 스펙만 표기)으로 수정
- [x] `coordinator/internal/httpapi/ui/dashboard.html`에서 `필수 사전 요구조건`/`기본 실행 명령` 섹션 제거
- [x] `coordinator/internal/httpapi/ui/styles.css`에서 제거된 UI의 미사용 스타일 정리
- [x] 작업 완료 후 체크리스트 상태 업데이트

## 변경 파일
- `REQUIREMENTS.md`
- `tasks/0080-remove-setup-prerequisites-and-run-commands.md`
- `coordinator/internal/httpapi/ui/dashboard.html`
- `coordinator/internal/httpapi/ui/styles.css`
