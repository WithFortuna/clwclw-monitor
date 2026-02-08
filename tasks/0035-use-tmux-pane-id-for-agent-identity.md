# Use Tmux Pane ID for Agent Identity

## 요구사항
- REQUIREMENTS.md 참조: 3.2 Multi-Session Support (멀티 세션 지원)

## 배경
현재 agent ID는 `agent/data/agent-id.txt` 파일로 관리되어 여러 tmux 세션에서 동시 실행 시 ID 충돌 발생.
사용자 제안: tmux pane ID (`$TMUX_PANE`)를 agent 식별자로 사용하면 각 세션을 자동으로 구분 가능.

## 현재 구현의 문제
1. 모든 tmux 세션이 같은 `agent-id.txt` 파일 공유
2. Hook 실행 시 어느 agent의 task인지 불명확
3. `AGENT_STATE_DIR`로 우회하지만 수동 설정 필요

## 제안된 해결책
tmux pane ID를 agent ID로 사용:
- Hook 실행: `$TMUX_PANE` 환경 변수 자동 전달
- Agent 등록: `{hostname}@{pane_id}` 형식으로 ID 생성
- Task claim/complete: pane ID로 정확한 agent 매칭

## 핵심 설계 원칙
**CRITICAL: `#S:#I.#P`는 연결 관계 식별에 절대 사용하지 않음**
- `#S:#I.#P` (session:window.pane): **변동값** (패널 재배치 시 변경)
- `%0`, `%1` (tmux pane ID): **고유 식별자** (패널 생명주기 동안 불변)

**연결 관계 흐름**:
1. `--tmux-target #S:#I.#P` 입력받음 (**최초 1회만**)
2. 즉시 `tmux display-message -t #S:#I.#P -p '#D'`로 pane ID 변환
3. 이후 모든 연결 관계는 **pane ID만 사용**
4. `#S:#I.#P`가 필요한 시점: `tmux list-panes -a -F '#{pane_id} #{session_name}:#{window_index}.#{pane_index}'`로 동적 조회

## 작업 목록
- [x] Phase 1: `--tmux-target` 입력 처리 변경
  - [x] `tmuxPaneIdForTarget(target)` 함수 확인 (이미 구현됨)
  - [x] `work` 명령 시작 시 `--tmux-target` → pane ID 즉시 변환
  - [x] pane ID를 primary로 사용 (`tmuxTarget`은 deprecated로 유지)
- [x] Phase 2: Heartbeat 메타데이터 수정
  - [x] `pane_id`를 primary 식별자로 사용 (메타데이터 필드명 변경)
  - [x] `tmux_target`, `tmux_session` 제거
  - [x] Agent meta에 `pane_id`만 저장
- [x] Phase 3: Hook에서 pane ID 우선 사용
  - [x] Hook 실행 시 `detectTmuxPaneId()` 우선 사용
  - [x] Heartbeat, event에 `pane_id` 전달
  - [x] `resolveTarget()` 함수는 유지 (이미 pane ID 우선)
- [x] Phase 4: `#S:#I.#P` 동적 조회 함수 추가
  - [x] `getPaneTarget(paneId)` 함수 구현
  - [x] 사용처: 로깅/디버깅 목적으로만
- [ ] Phase 5: 테스트
  - [ ] 2개 이상의 tmux pane에서 동시에 agent 실행
  - [ ] 각 pane이 독립적인 agent로 등록되는지 확인
  - [ ] Task claim/complete가 올바른 agent에 매칭되는지 확인
  - [ ] 패널 재배치 후에도 연결 유지되는지 확인

## 구현 상세

### Before (현재 방식)
```javascript
function getOrCreateAgentId() {
  const file = path.join(getStateDir(), 'agent-id.txt');
  if (fs.existsSync(file)) {
    return fs.readFileSync(file, 'utf-8').trim();
  }
  const id = crypto.randomUUID();
  fs.writeFileSync(file, id, 'utf-8');
  return id;
}
```

### After (제안된 방식 - UUID v5 사용)
```javascript
function getOrCreateAgentId() {
  const paneId = detectTmuxPaneId();
  if (paneId) {
    // Generate deterministic UUID from pane ID + hostname
    // UUID v5: namespace-based UUID (same input = same UUID)
    const hostname = os.hostname();
    const namespace = '6ba7b810-9dad-11d1-80b4-00c04fd430c8'; // DNS namespace (standard)
    const name = `clwclw-agent:${hostname}:${paneId}`;

    // Node.js crypto: generate UUID v5
    const hash = crypto.createHash('sha1');
    hash.update(Buffer.from(namespace.replace(/-/g, ''), 'hex'));
    hash.update(name);
    const digest = hash.digest();

    // Format as UUID v5
    digest[6] = (digest[6] & 0x0f) | 0x50; // Version 5
    digest[8] = (digest[8] & 0x3f) | 0x80; // Variant 10

    const uuid = [
      digest.slice(0, 4).toString('hex'),
      digest.slice(4, 6).toString('hex'),
      digest.slice(6, 8).toString('hex'),
      digest.slice(8, 10).toString('hex'),
      digest.slice(10, 16).toString('hex')
    ].join('-');

    return uuid;
  }

  // Fallback: use file-based UUID (non-tmux environments)
  const file = path.join(getStateDir(), 'agent-id.txt');
  if (fs.existsSync(file)) {
    return fs.readFileSync(file, 'utf-8').trim();
  }
  const id = crypto.randomUUID();
  fs.writeFileSync(file, id, 'utf-8');
  return id;
}
```

또는 더 간단하게 (if Node.js supports UUID v5 natively in future):
```javascript
function getOrCreateAgentId() {
  const paneId = detectTmuxPaneId();
  if (paneId) {
    const hostname = os.hostname();
    const namespace = crypto.UUID_NAMESPACE_DNS; // Future Node.js API
    return crypto.randomUUID({ version: 5, namespace, name: `clwclw:${hostname}:${paneId}` });
  }
  // ... fallback ...
}
```

## 장점
1. **자동 멀티 세션 지원**: 각 tmux pane이 자동으로 독립 agent로 작동
2. **명확한 식별**: Pane ID로 agent-task 매칭 명확
3. **설정 불필요**: `AGENT_STATE_DIR` 수동 설정 불필요
4. **디버깅 용이**: Pane ID로 agent 추적 쉬움

## 주의사항
1. **UUID 호환성**: `agents.id`는 UUID 타입으로 정의됨 (0001_init.sql:18)
   - 해결책 A: Pane ID를 UUID로 변환 (deterministic UUID v5 사용)
   - 해결책 B: Schema 변경 (agents.id를 TEXT로 변경) - 마이그레이션 필요
   - **권장**: 해결책 A (스키마 변경 없이 구현 가능)
2. **비 tmux 환경**: SSH 세션이나 로컬 실행에서는 fallback 필요
3. **Pane ID 재사용**: Tmux 재시작 시 pane ID 재사용 가능성 (큰 문제 아님)
4. **Deterministic UUID 생성**: 같은 pane ID는 항상 같은 UUID 생성 (일관성 보장)

## 변경 파일
- `agent/clw-agent.js`
  - `work` 명령: `--tmux-target` 입력 시 즉시 pane ID 변환 (line 1378-1410)
  - `heartbeat()` 호출: 모든 meta에서 `pane_id`만 전달 (line 1500-1538)
  - Hook 핸들러 (`cmd === 'hook'`): `detectTmuxPaneId()` 우선 사용 (line 1207-1318)
  - `getPaneTarget(paneId)` 함수 추가: pane ID → `#S:#I.#P` 변환 (디버깅용) (line 226-240)

## 주요 변경 사항
1. **연결 관계 식별자 변경**: `#S:#I.#P` → **pane ID** (`%0`, `%1`, ...)
2. **Heartbeat 메타데이터**: `tmux_target`, `tmux_session` 제거 → `pane_id`만 사용
3. **초기화 시점**: `--tmux-target` 입력 즉시 pane ID 변환
4. **Hook 실행**: `$TMUX_PANE` 환경 변수 → `detectTmuxPaneId()` 우선
5. **역방향 조회**: `getPaneTarget(paneId)` 함수로 필요 시 `#S:#I.#P` 조회 가능
