# Cloudflare Tunnel Deployment

## 요구사항
- Coordinator를 Cloudflare Tunnel을 통해 외부에 노출
- Docker Compose로 coordinator + cloudflared 컨테이너 관리
- Agent는 호스트에서 실행 (tmux 접근 필요)

## 작업 목록
- [x] Coordinator Dockerfile 작성
- [x] docker-compose.yml 작성
- [x] .env.example 작성 (CLOUDFLARED_TOKEN 포함)
- [x] CLAUDE.md에 Docker 실행 가이드 추가
- [x] .dockerignore 작성

## 변경 파일
- `coordinator/Dockerfile` (신규)
- `docker-compose.yml` (신규)
- `.env.example` (신규)
- `.dockerignore` (신규)
- `CLAUDE.md` (수정 - Docker Deployment 섹션 추가)

## 참고사항
- Agent는 컨테이너화하지 않음 (로컬 tmux 세션 접근 필요)
- Cloudflare Tunnel 설정은 Cloudflare Zero Trust 대시보드에서 사전 설정 필요
- 토큰은 `cloudflare tunnel create <name>` 명령으로 생성
