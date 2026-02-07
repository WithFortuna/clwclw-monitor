# Dual Status System Implementation

## 요구사항
- Issue: [#1 - [fix] 클로드 코드 에이전트 상태관리](https://github.com/WithFortuna/clwclw-monitor/issues/1)
- REQUIREMENTS.md 참조: Agent Status System

## 문제점
- `Agent.Status` 필드가 두 가지 개념을 혼재:
  1. Worker 프로세스 라이프사이클 (실행 중/중단)
  2. Claude Code 실행 상태 (idle/running/waiting)
- Tmux 패널만 열려있을 때 status가 "running"으로 잘못 표시됨

## 해결 방안
- **Worker Status** (computed): `last_seen` 타임스탬프로 계산 (online/offline)
- **Claude Status** (reported): Agent가 heartbeat로 보고 (idle/running/waiting)

## 작업 목록

### Backend (Go)
- [x] Model: `ClaudeStatus`, `WorkerStatus` 타입 추가
- [x] Database Migration: `claude_status` 컬럼 추가 (`0003_dual_status.sql`)
- [x] Memory Store: 자동 상태 업데이트 제거, `claude_status` 처리
- [x] Postgres Store: `claude_status` 컬럼 지원
- [x] API Handlers: Heartbeat에 `claude_status` 추가, agents list에 `worker_status` 계산
- [x] Dashboard API: 두 상태 모두 반환

### Agent (Node.js)
- [x] Heartbeat: `claude_status` 필드 전송 (backward compatibility 유지)

### Dashboard (Frontend)
- [x] UI Logic: 두 상태 렌더링 함수 추가
- [x] HTML: 테이블 헤더 업데이트 (Worker | Claude)
- [x] CSS: Badge 스타일 추가 (`.badge.err`, `.badge.muted-badge`)

## 변경 파일

### Backend
1. `coordinator/internal/model/model.go` - Status types
2. `coordinator/internal/store/memory/memory.go` - Memory store
3. `coordinator/internal/store/postgres/postgres.go` - Postgres store
4. `coordinator/internal/httpapi/handlers.go` - API handlers
5. `coordinator/internal/httpapi/dashboard.go` - Dashboard API
6. `supabase/migrations/0003_dual_status.sql` - DB migration (NEW)

### Agent
7. `agent/clw-agent.js` - Heartbeat payload

### Dashboard
8. `coordinator/internal/httpapi/ui/app.js` - UI rendering
9. `coordinator/internal/httpapi/ui/index.html` - HTML structure
10. `coordinator/internal/httpapi/ui/styles.css` - CSS styling

## 핵심 변경사항

### 상태 분리
- **이전**: 단일 `status` 필드 → Worker 라이프사이클과 Claude 실행 상태 혼재
- **이후**:
  - `worker_status` (computed): `last_seen`에서 계산 (30초 threshold)
  - `claude_status` (reported): Agent가 heartbeat로 전송

### 자동 업데이트 제거
- Task claim/complete/fail 시 agent status 자동 업데이트 제거
- Agent heartbeat가 유일한 source of truth

### Backward Compatibility
- 기존 `status` 필드 유지 (deprecated)
- 구 버전 agent는 `status`만 전송 → `claude_status`로 매핑
- 신 버전 agent는 둘 다 전송

## 테스트 시나리오

1. **Worker online, no task**: Worker=`online`, Claude=`idle`
2. **Worker online, task executing**: Worker=`online`, Claude=`running`
3. **Worker offline**: Worker=`offline` (last_seen > 30s)
4. **Task completion**: Claude returns to `idle`

## 완료 일시
- 2026-02-06

## 관련 문서
- `REQUIREMENTS.md` - Agent Status System 섹션
- `CLAUDE.md` - 작업 규칙 및 프로젝트 가이드
- `RUNBOOK.md` - 테스트 절차
