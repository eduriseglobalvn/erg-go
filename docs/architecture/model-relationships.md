# Model Relationship Hardening

This document is the handoff matrix for Jira ERG-123 through ERG-134. It records
which persisted references are enforced by PostgreSQL foreign keys, which Mongo
references are service validated, and which references remain intentionally
documented only for legacy or audit reasons.

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

## Migration And Rollback Plan

Relationship hardening is deployed in small, reversible steps.

1. Add indexes and service validation first. These changes are backward
   compatible and prevent new orphan data before database constraints are
   applied.
2. Run PostgreSQL orphan checks with `postgrescore.CheckRelationshipIntegrity`.
   Export the report and fix invalid rows with explicit product approval.
3. Apply PostgreSQL foreign keys after the report is clean. Keep `restrict`,
   `set null`, and `cascade` policies aligned with the matrix above.
4. Validate Mongo relationship indexes in staging by running each module
   bootstrap and checking that `EnsureIndexes` succeeds.
5. Roll back by disabling the new write path first, dropping newly added
   database constraints or indexes second, and restoring data from the orphan
   report when needed. Do not delete production data as part of rollback.

## Mongo Relationship Matrix

Mongo references stay service validated so LMS, hoclieu, elearning,
notifications, and crawler can be split into microservices later without a
shared database FK boundary.

| Module | Relationship | Required | Enforcement | Guardrail |
| --- | --- | --- | --- | --- |
| lms | class.center_id -> centers._id | yes | service validation | `CreateClass` loads the center in the same tenant and checks actor scope. |
| lms | student.class_id -> classes._id | yes | service validation | `CreateStudent` loads the class in the same tenant and checks actor scope. |
| lms | assignment.class_id -> classes._id | yes | service validation | Delivery path validates class and recipient scope before insert. |
| lms | assignment.quiz_ids -> quizzes._id | yes | service validation | Quiz publishing and delivery paths load quiz IDs before assignment. |
| lms | attempt.assignment_id/quiz_id/student_id | yes | unique partial index plus service validation | Active attempts are unique per tenant, assignment, quiz, student, and status. |
| lms | question.subject_id/level_id/topic_id | yes/topic optional | service validation plus catalog indexes | ObjectID parsing rejects malformed references before repository writes. |
| lms | discussion.thread_id/class_id/assignment_id | yes where present | service validation plus thread/reply indexes | Discussion queries are tenant scoped and indexed by class/assignment/thread. |
| lms | announcement.class_ids/student_ids | depends on target | service validation plus target indexes | Announcement targeting remains tenant scoped and indexed by class target. |
| hoclieu | resource.asset_id/item_ids/taxonomy_ids | depends on content type | seed/service validation plus indexes | Repository indexes cover taxonomy, assets, resources, presets, and items. |
| elearning | category.parent_id -> categories._id | no | service validation | Update rejects unsafe IDs, self-parent, and missing parent in the same tenant. |
| elearning | level.category_id -> categories._id | yes | service validation | Create and update load the category in the same tenant before saving. |
| elearning | unit.level_id -> levels._id | yes | service validation | Create and update load the level in the same tenant before saving. |
| notifications | target user/group references | no | documented/service boundary | Cross-context delivery references are validated before fan-out, not by DB FK. |
| crawler | source/job/content references | no | documented/service boundary | External source references are normalized and treated as integration data. |

## API DTO And Form Validation Rules

- Required reference IDs must be present at DTO binding time and must be loaded
  by the service before persistence.
- Optional reference IDs must be either empty or pass `ValidateReferenceID`.
- Reference arrays must reject empty values, duplicates, and oversized batches
  with `ValidateReferenceIDs`.
- Tenant-scoped services must load referenced documents using the same tenant ID
  that will be written to the child document.
- Relationship updates must validate the new parent before mutating the child.
- Self-references and cycles are rejected at the service layer when the model
  supports a parent pointer.
- Repository filters must include `tenant_id` for tenant-owned documents.

## Architecture Guardrails

- Every module keeps the canonical `api`, `application`, `domain`, and
  `infrastructure` folders.
- Module root Go files are limited to `module.go`, `adapters.go`, and
  `compat.go`.
- Relationship docs must mention ERG-129 through ERG-134 so the final hardening
  tasks remain discoverable after context compaction.
- Mongo relationship indexes are created in module bootstrap paths and must not
  require destructive migration steps.

## Handoff Checklist

- ERG-129: migration and rollback plan documented in this file.
- ERG-130: LMS Mongo relationship indexes and validation expectations recorded.
- ERG-131: hoclieu, elearning, notifications, and crawler relationship
  boundaries documented; elearning update paths validate parent/category/level.
- ERG-132: architecture guardrail test covers canonical module layout and this
  relationship handoff.
- ERG-133: reusable reference ID validators exist for DTO/form handlers and
  service code.
- ERG-134: this checklist is the durable handoff artifact for future agents.
