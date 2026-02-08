# Claude Code 실행 감지 개선

## 요구사항
- 관련 작업: `0024-dual-status-system.md` 개선
- Claude Code 프로세스 실행 여부를 tmux 화면에서 자동 감지

## 문제점
- 기존: Agent가 태스크 유무로만 상태 판단 (`idle`, `running`, `waiting`)
- Claude Code가 실제로 실행 중인지 감지하지 못함
- 사용자 요구: **"running"** 또는 **"not running"** 만 표시

## 해결 방안
- tmux 화면 캡처 후 Claude Code 실행 패턴 감지
- 감지되면 `running`, 아니면 `idle` (UI에서 "not running" 표시)
- `waiting` 상태 제거하여 단순화

## 작업 목록

### Agent (Node.js)
- [x] `detectClaudeCodeRunning()` 함수 추가
  - Claude Code 특징 패턴 매칭 (Claude Code, Anthropic, 모델명, 프롬프트 마커 등)
- [x] Work 루프: 태스크 실행 중 Claude Code 감지
- [x] Work 루프: 태스크 없을 때도 Claude Code 감지
- [x] Hook: `completed` 후 Claude Code 재감지
- [x] 상태 단순화: `running` | `idle` (waiting 제거)

### UI (이미 구현됨)
- [x] `app.js`: `claudeStatusBadge()` 함수에서 `idle` → "not running" 표시 변환
- [x] `styles.css`: `.badge.not-running` 스타일 (빨간색)

## 변경 파일
1. `agent/clw-agent.js`
   - Line 543-558: `detectClaudeCodeRunning()` 함수 추가
   - Line 1010-1050: Work 루프 태스크 실행 중 감지
   - Line 1102-1120: Work 루프 태스크 없을 때 감지
   - Line 823-842: Hook에서 감지

## 감지 방법 (기반: `util/find/find_claude_in_tmux.sh`)

### 재귀적 프로세스 트리 탐색
```javascript
// 1. 모든 프로세스 정보 읽기 (PID, PPID)
ps -ax -o pid,ppid

// 2. 프로세스 트리 구축 (Map<parentPid, [childPids]>)
buildProcessTree()

// 3. tmux pane의 PID에서 모든 자손(descendants) 재귀적으로 찾기
findAllDescendants(panePid, processTree)

// 4. 시스템의 모든 'claude' 프로세스 PID 찾기
ps -eo pid,comm | awk '$2 == "claude" {print $1}'

// 5. 자손 목록에 claude PID가 있는지 확인
descendants.includes(claudePid) → running
```

### 핵심 개념
- **Claude Code는 별도의 TTY와 PPID를 가짐** → 직접 자식이 아닌 **자손(descendants)** 확인 필요
- **재귀 탐색**: pane_pid → 모든 자식 → 자식의 자식 → ... → claude 프로세스 발견

### 장점
- ✅ 프로세스 트리 전체 탐색으로 정확한 감지
- ✅ 화면 텍스트와 무관
- ✅ Ctrl+C 즉시 감지
- ✅ 별도 프로세스 그룹의 claude도 감지

## 테스트
```bash
# 1. Coordinator 실행
go run ./coordinator/cmd/coordinator

# 2. Claude Code 실행 (tmux 내부)
tmux new -s claude-code
claude code

# 3. Worker 시작
COORDINATOR_URL=http://localhost:8080 \
node agent/clw-agent.js work --channel test --tmux-target claude-code:0.0

# 4. Dashboard 확인
open http://localhost:8080/
# → Claude 실행: "running" (초록), 종료: "not running" (빨강)
```

## 완료 일시
- 2026-02-07

## 관련 문서
- `tasks/0024-dual-status-system.md` - 기반 작업
- `CLAUDE.md` - 작업 규칙
