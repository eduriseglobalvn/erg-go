# ERG-81 FE Integration Contract

Status: BE contract notes for FE migration away from mock data.
Scope: Hoclieu content/assets/viewer, LMS teacher/student runtime APIs, Elearning student APIs.
Related OpenAPI skeleton: `docs/erg-81-openapi-skeleton.yaml`.

## Base Domains

Use the same path contracts on each domain; routing/proxy decides the product shell.

| Domain | Intended shell | API prefixes |
| --- | --- | --- |
| `https://hoclieu.erg.edu.vn` | Hoclieu content portal | `/api/hoclieu/*` |
| `https://lms.erg.edu.local` | LMS teacher/admin shell | `/api/lms/*` |
| `https://elearning.erg.edu.local` | Student elearning shell | `/api/elearning/*`, selected `/api/lms/*` runtime calls |

Local development default is still `http://localhost:8080`.

## Shared HTTP Contract

Most ERG-81 endpoints use the canonical response envelope:

```json
{
  "statusCode": 200,
  "message": "Success",
  "data": {},
  "errors": null,
  "timestamp": "2026-05-07T04:00:00Z",
  "path": "/api/lms/scopes/me",
  "request_id": "req-123"
}
```

Errors use the same envelope with `data` omitted and `errors` set to a code string, for example `UNAUTHORIZED`, `FORBIDDEN`, `SCOPE_FORBIDDEN`, `NOT_FOUND`, `ATTEMPT_PACKAGE_HASH_MISMATCH`.

Auth header:

```http
Authorization: Bearer <jwt>
X-Tenant-ID: default
```

Tenant resolution is middleware/config driven. FE should keep `X-Tenant-ID` configurable and not hard-code `default` outside local/dev fixtures.

## Permissions And Roles

### Hoclieu

Public read endpoints do not require JWT. Protected endpoints require JWT plus route permission:

| Action | Permission |
| --- | --- |
| List/get content | `hoclieu.resource.read` |
| Create content | `hoclieu.resource.create` |
| Update content | `hoclieu.resource.update` |
| Upload/create asset | `hoclieu.asset.upload` |
| Update/manage asset | `hoclieu.asset.manage` |
| Launch/viewer/stream asset | `hoclieu.asset.launch` |
| Download asset | `hoclieu.asset.download` and asset `canDownload=true` |

Enterprise policy currently defines `hoclieu.*` and `hoclieu.content.*`; ERG-81 routes use the more specific `hoclieu.resource.*` / `hoclieu.asset.*` permission strings. FE should display 403 as permission denied and not infer access only from high-level role labels.

### LMS

All `/api/lms/*` endpoints require JWT when the JWT validator is configured. Fine-grained access is enforced in service scope:

| Actor | Access behavior |
| --- | --- |
| `admin`, `erg_admin`, `global_admin`, `lms_admin` | Global LMS scope. Can create/update education units, classes, students, content, assignments, reports. |
| Teacher/manager user | Center/class scope only. Can see assigned centers/classes and class-owned students/content. |
| Student user | Own assignments/attempts/scores. Attempt mutation requires `actor.userId == attempt.studentId`. |

Enterprise roles map available permissions such as `lms.*`, `lms.class.read`, `lms.class.manage`, `lms.exam.read`, `lms.exam.manage`, `lms.grade.read`, `lms.grade.update`, `lms.assignment.submit`.

### Elearning Student

Student APIs under `/api/elearning` require JWT. Current student contract does not add a role middleware; FE should still use student/learner sessions and handle 401/404/500 envelopes.

## Hoclieu Content Contract

### Endpoint Matrix

| Method | Path | Auth | Permission | Purpose |
| --- | --- | --- | --- | --- |
| GET | `/api/hoclieu/home` | Public | None | Home hero, metrics, programs, shortcuts. |
| GET | `/api/hoclieu/programs` | Public | None | Program list. |
| GET | `/api/hoclieu/programs/{slug}` | Public | None | Program detail. |
| GET | `/api/hoclieu/taxonomy` | Public | None | Grades, subjects, categories, sections, file types. |
| GET | `/api/hoclieu/portfolio` | Public | None | Portfolio shortcuts. |
| GET | `/api/hoclieu/community` | Public | None | Community shortcuts. |
| GET | `/api/hoclieu/resources` | JWT | `hoclieu.resource.read` | Search/filter content cards. |
| POST | `/api/hoclieu/resources` | JWT | `hoclieu.resource.create` | Create resource. |
| GET | `/api/hoclieu/resources/{resourceId}` | JWT | `hoclieu.resource.read` | Resource detail with assets/items. |
| PATCH | `/api/hoclieu/resources/{resourceId}` | JWT | `hoclieu.resource.update` | Update resource metadata. |
| GET | `/api/hoclieu/resources/{resourceId}/items` | JWT | `hoclieu.resource.read` | Viewer/table-of-content items. |
| GET | `/api/hoclieu/quizzes` | JWT | `hoclieu.resource.read` | Quiz-like resources. |

### List Resources

```http
GET /api/hoclieu/resources?programSlug=global-success&gradeId=7&subjectId=tieng-anh&categoryId=textbook&sectionId=unit-1&fileType=PPTX&query=review&page=1&limit=20
Authorization: Bearer <jwt>
```

Query params:

| Param | Type | Notes |
| --- | --- | --- |
| `programSlug` | string | Optional program filter. |
| `gradeId` | string | Optional grade filter. |
| `subjectId` | string | Optional subject filter. |
| `categoryId` | string | Optional category filter. |
| `sectionId` | string | Optional section/unit filter. |
| `fileType` | enum | `PDF`, `PPTX`, `VIDEO`, `AUDIO`, `HTML5`, `LINK`, `QUIZ`, `ZIP`, `DOCX`, `XLSX`, `IMAGE`. |
| `query` | string | Case-insensitive title/subtitle/tag search. |
| `page` | number | Defaults to 1. |
| `limit` | number | Defaults to 20, max 100. |

Response example:

```json
{
  "statusCode": 200,
  "message": "Success",
  "data": {
    "data": [
      {
        "id": "hoclieu-res-001",
        "slug": "global-success-7-unit-1",
        "title": "Global Success 7 - Unit 1",
        "subtitle": "Lesson plan and slide deck",
        "thumbnailUrl": "/assets/hoclieu/unit-1.png",
        "programSlug": "global-success",
        "subjectId": "tieng-anh",
        "gradeId": "7",
        "categoryId": "textbook",
        "sectionId": "unit-1",
        "selectedFileType": "PPTX",
        "fileTypeBadge": "PPTX",
        "launchMode": "slide_image_proxy",
        "originalFileName": "unit-1.pptx",
        "detectedMimeType": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
        "fileExtension": ".pptx",
        "metadataWarnings": [],
        "priceType": "free",
        "accessState": "available",
        "canDownload": false,
        "updatedAt": "2026-05-07T04:00:00Z"
      }
    ],
    "total": 1,
    "page": 1,
    "limit": 20,
    "totalPages": 1
  },
  "errors": null,
  "timestamp": "2026-05-07T04:00:00Z",
  "path": "/api/hoclieu/resources",
  "request_id": "req-123"
}
```

File-type badge source: FE must use BE fields `selectedFileType` and `fileTypeBadge` from `ResourceCardDTO` / `AssetDTO`. Do not recompute badges from extension, URL, or old mock fixture keys. BE also exposes optional `detectedMimeType`, `fileExtension`, `metadataWarnings`, and `fileTypeAudit` for diagnostics.

### Create Resource

```http
POST /api/hoclieu/resources
Authorization: Bearer <jwt>
Content-Type: application/json
```

```json
{
  "title": "Global Success 7 - Unit 2",
  "slug": "global-success-7-unit-2",
  "subtitle": "Vocabulary practice",
  "description": "Teacher-facing resource bundle.",
  "thumbnailUrl": "/assets/hoclieu/unit-2.png",
  "programSlug": "global-success",
  "subjectId": "tieng-anh",
  "gradeId": "7",
  "categoryId": "textbook",
  "sectionId": "unit-2",
  "selectedFileType": "PDF",
  "priceType": "free",
  "canDownload": false,
  "tags": ["grade-7", "vocabulary"]
}
```

Response `201` returns a `ResourceDetailDTO`.

### Assets And Viewer

| Method | Path | Auth | Permission | Purpose |
| --- | --- | --- | --- | --- |
| POST | `/api/hoclieu/assets` | JWT | `hoclieu.asset.upload` | Create asset metadata and attach to resource. |
| PATCH | `/api/hoclieu/assets/{assetId}` | JWT | `hoclieu.asset.manage` | Update asset metadata/status. |
| GET | `/api/hoclieu/assets/{assetId}/launch` | JWT | `hoclieu.asset.launch` | Returns same-domain viewer URLs and audit metadata. |
| GET | `/api/hoclieu/assets/{assetId}/pages?token=...` | JWT | `hoclieu.asset.launch` | Returns converted page/image list. |
| GET | `/api/hoclieu/assets/{assetId}/stream?token=...` | JWT | `hoclieu.asset.launch` | Streams inline protected placeholder/content. |
| GET | `/api/hoclieu/assets/{assetId}/download?token=...` | JWT | `hoclieu.asset.download` | Downloads only when `canDownload=true`. |

Create asset body:

```json
{
  "resourceId": "hoclieu-res-001",
  "title": "Unit 1 slide deck",
  "selectedFileType": "PPTX",
  "originalFileName": "unit-1.pptx",
  "fileSizeBytes": 2480000,
  "storageProvider": "google_slides",
  "upstreamUrl": "https://docs.google.com/presentation/d/...",
  "canDownload": false
}
```

Launch response example:

```json
{
  "statusCode": 200,
  "message": "Success",
  "data": {
    "assetId": "asset_001",
    "resourceId": "hoclieu-res-001",
    "selectedFileType": "PPTX",
    "launchMode": "slide_image_proxy",
    "title": "Unit 1 slide deck",
    "viewerTokenUrl": "/api/hoclieu/assets/asset_001/pages?token=eyJ...",
    "streamUrl": "/api/hoclieu/assets/asset_001/stream?token=eyJ...",
    "expiresAt": "2026-05-07T04:15:00Z",
    "canDownload": false,
    "watermark": {
      "text": "ERG",
      "opacity": 0.08,
      "position": "center"
    },
    "audit": {
      "event": "hoclieu.asset.launch",
      "assetId": "asset_001",
      "userId": "teacher_001",
      "at": "2026-05-07T04:00:00Z"
    }
  },
  "errors": null,
  "timestamp": "2026-05-07T04:00:00Z",
  "path": "/api/hoclieu/assets/asset_001/launch",
  "request_id": "req-123"
}
```

Viewer pages response:

```json
{
  "statusCode": 200,
  "message": "Success",
  "data": {
    "assetId": "asset_001",
    "resourceId": "hoclieu-res-001",
    "selectedFileType": "PPTX",
    "pages": [
      {
        "index": 1,
        "title": "Warm up",
        "imageUrl": "/api/hoclieu/assets/asset_001/pages/1.png?token=eyJ...",
        "width": 1280,
        "height": 720,
        "durationSec": 0
      }
    ]
  },
  "errors": null,
  "timestamp": "2026-05-07T04:00:00Z",
  "path": "/api/hoclieu/assets/asset_001/pages",
  "request_id": "req-123"
}
```

Viewer security limitation: hiding Google URLs in the BE launch response is not enough if FE embeds Google directly. A Google Docs/Slides iframe still exposes the upstream URL in browser devtools/network and sometimes in iframe UI. The better direction for truly hiding links is BE-owned proxying or server-side page conversion (`slide_image_proxy`, PDF/image pages, stream endpoints), where FE renders same-domain pages/assets and never receives the Google URL.

## LMS Contract

### Scopes

| Method | Path | Auth | Permissions/scope | Purpose |
| --- | --- | --- | --- | --- |
| GET | `/api/lms/scopes/me` | JWT | Any LMS actor | Current, assigned, and available management scopes. |
| PUT | `/api/lms/scopes/current` | JWT | Scope must be valid for actor | Persist current scope. |

Scope levels: `global`, `center`, `class`. `system` input normalizes to `global`. Education unit types: `school`, `center`.

```http
GET /api/lms/scopes/me
Authorization: Bearer <jwt>
```

```json
{
  "statusCode": 200,
  "message": "Success",
  "data": {
    "canAccessGlobalErg": true,
    "assignedCenters": [
      {
        "id": "unit_001",
        "type": "school",
        "name": "ERG School District",
        "code": "ERG-SCHOOL",
        "status": "active",
        "createdAt": "2026-05-07T04:00:00Z",
        "updatedAt": "2026-05-07T04:00:00Z"
      }
    ],
    "assignedClasses": [],
    "currentScope": {
      "level": "global",
      "type": "system",
      "badge": "System",
      "icon": "shield",
      "description": "Full ERG LMS system scope"
    },
    "availableScopes": [
      {
        "level": "global",
        "type": "system",
        "badge": "System",
        "icon": "shield",
        "description": "Full ERG LMS system scope"
      }
    ]
  },
  "errors": null,
  "timestamp": "2026-05-07T04:00:00Z",
  "path": "/api/lms/scopes/me",
  "request_id": "req-123"
}
```

Update current scope:

```json
{
  "level": "class",
  "centerId": "unit_001",
  "classId": "class_001"
}
```

### Education Units

| Method | Path | Auth | Permissions/scope | Purpose |
| --- | --- | --- | --- | --- |
| GET | `/api/lms/education-units` | JWT | Global or assigned center/class visibility | List schools/centers. |
| POST | `/api/lms/education-units` | JWT | Global LMS actor | Create `school` or `center`. |
| GET | `/api/lms/education-units/{id}` | JWT | Global/assigned center/class | Get unit. |
| PATCH | `/api/lms/education-units/{id}` | JWT | Global LMS actor | Update unit. |
| GET | `/api/lms/education-units/{id}/classes` | JWT | Global/assigned center/class | List classes in unit. |
| GET | `/api/lms/centers` | JWT | Same as education units | Compatibility alias/list centers. |
| POST | `/api/lms/centers` | JWT | Global LMS actor | Compatibility create center. |
| PATCH | `/api/lms/centers/{centerId}` | JWT | Global LMS actor | Compatibility update center. |

List query params: `keyword`, `status`, `type=school|center`, `page`, `limit`.

Create body:

```json
{
  "type": "school",
  "name": "ERG School District",
  "code": "ERG-SCHOOL",
  "address": "Ha Noi",
  "managerUserId": "teacher_001"
}
```

Response item:

```json
{
  "id": "unit_001",
  "type": "school",
  "name": "ERG School District",
  "code": "ERG-SCHOOL",
  "address": "Ha Noi",
  "status": "active",
  "managerUserId": "teacher_001",
  "createdAt": "2026-05-07T04:00:00Z",
  "updatedAt": "2026-05-07T04:00:00Z"
}
```

### Classes And Students

| Method | Path | Auth | Permissions/scope | Purpose |
| --- | --- | --- | --- | --- |
| GET | `/api/lms/classes` | JWT | Global/assigned center/class | List classes. |
| POST | `/api/lms/classes` | JWT | Global or center manager | Create class. |
| GET | `/api/lms/classes/{classId}` | JWT | Class access | Get class. |
| PATCH | `/api/lms/classes/{classId}` | JWT | Global, center manager, or homeroom teacher | Update class. |
| GET | `/api/lms/classes/{classId}/students` | JWT | Class access | Students in class. |
| POST | `/api/lms/classes/{classId}/students/bulk-move` | JWT | Global LMS actor | Move students. |
| GET | `/api/lms/students` | JWT | Global/assigned scope | List students. |
| POST | `/api/lms/students` | JWT | Global LMS actor | Create student. |
| GET | `/api/lms/students/{studentId}` | JWT | Student/class access | Get student detail. |
| PATCH | `/api/lms/students/{studentId}` | JWT | Student/class access | Update student. |
| GET | `/api/lms/students/{studentId}/journey` | JWT | Student/class access | Student journey/report. |

Class list query params: `centerId` or `unitId`, `grade`, `keyword`, `status`, `academicYear`, `page`, `limit`.

Create class body:

```json
{
  "centerId": "unit_001",
  "name": "7A1",
  "grade": "7",
  "academicYear": "2026-2027",
  "homeroomTeacherId": "teacher_001"
}
```

Student list query params: `centerId` or `unitId`, `classId`, `keyword`, `status`, `progress`, `subjectId`, `cursor`, `limit`.

Create student body:

```json
{
  "fullName": "Nguyen Van A",
  "classId": "class_001",
  "birthday": "2013-09-05T00:00:00Z",
  "phone": "0900000000",
  "note": "Imported from LMS roster"
}
```

Create student response includes a temporary password once:

```json
{
  "student": {
    "id": "student_001",
    "fullName": "Nguyen Van A",
    "username": "student_001",
    "centerId": "unit_001",
    "centerName": "ERG School District",
    "classId": "class_001",
    "className": "7A1",
    "status": "active",
    "completedAssignments": 0
  },
  "tempPassword": "A1b2C3d4"
}
```

### Dashboard And Reports

| Method | Path | Auth | Permissions/scope | Purpose |
| --- | --- | --- | --- | --- |
| GET | `/api/lms/dashboard/overview` | JWT | Global/assigned scope | Summary metrics. |
| GET | `/api/lms/dashboard/interventions` | JWT | Global/assigned scope | Students needing support. |
| GET | `/api/lms/assignments/active` | JWT | Global/assigned scope | Active assignment cards. |
| GET | `/api/lms/classes/{classId}/reports` | JWT | Class access | Class report. |
| GET | `/api/lms/reports/classroom` | JWT | Global/assigned scope | Classroom report. |
| GET | `/api/lms/reports/students/{studentId}` | JWT | Student/class access | Student journey. |
| GET | `/api/lms/reports/assignments/{assignmentId}` | JWT | Assignment access | Assignment report. |
| GET | `/api/lms/reports/export` | JWT | Global/assigned scope | Export URL. |

Dashboard query params: `scopeType=global|center|class`, `unitId`, `centerId`, `classId`, `range`.

```http
GET /api/lms/dashboard/overview?scopeType=class&classId=class_001&range=30d
Authorization: Bearer <jwt>
```

```json
{
  "statusCode": 200,
  "message": "Success",
  "data": {
    "scope": {
      "scopeType": "class",
      "unitId": "unit_001",
      "centerId": "unit_001",
      "classId": "class_001",
      "range": "30d",
      "classCount": 1,
      "studentCount": 32
    },
    "metrics": {
      "openAssignments": 4,
      "needsSupport": 3,
      "completed": 28,
      "completedAssignments": 108,
      "totalAssignments": 128,
      "totalStudents": 32,
      "completionRate": 84.38
    },
    "generatedAt": "2026-05-07T04:00:00Z"
  },
  "errors": null,
  "timestamp": "2026-05-07T04:00:00Z",
  "path": "/api/lms/dashboard/overview",
  "request_id": "req-123"
}
```

### Question Bank

| Method | Path | Auth | Permissions/scope | Purpose |
| --- | --- | --- | --- | --- |
| GET | `/api/lms/question-bank/subjects` | JWT | Content scope access | Subject list alias. |
| GET | `/api/lms/question-bank/categories` | JWT | Content scope access | Category/topic helper. |
| GET | `/api/lms/question-bank/questions` | JWT | Content scope access | Question list alias. |
| POST | `/api/lms/question-bank/questions` | JWT | Valid content scope | Create question alias. |
| PATCH | `/api/lms/question-bank/questions/{questionId}` | JWT | Valid content scope | Update question alias. |
| GET | `/api/lms/subjects` | JWT | Content scope access | Subject list. |
| GET | `/api/lms/subjects/{subjectId}/levels` | JWT | Content scope access | Levels. |
| GET | `/api/lms/levels/{levelId}/topics` | JWT | Content scope access | Topics. |
| GET | `/api/lms/questions` | JWT | Content scope access | List questions. |
| POST | `/api/lms/questions` | JWT | Valid content scope | Create question. |
| PATCH | `/api/lms/questions/{questionId}` | JWT | Valid content scope | Update question. |
| DELETE | `/api/lms/questions/{questionId}` | JWT | Valid content scope | Archive question. |
| POST | `/api/lms/questions/random-pick` | JWT | Content scope access | Pick random question set. |

List questions query params: `scope=global|center`, `centerId`, `subjectId`, `levelId`, `topicId`, `keyword`, `kind` or `type`, `cursor`, `limit`.

Create question body:

```json
{
  "scope": {
    "type": "global"
  },
  "subjectId": "subject_001",
  "levelId": "level_001",
  "topicId": "topic_001",
  "kind": "single_choice",
  "type": "single_choice",
  "stem": "Choose the correct answer.",
  "choices": [
    { "id": "A", "label": "A. Option one", "correct": true },
    { "id": "B", "label": "B. Option two", "correct": false }
  ],
  "answer": "A",
  "metadata": {
    "difficulty": "easy"
  }
}
```

Random pick body:

```json
{
  "subjectId": "subject_001",
  "levelId": "level_001",
  "topicIds": ["topic_001"],
  "count": 10,
  "typeMix": {
    "single_choice": 8,
    "short_answer": 2
  },
  "excludeQuestionIds": ["question_009"]
}
```

### Quiz Bank

| Method | Path | Auth | Permissions/scope | Purpose |
| --- | --- | --- | --- | --- |
| GET | `/api/lms/quiz-bank` | JWT | Content scope access | Quiz list alias. |
| GET | `/api/lms/quizzes` | JWT | Content scope access | List quizzes. |
| POST | `/api/lms/quizzes` | JWT | Valid content scope | Create quiz. |
| POST | `/api/lms/quizzes/from-questions` | JWT | Valid content scope from first question | Create quiz from selected questions. |
| POST | `/api/lms/quizzes/random` | JWT | Valid content scope | Create random quiz. |
| GET | `/api/lms/quizzes/{quizId}` | JWT | Quiz access | Quiz detail. |
| PATCH | `/api/lms/quizzes/{quizId}` | JWT | Quiz access | Update quiz. |
| PUT | `/api/lms/quizzes/{quizId}` | JWT | Quiz access | FE compatibility update. |
| POST | `/api/lms/quizzes/{quizId}/publish` | JWT | Quiz access | Publish/package quiz. |
| GET | `/api/lms/quizzes/{quizId}/package` | JWT | Quiz access | Runtime package. |
| GET | `/api/lms/quizzes/{quizId}/students` | JWT | Quiz/class access | Student progress by quiz. |

List quiz query params: `scope`, `centerId`, `subjectId`, `levelId`, `kind`, `keyword`, `cursor`, `limit`.

Create quiz body:

```json
{
  "scope": {
    "type": "global"
  },
  "title": "Unit 1 Review Quiz",
  "kind": "practice",
  "subjectId": "subject_001",
  "levelId": "level_001",
  "topicIds": ["topic_001"],
  "questionIds": ["question_001", "question_002"],
  "settings": {
    "durationMinutes": 20,
    "shuffleQuestions": true
  },
  "themeId": "default"
}
```

Update quiz body:

```json
{
  "slides": [],
  "settings": {
    "durationMinutes": 25
  },
  "result": {
    "passPercent": 70
  },
  "theme": {
    "accent": "#2563eb"
  }
}
```

Package response shape:

```json
{
  "version": 3,
  "packageHash": "sha256:abc",
  "contentHash": "sha256:def",
  "signature": "optional-signature",
  "gradingMode": "auto",
  "quiz": {
    "quiz": {
      "id": "quiz_001",
      "scope": { "type": "global" },
      "title": "Unit 1 Review Quiz",
      "kind": "practice",
      "subjectId": "subject_001",
      "levelId": "level_001",
      "topicIds": ["topic_001"],
      "questionIds": ["question_001"],
      "status": "published",
      "version": 3,
      "packageHash": "sha256:abc",
      "createdAt": "2026-05-07T04:00:00Z",
      "updatedAt": "2026-05-07T04:00:00Z"
    },
    "slides": [],
    "settings": {},
    "result": {},
    "theme": {}
  }
}
```

### Assignments And Attempts

| Method | Path | Auth | Permissions/scope | Purpose |
| --- | --- | --- | --- | --- |
| POST | `/api/lms/assignments` | JWT | Global/class teacher scope | Create assignment batch. |
| POST | `/api/lms/assignments/deliveries` | JWT | Global/class teacher scope | Create assignment and mark delivered. |
| GET | `/api/lms/assignments/{assignmentId}/progress` | JWT | Assignment/class access | Progress summary. |
| GET | `/api/lms/classes/{classId}/assignments` | JWT | Class access | Class assignment list. |
| GET | `/api/lms/students/me/assignments` | JWT | Student actor | Current student's assignments. |
| GET | `/api/lms/students/me/scores` | JWT | Student actor | Current student's scores. |
| POST | `/api/lms/attempts` | JWT | Student must be assignment recipient | Start/reuse active attempt. |
| PATCH | `/api/lms/attempts/{attemptId}/draft` | JWT | Attempt owner | Save offline draft. |
| PUT | `/api/lms/attempts/{attemptId}/answers/{questionId}` | JWT | Attempt owner | Save answer canonical path. |
| POST | `/api/lms/attempts/{attemptId}/answers` | JWT | Attempt owner | FE compatibility body includes `questionId`. |
| POST | `/api/lms/attempts/{attemptId}/submit` | JWT | Attempt owner | Submit attempt. |
| POST | `/api/lms/attempts/{attemptId}/sync` | JWT | Attempt owner | Offline sync. |

Create assignment body:

```json
{
  "classId": "class_001",
  "quizIds": ["quiz_001"],
  "recipientMode": "all",
  "studentIds": [],
  "dueAt": "2026-05-14T16:59:59Z",
  "teacherNote": "Complete before next lesson."
}
```

Start attempt body:

```json
{
  "assignmentId": "assignment_001",
  "quizId": "quiz_001",
  "packageId": "pkg_quiz_001_v3",
  "packageHash": "sha256:abc"
}
```

Attempt response:

```json
{
  "id": "attempt_001",
  "assignmentId": "assignment_001",
  "quizId": "quiz_001",
  "studentId": "student_001",
  "packageId": "pkg_quiz_001_v3",
  "packageHash": "sha256:abc",
  "status": "in_progress",
  "answers": {},
  "score": 0,
  "maxScore": 100,
  "percent": 0,
  "passed": false,
  "startedAt": "2026-05-07T04:00:00Z",
  "updatedAt": "2026-05-07T04:00:00Z"
}
```

Save answer canonical:

```http
PUT /api/lms/attempts/attempt_001/answers/question_001
Authorization: Bearer <jwt>
Content-Type: application/json
```

```json
{
  "answer": "A",
  "clientResult": {
    "correct": true,
    "elapsedMs": 4300
  },
  "answeredAt": "2026-05-07T04:02:00Z"
}
```

Save answer compatibility:

```json
{
  "questionId": "question_001",
  "answer": "A",
  "clientResult": {
    "correct": true
  }
}
```

Sync body:

```json
{
  "packageHash": "sha256:abc",
  "quizVersion": "3",
  "attempt": {
    "answers": {
      "question_001": "A"
    }
  },
  "events": [
    {
      "type": "answer_saved",
      "questionId": "question_001",
      "at": "2026-05-07T04:02:00Z"
    }
  ],
  "client": {
    "deviceId": "web-001",
    "offline": false
  }
}
```

Submit body:

```json
{
  "answers": {
    "question_001": "A"
  },
  "submittedAt": "2026-05-07T04:10:00Z"
}
```

Submit response:

```json
{
  "score": 8,
  "maxScore": 10,
  "percent": 80,
  "passed": true
}
```

## Elearning Student Contract

All endpoints below are mounted under `/api/elearning`, require JWT, and return canonical envelopes.

| Method | Path | Query/body | Purpose |
| --- | --- | --- | --- |
| GET | `/dashboard` | None | Student summary, assignments, scores, announcements, notifications. |
| GET | `/assignments` | `status` optional | Student assignment list. |
| GET | `/assignments/{assignmentId}` | None | Assignment detail and attempts. |
| GET | `/scores` | `subjectId` optional | Score list. |
| GET | `/announcements` | None | Student announcements. |
| GET | `/notifications` | None | Student notifications and unread count. |
| POST | `/notifications/{notificationId}/read` | None | Mark notification read. |
| GET | `/discussions` | None | Student discussion threads. |
| POST | `/discussions` | JSON body | Create discussion. |
| POST | `/discussions/{threadId}/replies` | JSON body | Reply to discussion. |

Dashboard response example:

```json
{
  "statusCode": 200,
  "message": "Success",
  "data": {
    "student": {
      "id": "student_001",
      "roles": ["student"],
      "tenantId": "default"
    },
    "summary": {
      "openAssignments": 2,
      "submitted": 5,
      "unread": 1,
      "discussions": 3
    },
    "assignments": [
      {
        "id": "assignment_001",
        "studentId": "student_001",
        "title": "Unit 1 Review Quiz",
        "quizId": "quiz_001",
        "status": "open",
        "dueAt": "2026-05-14T16:59:59Z",
        "updatedAt": "2026-05-07T04:00:00Z",
        "packageUrl": "/api/lms/quizzes/quiz_001/package",
        "teacherNote": "Complete before next lesson."
      }
    ],
    "scores": [
      {
        "assignmentId": "assignment_000",
        "studentId": "student_001",
        "bestScore": 8,
        "maxScore": 10,
        "percent": 80,
        "updatedAt": "2026-05-06T04:00:00Z"
      }
    ],
    "announcements": [
      {
        "id": "ann_001",
        "title": "Class update",
        "content": "Bring workbook tomorrow.",
        "pinned": true,
        "createdAt": "2026-05-07T04:00:00Z"
      }
    ],
    "notifications": [
      {
        "id": "noti_001",
        "studentId": "student_001",
        "title": "New assignment",
        "body": "Unit 1 Review Quiz is available.",
        "read": false,
        "createdAt": "2026-05-07T04:00:00Z"
      }
    ],
    "generatedAt": "2026-05-07T04:00:00Z"
  },
  "errors": null,
  "timestamp": "2026-05-07T04:00:00Z",
  "path": "/api/elearning/dashboard",
  "request_id": "req-123"
}
```

Create discussion:

```json
{
  "title": "Question about Unit 1",
  "content": "Can I resubmit the quiz after practice?",
  "assignmentId": "assignment_001"
}
```

Create discussion reply:

```json
{
  "content": "I found the answer in the teacher note."
}
```

Notification read response:

```json
{
  "id": "noti_001",
  "studentId": "student_001",
  "status": "read",
  "readAt": "2026-05-07T04:05:00Z"
}
```

## FE Migration Checklist

- Replace Hoclieu mock taxonomy with `GET /api/hoclieu/taxonomy`; keep enum handling for all `AssetFileType` values.
- Replace Hoclieu resource cards with `GET /api/hoclieu/resources`; render file badges from BE `selectedFileType` / `fileTypeBadge`.
- Replace any direct Google Docs/Drive iframe URL usage with launch response `viewerTokenUrl`, `streamUrl`, or converted page/image flow. Do not persist or expose upstream Google URLs in FE state.
- Move Hoclieu create/edit flows to `POST/PATCH /api/hoclieu/resources` and `POST/PATCH /api/hoclieu/assets`; show 400 invalid file type from BE.
- Replace LMS scope mocks with `GET /api/lms/scopes/me` and `PUT /api/lms/scopes/current`; wire dashboard filters to `scopeType`, `unitId`, `centerId`, `classId`, `range`.
- Replace units/classes/students fixtures with `/api/lms/education-units`, `/classes`, `/students`; keep `centerId` and `unitId` compatibility.
- Replace dashboard cards with `/api/lms/dashboard/overview`, `/dashboard/interventions`, and `/assignments/active`.
- Replace question bank mocks with `/api/lms/subjects`, `/subjects/{id}/levels`, `/levels/{id}/topics`, `/questions`, `/questions/random-pick`.
- Replace quiz editor persistence with `/api/lms/quizzes`, `/quizzes/from-questions`, `/quizzes/random`, `PATCH|PUT /quizzes/{id}`, `/publish`, `/package`.
- Replace attempt runtime mocks with `/api/lms/attempts`, `/draft`, `/answers`, `/submit`, `/sync`; preserve package hash conflict handling.
- Replace Elearning student dashboard, assignments, scores, announcements, notifications, and discussions with `/api/elearning/*` student endpoints.
- Keep feature flags only for workflows not represented in this contract; remove mock modules once each screen has a BE-backed smoke path.

## Known Limitations / Blockers

- Swagger generated docs (`docs/swagger.yaml`, `docs/swagger.json`, `docs/docs.go`) are not regenerated here because that would require route annotations and would touch generated files broadly while other workers are editing controllers/services.
- `docs/openapi.yaml` is a separate, older bot-oriented skeleton and is not the canonical generated Swagger for ERG-81.
- Hoclieu viewer currently returns same-domain token URLs, but true upstream link hiding requires BE proxy/page conversion; direct Google iframe embeds remain link-leaky by browser design.
- Permission naming for Hoclieu route middleware (`hoclieu.resource.*`, `hoclieu.asset.*`) is more specific than the enterprise policy constants currently shown in access-control docs (`hoclieu.content.*`). FE should rely on API responses/claims, not role-name guesses.
