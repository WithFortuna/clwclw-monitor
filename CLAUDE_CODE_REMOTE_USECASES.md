# Claude-Code-Remote 동작 흐름 / 유스케이스별 구현 정리 (모듈 수준)

이 문서는 이 워크스페이스에 포함된 `Claude-Code-Remote/` 코드를 기준으로, **어떤 이벤트(훅/웹훅/메일)가 어디로 들어오고**, **사용자 검증은 어떻게 하고**, **명령 주입 시 기존 대화 세션을 유지하는지/새로 여는지** 같은 “기술적 결정사항”을 **유스케이스(Use Case) 단위**로 구체화해서 정리한 것입니다.

> 기준 코드 루트: `Claude-Code-Remote/`

---

## 0) 용어 정리 (이 문서에서 쓰는 표현)

- **Claude 훅(Claude Code hooks)**: Claude Code가 특정 이벤트(`Stop`, `SubagentStop`)에 실행하는 “로컬 커맨드 훅”.
  - 설정/설치 위치: `~/.claude/settings.json` 또는 프로젝트별 `CLAUDE_HOOKS_CONFIG`.
  - 이 레포에서 훅 커맨드로 호출되는 스크립트: `Claude-Code-Remote/claude-hook-notify.js`.
- **외부 채널 웹훅(External webhook)**: Telegram/LINE 같은 플랫폼이 우리 서버(Express)에 HTTP 요청으로 “메시지 이벤트”를 전달하는 방식.
  - Telegram: `Claude-Code-Remote/src/channels/telegram/webhook.js`(Express) + `start-telegram-webhook.js`
  - LINE: `Claude-Code-Remote/src/channels/line/webhook.js`(Express) + `start-line-webhook.js`
- **이메일 인바운드(Email inbound)**: 외부에서 웹훅을 보내는 게 아니라, 우리가 IMAP으로 주기적으로 inbox를 읽어 “답장”을 커맨드로 해석하는 방식.
  - 구현이 2갈래로 존재:
    1) 데몬 기반: `src/daemon/taskping-daemon.js` + `src/relay/email-listener.js` + `src/relay/command-relay.js`
    2) relay-pty 기반: `src/relay/relay-pty.js`(IMAP+원격 주입을 통합)
- **세션/토큰(Token)**: 알림 메시지에 포함되는 8자리 토큰. 사용자가 이 토큰을 포함해 명령을 보내면, 시스템이 토큰을 기준으로 “어느 세션에 주입할지”를 결정.
  - Telegram/LINE은 주로 `src/data/sessions/*.json`에서 토큰→세션 매핑.
  - Email(특히 relay-pty)은 `src/data/session-map.json`에서 토큰→세션 매핑도 사용.
- **주입(Injection)**: 원격에서 받은 커맨드를 로컬 Claude Code에 전달하는 방식.
  - tmux 모드: `tmux send-keys`로 **기존 tmux 세션**에 타이핑/엔터를 “주입”.
  - pty 모드: (일부 코드 경로에서) PTY 파일로 write 하려는 설계가 있으나, 이 레포 스냅샷에서는 실제 운용은 tmux/OS 자동화 경로가 중심.
  - OS 자동화 폴백: AppleScript/클립보드/알림 등으로 “사용자에게 붙여넣기”를 유도하거나 자동 붙여넣기 시도.

---

## 1) 컴포넌트/모듈 경계 (한 장 요약)

### 1.1 엔트리 포인트(실행 파일)

- Claude 훅 트리거: `Claude-Code-Remote/claude-hook-notify.js`
- CLI: `Claude-Code-Remote/claude-remote.js`
- 웹훅 서버 실행:
  - `Claude-Code-Remote/start-telegram-webhook.js`
  - `Claude-Code-Remote/start-line-webhook.js`
  - `Claude-Code-Remote/start-all-webhooks.js`(여러 개를 병렬 실행)
- 이메일 relay-pty 실행: `Claude-Code-Remote/start-relay-pty.js` → `Claude-Code-Remote/src/relay/relay-pty.js`
- (옵션) tmux 출력 기반 “미스 방지/스마트 감지”: `Claude-Code-Remote/smart-monitor.js`

### 1.2 Outbound(알림 전송) 채널 모듈

- Desktop 알림/사운드: `Claude-Code-Remote/src/channels/local/desktop.js`
- Telegram 알림: `Claude-Code-Remote/src/channels/telegram/telegram.js` (Bot API `sendMessage`)
- LINE 알림: `Claude-Code-Remote/src/channels/line/line.js` (Messaging API `push`)
- Email 알림: `Claude-Code-Remote/src/channels/email/smtp.js` (SMTP send + 세션/토큰 생성)

### 1.3 Inbound(명령 수신) 모듈

- Telegram 웹훅 인바운드: `Claude-Code-Remote/src/channels/telegram/webhook.js`
- LINE 웹훅 인바운드: `Claude-Code-Remote/src/channels/line/webhook.js`
- Email 인바운드(데몬): `Claude-Code-Remote/src/relay/email-listener.js`
- Email 인바운드(통합형): `Claude-Code-Remote/src/relay/relay-pty.js`

### 1.4 주입(Injection) 모듈

- 범용 주입 스위치: `Claude-Code-Remote/src/utils/controller-injector.js`
  - `INJECTION_MODE=tmux|pty`에 따라 `tmux send-keys` 또는 PTY write 시도
- tmux 주입(원격/무인 주입 강화): `Claude-Code-Remote/src/relay/tmux-injector.js`
  - 특징: 세션이 없으면 **새로 만들 수도 있음**(= 새 대화 세션이 생길 수 있는 경로)
- 스마트 폴백(자동화): `Claude-Code-Remote/src/relay/smart-injector.js` + `src/automation/*`

### 1.5 세션/토큰 저장소(로컬 파일)

- 세션 상세(토큰/만료/세션명 등): `Claude-Code-Remote/src/data/sessions/*.json`
  - Telegram/LINE 웹훅에서 토큰으로 세션을 찾을 때 여기 파일들을 스캔
- 토큰→세션 요약 맵(주로 이메일/relay-pty용): `Claude-Code-Remote/src/data/session-map.json`
- 기타 중복처리/로그:
  - 이메일 self-reply loop 방지: `Claude-Code-Remote/src/data/sent-messages.json`
  - relay-pty 중복처리: `Claude-Code-Remote/src/data/processed-messages.json`

---

## 2) 핵심 “기술적 결정사항” 요약 (질문 포인트 중심)

### 2.1 “텔레그램 같은 외부 채널로부터 이벤트 훅을 받은 건지?”

- **Telegram/LINE에서 ‘명령’을 받는 경로는 “외부 채널 → 우리 서버(웹훅)”가 맞습니다.**
  - Telegram: `start-telegram-webhook.js`가 Express 서버를 띄우고, Telegram이 `/webhook/telegram`으로 이벤트를 POST.
  - LINE: `start-line-webhook.js`가 Express 서버를 띄우고, LINE이 `/webhook`으로 이벤트를 POST(서명 포함).
- 반대로, **Claude 작업 완료 감지**는 외부 채널 웹훅이 아니라 **Claude Code의 로컬 훅**으로 시작합니다.
  - Claude Code `Stop/SubagentStop` → 로컬 커맨드 훅 실행 → `claude-hook-notify.js` → Telegram/LINE/Email로 “Outbound 알림”.

즉, “외부 채널 웹훅”은 **인바운드(사용자 명령)** 용이고, “Claude Code hooks”는 **완료/대기 알림 트리거(Outbound)** 용입니다.

### 2.2 “사용자 명령을 받았으면 올바른 사용자 검증은 사전 목록(whitelist)인가?”

채널별 인증/검증은 다음처럼 “사전 설정” 중심입니다.

- **Telegram**
  - 환경변수 `TELEGRAM_WHITELIST`(쉼표 구분)가 있으면: chatId 또는 userId가 whitelist에 포함될 때만 허용 (`src/channels/telegram/webhook.js`의 `_isAuthorized`)
  - whitelist가 비어 있으면: `.env`에 설정된 `TELEGRAM_CHAT_ID` 또는 `TELEGRAM_GROUP_ID`와 일치할 때만 허용
  - 추가로: 토큰(`8자리`)이 유효(세션 파일 존재 + 만료 전)해야 주입 수행
  - 참고: 이 코드 스냅샷에서는 “Telegram이 보낸 요청임을 서명으로 검증”하는 로직은 별도로 없고, **(1) 엔드포인트 노출 + (2) whitelist**를 중심으로 방어합니다.
- **LINE**
  - 1차: `x-line-signature`를 `LINE_CHANNEL_SECRET`으로 검증 (`src/channels/line/webhook.js`의 `_validateSignature`)
  - 2차: `LINE_WHITELIST` 또는 `.env`의 `LINE_USER_ID`/`LINE_GROUP_ID` 기반 authorize
  - 3차: 토큰 유효성(세션 파일 존재 + 만료 전)
- **Email**
  - 1차: 발신자 whitelist(`ALLOWED_SENDERS`) 검사(데몬/relay-pty 흐름에 따라 구현이 조금 다름)
  - 2차: 토큰 기반 세션 매칭(Subject의 `[Claude-Code-Remote #TOKEN]` 등)
  - 3차: 세션 만료/명령횟수 제한(`commandCount/maxCommands`) 등(데몬 흐름에서 특히 명시)
  - 4차: 위험 커맨드 간단 필터(`rm -rf`, `sudo`, `curl | sh` 등)

### 2.3 “명령 주입은 기존 대화 세션을 유지하나, 새 대화 세션을 여나?”

결론부터:

- **tmux 주입 경로는 기본적으로 “기존 tmux 세션(= 기존 Claude 대화/컨텍스트)”에 send-keys 하는 구조**라서, *해당 tmux 세션이 살아있다면* **대화 세션을 유지**합니다.
- 다만 **`src/relay/tmux-injector.js`를 사용하는 일부 경로(특히 relay-pty)**는 “tmux 세션이 없으면 새로 만들기(create)”를 수행할 수 있어, 그 경우 **새 대화 세션이 열릴 수 있습니다.**

정리:

- Telegram/LINE 웹훅 → `ControllerInjector` → `tmux send-keys`(세션 없으면 에러) ⇒ **세션 유지(단, 세션이 존재해야 함)**
- relay-pty → `TmuxInjector.injectCommandFull()` → (세션 없으면 create) ⇒ **세션 유지 또는 새 세션 생성**
- Email 데몬(`command-relay.js`)의 자동화 경로는 “현재 열려 있는 Claude Code/Terminal”에 붙여넣기/타이핑을 시도 ⇒ **어느 창에 주입되느냐에 따라 세션 유지 여부가 달라질 수 있음**(명시적으로 새 세션을 생성하진 않음)

---

## 3) 유스케이스별 상세 구현 (Use Case + 기술 결정 중심)

### UC-01. 초기 설정/설치(훅 설치 + .env 구성)

**목적**
- Claude Code가 작업이 끝날 때마다(Stop/SubagentStop) 로컬 커맨드를 호출하도록 훅을 설치하고,
- Telegram/LINE/Email 등 채널 설정을 `.env`로 준비.

**주요 실행**
- `Claude-Code-Remote/setup.js` (추천)

**동작(요약)**
1) `setup.js`가 `.env`를 생성/갱신
2) `setup.js`가 `~/.claude/settings.json`에 아래 형태로 훅을 머지(업서트)
   - Stop → `node <...>/claude-hook-notify.js completed`
   - SubagentStop → `node <...>/claude-hook-notify.js waiting`

**기술적 결정**
- “완료 감지”는 외부 웹훅이 아닌 **Claude 로컬 훅**에 위임.
- 훅이 실행할 커맨드는 **프로세스 외부 스크립트(Node)** 이므로:
  - Claude Code가 실행되는 환경(특히 tmux 여부, working dir, env vars)이 알림 내용/세션 매핑에 영향을 줌.

---

### UC-02. 서비스 실행(웹훅 서버/데몬 기동)

**목적**
- 인바운드(사용자 명령)를 받기 위한 서버를 띄우고, 필요한 경우 이메일 데몬도 가동.

**주요 실행**
- `Claude-Code-Remote/start-all-webhooks.js`
  - Telegram: `start-telegram-webhook.js` (Express)
  - LINE: `start-line-webhook.js` (Express)
  - Email daemon: `claude-remote.js daemon start` → `src/daemon/taskping-daemon.js`

**기술적 결정**
- Telegram/LINE은 HTTP 웹훅 기반(실시간, 인바운드 이벤트 푸시)
- Email은 IMAP 폴링 기반(주기적 체크, 인바운드 이벤트 Pull)

---

### UC-03. 작업 완료/대기 감지 → 알림 전송(Claude 훅 기반, 핵심)

**목적**
- Claude Code가 작업을 마치거나 입력이 필요할 때 사용자에게 외부 채널로 알림 전송.

**트리거**
- Claude Code 훅: Stop/SubagentStop → `node Claude-Code-Remote/claude-hook-notify.js completed|waiting`

**주요 모듈**
- 트리거 스크립트: `Claude-Code-Remote/claude-hook-notify.js`
- 채널 구현:
  - Telegram: `src/channels/telegram/telegram.js`
  - LINE: `src/channels/line/line.js`
  - Email: `src/channels/email/smtp.js`
  - Desktop: `src/channels/local/desktop.js`
- 대화 캡처: `src/utils/tmux-monitor.js`

**동작(구체)**
1) 훅 스크립트가 `.env`를 로드하고, 활성화된 채널을 구성
2) 알림 객체(notification)를 생성(타입 completed/waiting, project 등)
3) 채널별 `send()` 실행
4) Telegram/LINE/Email 채널은 “세션 토큰”을 생성하고, 향후 인바운드 커맨드를 받을 수 있도록 세션 파일을 저장

**대화/트레이스 포함 여부**
- Telegram/LINE/Email은 tmux 세션을 인식할 수 있으면 `tmux-monitor`로 최근 대화/트레이스를 캡처해 메시지에 포함하려고 시도합니다.
  - Telegram/LINE: 질문/응답 일부를 메시지 본문에 넣음
  - Email: 질문/응답 + 실행 트레이스(HTML) 포함 가능

**기술적 결정**
- 알림 메시지에 “토큰”을 반드시 넣어 **원격 명령 라우팅 키**로 사용
- “완료 감지 정확도”는 Claude 훅이 주 담당(안정적 이벤트), tmux 모니터는 보조/캡처

---

### UC-04. Telegram으로 “명령” 받기(외부 채널 웹훅 인바운드)

**목적**
- 사용자가 Telegram에서 토큰 포함 명령을 보내면, 로컬 Claude 세션으로 주입.

**트리거**
- Telegram이 우리 웹훅 서버에 update 이벤트 POST

**주요 모듈**
- 서버 실행: `Claude-Code-Remote/start-telegram-webhook.js`
- 핸들러: `Claude-Code-Remote/src/channels/telegram/webhook.js`
- 주입: `Claude-Code-Remote/src/utils/controller-injector.js`
- 세션 조회: `Claude-Code-Remote/src/data/sessions/*.json` 스캔

**동작(구체)**
1) Express가 `/webhook/telegram`에서 `update.message` 또는 `update.callback_query` 처리
2) (인증) `_isAuthorized(userId, chatId)`
   - `TELEGRAM_WHITELIST`가 있으면: whitelist에 userId/chatId가 포함돼야 함
   - 없으면: `.env`의 `TELEGRAM_CHAT_ID`/`TELEGRAM_GROUP_ID`와 chatId가 일치해야 함
3) (파싱) 메시지가 아래 형식인지 확인
   - `/cmd TOKEN1234 <command>` 또는 `TOKEN1234 <command>`
4) (세션 검증) 토큰으로 `src/data/sessions/*.json` 중 `session.token === token`인 세션을 찾고, 만료시간(`expiresAt`) 확인
5) (주입) 세션에 저장된 `tmuxSession`(또는 default) 대상으로 `ControllerInjector.injectCommand(command, tmuxSession)`
6) 결과를 Telegram으로 확인 메시지 전송

**기술적 결정(보안/신뢰)**
- 요청 자체가 Telegram에서 왔는지에 대한 “서명 검증”보다는:
  - **(A) chatId/userId whitelist** + **(B) 토큰 기반 세션 검증**
  - 을 핵심 방어로 사용.

**기술적 결정(세션 유지)**
- tmux 모드(`INJECTION_MODE=tmux`)면 `tmux send-keys`로 **기존 tmux 세션에 입력**하므로 기존 대화 컨텍스트가 유지됩니다.
- tmux 세션이 존재하지 않으면(ControllerInjector는 자동 생성 안 함) 주입이 실패합니다.

---

### UC-05. LINE으로 “명령” 받기(외부 채널 웹훅 인바운드)

**목적**
- 사용자가 LINE에서 토큰 포함 명령을 보내면, 로컬 Claude 세션으로 주입.

**주요 모듈**
- 서버 실행: `Claude-Code-Remote/start-line-webhook.js`
- 핸들러: `Claude-Code-Remote/src/channels/line/webhook.js`
- 주입: `Claude-Code-Remote/src/utils/controller-injector.js`

**동작(구체)**
1) Express가 `/webhook`에서 raw body를 받아 `x-line-signature` 검증(`LINE_CHANNEL_SECRET`)
2) (인증) whitelist(`LINE_WHITELIST`) 또는 `.env`의 `LINE_USER_ID`/`LINE_GROUP_ID`로 user/group authorize
3) (파싱) 메시지 형식: `Token ABC12345 <command>`
4) (세션 검증) `src/data/sessions/*.json`에서 토큰에 해당하는 세션을 찾고 만료 확인
5) (주입) `ControllerInjector.injectCommand(command, tmuxSession)` 호출

**기술적 결정**
- LINE은 “플랫폼 요청 서명” 검증을 구현(텔레그램 대비 강함)
- 주입/세션 유지 특성은 Telegram과 동일(세션이 존재하면 유지, 없으면 실패)

---

### UC-06. Email 답장으로 “명령” 받기(데몬 기반)

**목적**
- 이메일 알림에 답장하면, 답장 본문을 커맨드로 해석해 Claude로 주입.

**주요 모듈**
- 데몬: `Claude-Code-Remote/src/daemon/taskping-daemon.js`
- IMAP 리스너: `Claude-Code-Remote/src/relay/email-listener.js`
- 큐/실행: `Claude-Code-Remote/src/relay/command-relay.js`
- 자동화/폴백: `Claude-Code-Remote/src/automation/*`, `src/relay/claude-command-bridge.js` 등

**동작(구체)**
1) 데몬이 IMAP으로 inbox를 주기적으로 체크(`UNSEEN`, `SINCE lastCheckTime`)
2) (루프 방지) `sent-messages.json`에 기록된 message-id인 경우 skip
3) (답장 판별) subject에 `[Claude-Code-Remote #TOKEN]` + Re: 형태 등으로 reply인지 판단
4) (세션 식별) 헤더 또는 subject에서 sessionId/token을 찾고, `src/data/sessions/*.json`에서 세션 검증
   - 만료, commandCount/maxCommands 등을 검사
5) (명령 추출) 본문에서 quoted content/서명 제거 후 커맨드만 추출
6) (안전 필터) 위험 패턴을 간단히 차단
7) 커맨드를 큐잉 후, `command-relay.js`가 여러 방법으로 주입 시도:
   - Claude 전용 자동화 → 클립보드 자동화 → 단순 자동화 → 파일 브릿지 → 마지막 폴백(알림으로 붙여넣기 유도)

**기술적 결정(세션/대화 유지)**
- 이 경로는 “tmux 세션으로 명확히 지정해서 주입”하기보다 “현재 접근 가능한 Claude Code”에 붙여넣는 자동화 폴백이 많습니다.
- 따라서 **어느 창(세션)에 입력이 들어가느냐**가 환경/포커스/권한에 따라 달라질 수 있고, 그에 따라 “기존 대화 유지”가 결정됩니다.
  - 원칙적으로는 “열려있는 기존 UI에 주입”이므로 새 대화를 생성하진 않지만, 타깃이 불명확하면 사용자 개입이 필요해질 수 있습니다.

---

### UC-07. Email 답장으로 “명령” 받기(relay-pty 통합형)

**목적**
- 이메일 인바운드 + 원격 주입을 하나의 프로세스로 통합(특히 tmux 무인 주입 강화).

**주요 모듈**
- 실행: `Claude-Code-Remote/start-relay-pty.js` → `Claude-Code-Remote/src/relay/relay-pty.js`
- 주입 우선순위:
  1) tmux 무인 주입: `Claude-Code-Remote/src/relay/tmux-injector.js`
  2) 스마트 폴백: `Claude-Code-Remote/src/relay/smart-injector.js`
- 세션 매핑: `Claude-Code-Remote/src/data/session-map.json` (토큰 → tmuxSession 등)

**동작(구체)**
1) IMAP 이벤트(`mail`) + 주기적 체크로 새 메일을 처리
2) 발신자 whitelist 검사(`ALLOWED_SENDERS`)
3) subject에서 토큰 추출(`[Claude-Code-Remote #TOKEN]`)
4) 본문 정리 후 커맨드 추출
5) 토큰으로 `session-map.json`에서 세션 정보를 찾고(`tmuxSession` 등)
6) **tmux-injector로 주입 시도**
   - `injectCommandFull()`은 tmux 설치 여부 확인 후,
   - **tmux 세션이 없으면 새로 생성(createClaudeSession)할 수 있음**
   - 이후 send-keys로 커맨드 입력 + Claude confirmation dialog 자동 처리까지 시도
7) tmux 실패 시 smart-injector로 AppleScript/클립보드 폴백
8) 중복 처리 방지용 `processed-messages.json`에 기록

**기술적 결정(세션/대화 유지 vs 새 세션)**
- 이 경로는 “무인 주입”을 우선하며, 그 과정에서 **tmux 세션이 없으면 새로 만들 수 있습니다.**
  - 새로 만들어진 tmux 세션에서 `clauderun` 등을 통해 Claude를 띄우면, 그건 **새 대화 세션**입니다.
  - 반대로 기존 tmux 세션이 있으면 동일 세션으로 send-keys이므로 **대화 유지**입니다.

---

### UC-08. “로컬 터미널에서 Claude Code 5개” 시나리오 포함 여부(멀티 세션 라우팅)

**요구 시나리오**
- 사용자가 로컬에서 Claude Code 세션을 5개 띄움
- 각각에서 명령 수행
- 작업이 끝나면 앱이 감지해서 사용자에게 메신저로 알림
- 알림에 답장하면 “그 세션”에 명령이 들어가길 기대

**이 레포가 기본적으로 제공하는 것**
- “작업 끝남 감지 → 메신저 알림”은 훅 기반으로 제공(UC-03)
- “메신저 답장 → 로컬 세션 주입”도 토큰 기반으로 제공(UC-04/05)

**세션을 ‘정확히 5개로 분리’해 라우팅하려면 필요한 전제(중요)**
- 가장 안정적인 전제: **각 Claude Code 인스턴스가 서로 다른 tmux 세션에서 실행**되어야 합니다.
  - 훅 실행 시점에 `tmux display-message -p "#S"`로 세션명을 얻어 세션 레코드에 저장하고,
  - 이후 토큰이 그 tmux 세션으로 명령을 주입하는 구조이기 때문입니다.
- 반대로 “tmux 없이 일반 터미널 5개”면:
  - 훅 스크립트가 tmux 세션명을 얻지 못해 기본값(`TMUX_SESSION` 또는 하드코딩 기본)으로 뭉칠 수 있어,
  - 토큰이 “어느 대화 세션”인지 구분이 약해집니다.

**기술적 결정 요약**
- 이 설계는 “토큰 → (tmuxSession 같은) 주입 대상”을 강하게 요구합니다.
- 따라서 멀티 세션을 제대로 하려면:
  - (권장) tmux 세션 5개로 분리 + 각 세션에서 Claude 실행
  - (또는) 인스턴스별로 환경변수/설정을 달리해 훅이 구분 가능한 세션 키를 남기도록 해야 합니다.

---

## 4) 빠른 체크리스트(질문한 포인트만 한 번 더)

- Telegram 인바운드는 외부 “웹훅”인가? → **예**(Express 서버가 Telegram update 수신)
- Telegram 인바운드 사용자 검증은 사전 whitelist인가? → **예**(`TELEGRAM_WHITELIST` 또는 configured chat/group ID)
- LINE 인바운드는 서명 검증까지 하나? → **예**(`x-line-signature` 검증)
- 명령 주입은 기존 대화 세션을 유지하나?
  - Telegram/LINE 웹훅 주입: **기존 tmux 세션에 send-keys**(세션이 존재하면 유지, 없으면 실패)
  - relay-pty(tux-injector) 주입: **세션 없으면 새로 생성 가능**(새 대화가 생길 수 있음)

---

## 5) 다음 질문(정확도를 위해 확인 필요)

“5개 세션”이 아래 중 어떤 형태인지에 따라, “세션별 정확한 라우팅” 구현 수준이 달라집니다.

1) tmux 세션 5개(각각 `tmux new -s a`, `tmux new -s b` …)에서 Claude 실행?
2) tmux 1개 세션 안의 pane/window 5개?
3) tmux 없이 터미널 앱에서 창/탭 5개?

원하는 형태를 알려주면, 현재 코드 기준으로 “정확히 어디가 부족한지/어느 모듈을 손봐야 하는지”까지 유스케이스에 추가로 구체화해줄게요.

