# LMS erg-go Endpoint Inventory

This inventory supports ERG-40 by marking existing erg-go modules as `reuse`, `refactor`, or `deprecate for LMS` before the LMS-specific API work continues.

| Area | Current endpoints | LMS decision | Notes |
| --- | --- | --- | --- |
| Auth | `POST /api/auth/login`, `POST /api/auth/logout`, `POST /api/auth/refresh`, `GET /api/auth/profile` | Refactor/reuse | ERG-32 adds one-active-device session handling, device metadata, and session replacement errors. Profile still needs LMS scope and permission summary in ERG-33. |
| Users | `GET /api/users`, `GET /api/users/{userID}`, `GET /api/users/me/sessions` | Refactor/reuse | Keep generic user management, then add LMS filters for role, center, class, teacher, and student. Session endpoints now return device-oriented session DTOs. |
| Access Control | `/api/access-control/*` | Reuse | Reuse for role and effective permission checks including global ERG access, center/class permissions, and internal content visibility. |
| Courses | `/api/courses*` | Reuse/refactor | Reuse course and HocLieu metadata where possible. LMS course delivery should keep request/response DTOs separate from teacher/class assignment DTOs. |
| Documents | `/documents*`, `/api/documents*` | Refactor | `/api/documents` alias is mounted for API consistency while `/documents` remains as a legacy-compatible path. LMS document types can extend DTOs later. |
| Elearning content | `/api/elearning/categories`, `/api/elearning/levels/{slug}` | Refactor | Public taxonomy read APIs can seed subject/level/topic/unit work. Admin LMS content should move toward `/api/lms/admin/content/*`. |
| Admin elearning | `/admin/elearning/categories`, `/admin/elearning/levels`, `/admin/elearning/units` | Refactor | Keep existing admin behavior but do not add new LMS admin routes under the bare `/admin` prefix. |
| Notifications | `/api/notifications/*`, `/api/channels/*` | Reuse/refactor | Reuse for student notifications, announcements, moderation events, and session replacement signals. Static notification routes are registered before `/:id` routes to avoid path conflicts. |
| Analytics | `/api/insight/*` | Deprecate for LMS reports | Keep for web/session tracking only. LMS reports should be a dedicated reporting module and should not depend on traffic analytics DTOs. |
| Operations | `/api/operations/*` | Deprecate for LMS dashboard | Keep for technical admin only. Do not mix operation logs, firewall, or system config into teacher/student dashboards. |

## Follow-up mapping

| Jira | Next BE scope | Dependency on this inventory |
| --- | --- | --- |
| ERG-33 | LMS scopes, centers, classes, students | Reuse auth profile, users, and access-control patterns. |
| ERG-34 | Google Sheet import | Should attach imported students/classes to ERG-33 DTOs. |
| ERG-35 | Taxonomy, question bank, quiz bank | Refactor elearning taxonomy without adding new bare `/admin` routes. |
| ERG-36 | Quiz runtime, assignments, attempts, offline sync | Build as dedicated LMS runtime APIs, not course/document side effects. |
| ERG-37 | Discussion, announcements, notifications, moderation | Reuse notification module and keep moderation DTOs explicit. |
| ERG-38 | Reports and HocLieu/internal docs | Reuse courses/documents where suitable, keep LMS reports separate from `/api/insight`. |
