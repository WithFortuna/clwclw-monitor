# UI: 채널 보드 체인 접기/펼치기 + 체인 생성 팝오버

## 요구사항
- REQUIREMENTS.md 참조: 4.3 작업 태스크 보드
- 채널 헤더 보조 지표를 task 개수에서 chain 개수로 변경
- 체인 카드에서 태스크 보드를 접기/펼치기(toggle) 지원
- 채널 헤더 우측 `새 체인` 버튼 + popover 입력으로 체인 생성 지원
- 체인의 `Queued` 컬럼 헤더 우측 `+` 버튼 + popover 입력으로 task 생성 지원
- `Queued` popover에서 생성한 task는 해당 chain/channel 컨텍스트로 자동 등록

## 작업 목록
- [x] 대시보드 체인 렌더링 UI에 접기/펼치기 상태 추가
- [x] 채널 헤더 보조 지표를 chain 개수로 변경
- [x] 채널 헤더에 체인 생성 popover UI 추가
- [x] `/v1/chains` 호출로 체인 생성 이벤트/검증 처리
- [x] 체인 Queued 컬럼 헤더에 task 생성 popover UI 추가
- [x] `/v1/tasks` 호출 시 chain/channel 컨텍스트 자동 주입 처리
- [x] UI 스타일 반영 및 반응형 점검
- [ ] 변경 동작 수동 검증

## 변경 파일
- `REQUIREMENTS.md`
- `tasks/0060-ui-chain-collapse-and-create-popover.md`
- `coordinator/internal/httpapi/ui/app.js`
- `coordinator/internal/httpapi/ui/styles.css`
