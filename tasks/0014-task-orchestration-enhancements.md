# 0014 — Task Orchestration Enhancements (Requeue / Timeout / Fairness)

Status: **Todo**

## Goal

태스크 분배 모델을 실제 운영에 맞게 고도화한다(실패/오프라인/타임아웃/재시도 처리).

## Acceptance Criteria

- [ ] `in_progress`가 일정 시간 이상 유지되면 자동으로 `queued`로 되돌리는(재큐잉) 정책을 정의/구현한다.
- [ ] Agent가 오프라인(heartbeat timeout)일 때 해당 agent의 in-flight task를 안전하게 재할당할 수 있다.
- [ ] 우선순위(`priority`)를 FIFO 규칙과 어떻게 결합할지 명확히 정하고 구현한다.
- [ ] 한 agent의 동시 처리 한도(max in-flight) 정책을 정의하고 UI/서버에 반영한다.
- [ ] 실패 사유/재시도 횟수 등 운영 필드를 모델에 추가할지 결정하고 스키마/코드에 반영한다.

## Notes / References

- Store 인터페이스: `coordinator/internal/store/store.go`
- Supabase 스키마: `supabase/migrations/0001_init.sql`

