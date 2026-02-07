# Fix tmux pane tracking with stable pane IDs

## 문제 (Issue #5)

`tmux split-window` 실행 시 pane 번호가 재배열되어, Agent가 엉뚱한 pane에 명령을 주입하는 문제:

```bash
# 초기: Claude Code가 pane 0에서 실행
tmux list-panes
# 0: ... %0  ← Claude Code

# split 후
tmux list-panes
# 0: ... %1  ← 새 pane
# 1: ... %0  ← Claude Code (번호가 1로 변경!)

# Agent는 여전히 :1.0 (현재는 새 pane)을 가리킴
# → 엉뚱한 곳에 명령 주입!
```

## 해결책

tmux의 **pane ID** (`%0`, `%1` 등)는 split해도 절대 변경되지 않으므로, pane 번호 대신 pane ID를 사용.

## 작업 목록

- [x] `detectTmuxPaneId()` 함수 추가 (`#D` 사용)
- [x] `tmuxPaneIdForTarget()` 함수 추가
- [x] `tmuxHasPaneId()` 유효성 검사 함수 추가
- [x] `isPaneId()`, `resolveTarget()` 헬퍼 함수 추가
- [x] `tmuxCapture()` pane ID 지원 추가
- [x] `tmuxInject()` pane ID 지원 추가
- [x] `tmuxSend()` pane ID 지원 추가
- [x] `tmuxSendKeys()` pane ID 지원 추가
- [x] Agent work 루프: pane ID 자동 감지
- [x] Task state에 `tmux_pane_id` 필드 추가
- [x] 모든 heartbeat/event에 pane ID 포함
- [ ] 테스트: split 후에도 올바른 pane에 명령 주입되는지 검증

## 변경 파일

- `agent/clw-agent.js` (약 200줄 수정)

## 테스트 방법

1. tmux 세션 시작 및 Coordinator 실행:
   ```bash
   tmux new-session -s claude-code
   go run ./coordinator/cmd/coordinator
   ```

2. 다른 pane에서 Agent 시작:
   ```bash
   node agent/clw-agent.js work --channel test --tmux-target claude-code:1.0
   # 로그 확인: "detected stable pane ID: %0"
   ```

3. 원본 pane에서 split:
   ```bash
   tmux split-window
   tmux list-panes
   # 0: ... %1 (새 pane)
   # 1: ... %0 (Claude Code - 번호 변경!)
   ```

4. Task 생성하여 주입 테스트:
   ```bash
   curl -X POST http://localhost:8080/v1/tasks \
     -H 'Content-Type: application/json' \
     -d '{"channel_id":"...","title":"test split","description":"echo split test"}'
   ```

5. **검증**: Claude Code pane (%0, 번호는 1)에만 명령이 주입되는지 확인

## 기술적 세부사항

### tmux pane 식별자

- **Pane 번호** (`#P`): 0, 1, 2, ... (split 시 재배열됨)
- **Pane ID** (`#D`): %0, %1, %2, ... (생성 시 할당, 절대 변경 안됨)

### 핵심 로직

```javascript
function resolveTarget(target, fallbackPaneId) {
  // pane ID 형식이면 그대로 사용
  if (isPaneId(target)) return target;

  // fallback pane ID가 유효하면 사용
  if (fallbackPaneId && isPaneId(fallbackPaneId) && tmuxHasPaneId(fallbackPaneId)) {
    return fallbackPaneId;
  }

  // 아니면 target (하위 호환)
  return target;
}
```

### 상태 저장

```json
{
  "task_id": "...",
  "tmux_session": "claude-code",
  "tmux_target": "claude-code:1.0",
  "tmux_pane_id": "%0",  // ← 핵심: 안정적인 식별자
  "claimed_at": "..."
}
```

## 참고

- tmux man page: `display-message -p '#D'`
- Issue: #5 (@WithFortuna)
