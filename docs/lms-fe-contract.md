# eLearning FE Contract Notes

## Implemented Compatibility Routes

The backend now registers these FE-facing LMS routes:

- `PUT /api/lms/quizzes/{quizId}`
- `POST /api/lms/attempts/{attemptId}/answers`
- `POST /api/lms/classes/{classId}/students/bulk-move`
- `POST /api/lms/questions/random-pick`
- `POST /api/lms/quizzes/from-questions`
- `POST /api/lms/quizzes/random`

The backend now also exposes `/api/lms/auth/*` aliases for the existing auth controller:

- `POST /api/lms/auth/login`
- `POST /api/lms/auth/register`
- `POST /api/lms/auth/logout`
- `GET /api/lms/auth/profile`
- `GET /api/lms/auth/sessions`
- `PUT /api/lms/auth/accounts/{id}/profile`
- `PUT /api/lms/auth/accounts/{id}/password`
- `POST /api/lms/auth/providers/google`
- `POST /api/lms/auth/providers/apple`

## Compatibility Caveats

`/api/lms/auth/accounts/{id}/profile` and `/api/lms/auth/accounts/{id}/password` are implemented as authenticated self-service account endpoints. Admin users may update another profile, but password changes remain self-service only.

Apple provider login is registered so FE calls no longer 404, but it returns `501 Not Implemented` until Apple identity verification is configured.

The canonical auth API remains `/api/auth/*`; the LMS aliases exist to unblock the current eLearning FE contract.
