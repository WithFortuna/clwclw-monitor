# 0017 — Auth / RBAC (Formal)

Status: **Todo**

## Goal

대시보드/API를 “공유 토큰”을 넘어 사용자/권한 기반으로 보호할 수 있게 한다.

## Acceptance Criteria

- [ ] 인증 방식(예: Supabase Auth, 자체 JWT, OAuth 등)을 선택하고 위협 모델을 정리한다.
- [ ] UI 로그인/세션 관리와 API 인증 헤더 규칙을 확정한다.
- [ ] 최소 RBAC(예: viewer/operator/admin)와 기능별 권한 범위를 정의하고 구현한다.
- [ ] API Key/토큰 회전(rotate)/폐기(revoke) 플로우를 제공한다.

## Notes / References

- 현재 임시 인증: `COORDINATOR_AUTH_TOKEN` + `X-Api-Key` (`coordinator/internal/httpapi/middleware.go`)

