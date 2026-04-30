# Security Route Matrix

This matrix is the source of truth for backend route protection. New routes must pick one class before merge and tests should cover anonymous, invalid token and insufficient-role paths for protected classes.

| Class | Middleware | Examples |
| --- | --- | --- |
| Public | Tenant, firewall, CORS, rate limit | health, published posts/categories, reviews list/stats, recruitment jobs/apply, public disclosure read |
| Authenticated | `middleware.JWTMiddleware` | `/api/auth/profile`, `/api/users/me/*`, `/api/documents/*`, `/api/lms/*` |
| Content operator | `JWTMiddleware` + `RequireRoles(admin, moderator, editor)` | post/category write routes |
| Admin | `JWTMiddleware` + `RequireRoles(admin)` | users management, recruitment management, review moderation, public disclosure write routes |
| LMS scoped | `JWTMiddleware` plus service-level scope checks | teacher/student class, quiz, assignment and attempt APIs |
| System/internal | private deployment network or explicit service auth | queue workers, migration/backfill command, plugin runtime |

## Current Regression Coverage

- Documents reject anonymous and invalid tokens before private handlers.
- Posts write routes reject anonymous users and non-operator roles through shared middleware.
- Reviews and recruitment admin routes require the admin role.
- Users management routes require the admin role while `/api/users/me/*` remains authenticated-user scoped.
- LMS routes reject anonymous requests and perform teacher/student/admin scope checks inside the LMS service.
- Public disclosure read routes are public; create/delete are admin-only.

## Operating Rule

Module-local JWT parsing is not allowed for new HTTP routes. Use `internal/middleware.JWTMiddleware` so token parsing, generic client errors and claim injection remain centrally auditable.
