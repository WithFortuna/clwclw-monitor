# deploy 디렉터리 실행 기준 npm workspace publish 수정

## 요구사항
- `deploy/` 디렉터리에서 `./deploy.sh`를 실행해도 npm workspace publish가 정상 동작해야 한다.

## 작업 목록
- [x] `REQUIREMENTS.md`에 배포 스크립트 workspace 해석 요구 추가
- [x] `deploy/deploy.sh`의 npm publish 명령을 `deploy/` 실행 기준으로 수정
- [x] `deploy/` 기준 npm dry-run으로 동작 검증

## 변경 파일
- `REQUIREMENTS.md`
- `deploy/deploy.sh`
- `tasks/INDEX.md`
