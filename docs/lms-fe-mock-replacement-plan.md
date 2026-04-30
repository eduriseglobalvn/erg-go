# LMS FE Mock Replacement Plan

This file maps eLearning FE mock-heavy workflows to backend ownership so mock modules can be removed deliberately instead of drifting screen by screen.

| FE workflow | Backend status | Endpoint ownership | Notes |
| --- | --- | --- | --- |
| Auth/session | Implemented | `/api/lms/auth/login`, `/register`, `/logout`, `/profile`, `/sessions`, `/accounts/{id}/profile`, `/accounts/{id}/password` | Apple provider route exists but returns provider-not-configured until Apple identity verification is enabled. |
| Quiz list/detail/package | Implemented | `/api/lms/quizzes`, `/quizzes/{quizId}`, `/quizzes/{quizId}/package` | FE `PUT /quizzes/{id}` compatibility is registered beside canonical `PATCH`. |
| Attempt lifecycle | Implemented | `/api/lms/attempts`, `/attempts/{attemptId}/answers`, `/submit`, `/sync` | FE body-based answer save is registered beside canonical question-id path. |
| Quiz student progress | Implemented | `/api/lms/quizzes/{quizId}/students` | Supports class/status filters. |
| Classroom overview | Implemented baseline | `/api/lms/classes`, `/classes/{classId}/assignments`, `/students`, `/reports/classroom` | FE cards should use paginated class/student/report DTOs instead of local fixtures. |
| Assignment control | Implemented baseline | `/api/lms/assignments`, `/assignments/{assignmentId}/progress` | Policy fields such as late/retake windows should be added through DTO versioning when FE enables them. |
| Teacher live tracking | Planned extension | `/api/lms/attempts/{attemptId}/sync`, reports endpoints | Current sync supports attempt state; live heartbeat stream should be added as a separate real-time contract if needed. |
| Class reports | Implemented baseline | `/api/lms/reports/classroom`, `/reports/assignments/{assignmentId}`, `/reports/students/{studentId}`, `/reports/export` | Percentile/deep journey widgets should extend these DTOs. |
| Student dashboard | Implemented baseline | `/api/lms/students/me/assignments`, `/students/me/scores`, `/announcements` | Achievements are not currently modeled and should be consciously planned before FE uses them. |
| Quiz editor persistence | Implemented baseline | `/api/lms/quizzes`, `/quizzes/from-questions`, `/quizzes/random`, `/quizzes/{quizId}/publish` | Autosave/version history remains a planned extension. |
| Question bank | Implemented baseline | `/api/lms/questions`, `/questions/random-pick`, taxonomy endpoints | Advanced question types should be added through explicit schema evolution. |
| Public disclosure/content | Split | `/public-disclosure` public read, admin write; `/api/documents` protected private docs | Public disclosure writes require admin; private documents require JWT and owner/admin checks. |
| Sheet import/writeback | Implemented baseline | `/api/lms/imports/google-sheet/tabs`, `/preview`, `/commit`, `/imports/{jobId}`, `/writeback` | Browser-side parsing/writeback should be replaced by preview/commit/writeback calls. |

## Removal Rule

Every FE mock module must be tagged with one of:

- `implemented`: replace with the endpoint above.
- `planned`: keep behind a feature flag and link to a backend ticket.
- `removed`: delete the FE feature path if it is no longer part of the LMS product.

Backend contract tests currently cover route registration and security behavior for the highest-risk auth/LMS/document paths. DTO golden tests should be added as FE removes each mock module.
