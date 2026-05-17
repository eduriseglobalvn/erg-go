# Model Relationship Hardening

This document is the handoff matrix for Jira ERG-123 through ERG-128. It records
which persisted references are enforced by PostgreSQL foreign keys, which remain
service validated, and which are intentionally documented only for legacy or
audit reasons.

## Rules

- Enforce same-bounded-context required references with PostgreSQL foreign keys.
- Keep cross-context or future-microservice references documented or service
  validated unless they are already owned by the same relational core.
- Validate polymorphic references in service/repository code because one column
  can point to multiple target tables.
- Run orphan checks before applying relationship constraints to an existing
  database.
- Do not delete or rewrite production data automatically during constraint
  rollout.

## PostgreSQL Relationship Matrix

| Relationship | Source | Target | Required | Enforcement | Delete policy | Notes |
| --- | --- | --- | --- | --- | --- | --- |
| auth_session_user | user_sessions.user_id | users.id | yes | database FK | cascade | Sessions are ephemeral and owned by users. |
| user_role_user | user_roles.user_id | users.id | yes | database FK | cascade | Join row cannot outlive user. |
| user_role_role | user_roles.role_id | roles.id | yes | database FK | cascade | Join row cannot outlive role. |
| role_permission_role | role_permissions.role_id | roles.id | yes | database FK | cascade | Join row cannot outlive role. |
| role_permission_permission | role_permissions.permission_id | permissions.id | yes | database FK | cascade | Join row cannot outlive permission. |
| profile_user | profiles.user_id | users.id | yes | database FK | cascade | One profile per user. |
| certificate_user | certificates.user_id | users.id | yes | database FK | cascade | Certificate belongs to user profile. |
| social_account_user | social_accounts.user_id | users.id | yes | database FK | cascade | External account binding belongs to user. |
| course_progress_user | course_progress.user_id | users.id | yes | database FK | cascade | Progress belongs to user. |
| work_shift_user | work_shifts.user_id | users.id | yes | database FK | restrict | Scheduling/audit rows should not disappear silently. |
| post_category | posts.category_id | post_categories.id | yes | database FK | restrict | Category is required for post filtering. |
| center_parent | centers.parent_id | centers.id | no | database FK | set null | Root centers have no parent. |
| user_access_scope_user | user_access_scopes.user_id | users.id | yes | database FK | cascade | Scope belongs to user. |
| user_access_scope_center | user_access_scopes.center_id | centers.id | yes | database FK | cascade | Scope belongs to center. |
| community_post_topic | community_posts.topic_id | community_topics.id | yes | database FK | restrict | Posts require an existing topic. |
| community_post_author | community_posts.author_id | users.id | yes | database FK | restrict | Preserve author attribution. |
| community_media_post | community_media.post_id | community_posts.id | yes | database FK | cascade | Media is owned by post. |
| community_comment_post | community_comments.post_id | community_posts.id | yes | database FK | cascade | Comments are owned by post. |
| community_comment_author | community_comments.author_id | users.id | yes | database FK | restrict | Preserve author attribution. |
| community_reaction_user | community_reactions.user_id | users.id | yes | database FK | cascade | Reaction belongs to user. |
| community_follow_user | community_follows.user_id | users.id | yes | database FK | cascade | Follow belongs to user. |
| community_reaction_target | community_reactions.target_id | posts/comments | yes | service validation | service managed | Polymorphic target validated in community repository. |
| community_follow_target | community_follows.target_id | topics/users | yes | service validation | service managed | Polymorphic target validated in community repository. |
| auth_login_attempt_user | auth_login_attempts.user_id | users.id | no | documented only | audit retained | Login attempts may be anonymous and should survive account lifecycle changes. |
| recruitment_candidate_job | candidates.job_id | jobs.id | no | documented only | snapshot retained | Candidate keeps job title snapshot and legacy records may not have job_id. |

The canonical machine-readable copy lives in
`internal/persistence/postgrescore/relationships.go`.

## Orphan Check

Run `postgrescore.CheckRelationshipIntegrity(ctx, db)` before adding constraints
to an existing database. The report lists the relationship name, source table,
source column, target table, invalid row count, and sample source IDs.

The check intentionally skips polymorphic relationship targets and delegates them
to service validation.

## Community Polymorphic References

Community reactions allow these targets:

- `post`: must exist in `community_posts` with the same tenant and not be soft
  deleted.
- `comment`: must exist in `community_comments` with the same tenant and not be
  soft deleted.

Community follows allow these targets:

- `topic`: must exist in `community_topics` with the same tenant and not be soft
  deleted.
- `user`: must exist in `users`. Tenant checks for users remain application
  scoped because auth users are a shared relational core.

## Rollout Order

1. Run orphan checks.
2. Fix or explicitly approve any orphan rows.
3. Apply database FK associations and migration.
4. Run service tests for community target validation.
5. Run `go test ./...`.
