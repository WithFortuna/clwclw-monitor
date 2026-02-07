# Task별 Claude Code 실행 모드 지정 기능

## 요구사항
- REQUIREMENTS.md 참조: 섹션 4.7 (태스크별 실행 모드 지정)
- REQUIREMENTS.md 참조: 섹션 6.3 (Tasks 데이터 모델)

## 개요
Task 생성 시 Claude Code의 실행 모드(permission mode)를 지정하고, 에이전트가 task를 claim할 때 자동으로 해당 모드로 전환하는 기능을 구현한다.

## 실행 모드 종류
1. **accept-edits**: 편집 자동 승인 모드 (Edit/Write tool call 자동 승인)
2. **plan-mode**: 계획 수립 모드 (코드 실행 없이 탐색/계획만)
3. **bypass-permission**: 모든 권한 자동 승인 (optional, 사용자 Claude Code 설정에 따라 활성화 여부 다름)

## 구현 전략

### 1. 모드 감지 (Mode Detection)
- **방법**: `tmux capture-pane -p -t <target>`로 현재 화면 캡처
- **파싱**: 화면 하단에 표시되는 모드 표시 텍스트 검색
  - "accept edits on" → `accept-edits` 모드
  - "plan mode on" → `plan-mode` 모드
  - "bypass permission on" → `bypass-permission` 모드
  - 모두 없으면 → `normal` 모드 (기본)

### 2. 모드 전환 (Mode Switching)
- **방법**: `Shift+Tab` 키를 이용한 순환 방식
- **순서**: `normal` → `accept-edits` → `plan-mode` → (`bypass-permission`) → `normal` ...
- **구현**:
  1. 현재 모드 감지
  2. 목표 모드까지의 단계 수 계산
  3. `Shift+Tab` 키를 N번 전송 (`tmux send-keys -t <target> S-Tab`)
  4. 모드 전환 후 재감지로 검증

### 3. 모드 전환 타이밍
- **Task claim 직후**: 에이전트가 task를 claim한 직후, 명령 주입 전에 모드 전환
- **전환 실패 처리**:
  - 최대 3회 재시도
  - 재시도 실패 시 task를 `failed` 상태로 전환
  - 에러 이벤트 생성 (type: `mode_switch_failed`)

## 작업 목록

### Phase 1: 데이터 모델 확장
- [x] DB 스키마 수정: `tasks` 테이블에 `execution_mode` 컬럼 추가 (varchar, nullable)
  - Migration: `supabase/migrations/0006_execution_mode.sql`
- [x] Go 모델 업데이트: `coordinator/internal/model/model.go`의 `Task` 구조체에 `ExecutionMode` 필드 추가
- [x] Store 인터페이스 업데이트: `CreateTask`, `ClaimTask` 등에서 `execution_mode` 처리
  - Memory store: `coordinator/internal/store/memory/memory.go` (자동 처리, 수정 불필요)
  - Postgres store: `coordinator/internal/store/postgres/postgres.go` (완료)

### Phase 2: API 확장
- [x] Task 생성 API: `POST /v1/tasks` 요청 body에 `execution_mode` 필드 추가
- [x] Task 조회 API: `GET /v1/tasks`, `GET /v1/tasks/:id` 응답에 `execution_mode` 포함 (자동 처리됨)
- [x] 대시보드 UI: Task 생성 폼에 실행 모드 선택 드롭다운 추가

### Phase 3: Agent 모드 감지 구현
- [x] `agent/clw-agent.js`에 `detectCurrentMode(tmuxTarget, paneId)` 함수 추가
  - `tmux capture-pane -p -t ${tmuxTarget}` 실행
  - 화면 텍스트에서 모드 키워드 검색
  - 감지된 모드 반환 (또는 `normal`)
- [x] 모드 감지 유틸리티 구현 완료

### Phase 4: Agent 모드 전환 구현
- [x] `agent/clw-agent.js`에 `switchToMode(tmuxTarget, targetMode, paneId, maxRetries)` 함수 추가
  - 현재 모드 감지
  - 목표 모드까지의 Shift+Tab 횟수 계산
  - `tmux send-keys -t ${tmuxTarget} S-Tab` N번 실행
  - 지연 후 모드 재감지로 전환 검증
  - 재시도 로직 (최대 9회, 기본값)
- [x] 모드 전환 실패 시 에러 처리
  - 에러 이벤트 생성
  - Task를 `failed` 상태로 전환

### Phase 5: Worker 통합
- [x] `failTask()` 함수 추가 (completeTask와 유사)
- [x] `agent/clw-agent.js`의 `work` 커맨드 수정
  - Task claim 직후, 명령 주입 전에 모드 전환 로직 추가
  - `task.execution_mode`가 지정된 경우에만 모드 전환 수행
  - 모드 전환 실패 시 task fail 처리 + 이벤트 생성 + current task 클리어

### Phase 6: 테스트 & 검증
- [ ] 각 실행 모드별 테스트 시나리오 작성
- [ ] 모드 전환 시나리오 테스트
  - normal → accept-edits
  - normal → plan-mode
  - normal → bypass-permission (if available)
- [ ] 모드 전환 실패 시나리오 테스트
- [ ] 로그/이벤트 타임라인 검증

### Phase 7: 문서화
- [ ] CLAUDE.md 업데이트: 실행 모드 기능 설명 추가
- [ ] API 문서 업데이트 (task 생성 예시)
- [ ] 에이전트 환경 변수 문서 업데이트 (필요시)

## 변경 파일
- `supabase/migrations/0006_execution_mode.sql` (신규)
- `coordinator/internal/model/model.go`
- `coordinator/internal/store/memory/memory.go`
- `coordinator/internal/store/postgres/postgres.go`
- `coordinator/internal/httpapi/handlers.go`
- `coordinator/internal/httpapi/ui/index.html`
- `agent/clw-agent.js`
- `CLAUDE.md`

## 기술적 고려사항

### tmux 화면 캡처 신뢰성
- Claude Code UI는 동적으로 변할 수 있으므로, 화면 하단 N줄만 캡처하는 것이 안정적
- 예: `tmux capture-pane -p -t <target> -S -5` (마지막 5줄만)

### 모드 순환 순서
- Bypass permission 모드는 optional이므로 감지 시 순환 순서를 동적으로 조정해야 함
- 사용자가 bypass permission을 비활성화한 경우 순환 순서에서 제외

### 타이밍 이슈
- `tmux send-keys` 후 Claude Code가 모드 전환을 완료하기까지 시간이 필요
- 모드 전환 후 100-200ms 대기 후 재감지 권장

### 에러 핸들링
- 모드 감지 실패 시: 기본 `normal` 모드로 가정
- 모드 전환 실패 시: task fail + 에러 이벤트 생성 + 알림 (기존 notification 채널 활용)

## 예상 사용 시나리오

### 시나리오 1: 계획 수립 task
```bash
# Task 생성 (API)
curl -X POST http://localhost:8080/v1/tasks \
  -H 'Content-Type: application/json' \
  -d '{
    "channel_id": "backend-domain",
    "title": "코드베이스 탐색 후 인증 기능 계획 수립",
    "description": "현재 인증 구조를 분석하고 JWT 기반 인증으로 전환하는 계획을 작성하세요",
    "execution_mode": "plan-mode"
  }'

# Agent가 claim하면:
# 1. 현재 모드 감지 (예: normal)
# 2. plan-mode로 전환 (Shift+Tab 2회)
# 3. Task description을 Claude Code에 주입
# 4. Claude Code는 plan mode에서 코드 실행 없이 탐색/계획만 수행
```

### 시나리오 2: 자동 편집 task
```bash
# Task 생성
curl -X POST http://localhost:8080/v1/tasks \
  -H 'Content-Type: application/json' \
  -d '{
    "channel_id": "backend-domain",
    "title": "User API 엔드포인트 CRUD 구현",
    "description": "User CRUD API를 구현하세요 (GET /users, POST /users, PUT /users/:id, DELETE /users/:id)",
    "execution_mode": "accept-edits"
  }'

# Agent가 claim하면:
# 1. accept-edits 모드로 전환
# 2. Claude Code가 Edit/Write tool call을 자동 승인
# 3. 빠른 구현 가능
```

### 시나리오 3: 모드 미지정 (기본 동작)
```bash
# execution_mode를 지정하지 않으면 현재 모드 유지
curl -X POST http://localhost:8080/v1/tasks \
  -H 'Content-Type: application/json' \
  -d '{
    "channel_id": "backend-domain",
    "title": "일반 작업",
    "description": "Task description",
    "execution_mode": null
  }'

# Agent는 모드 전환 없이 바로 명령 주입
```

## 참고사항
- 이 기능은 에이전트가 tmux 환경에서 동작할 때만 사용 가능
- Claude Code의 permission mode UI가 변경되면 모드 감지 로직도 업데이트 필요
- 향후 Claude Code가 모드 상태를 API나 파일로 노출하면 더 안정적인 구현 가능
