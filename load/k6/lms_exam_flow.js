import http from "k6/http";
import { check, group, sleep } from "k6";
import exec from "k6/execution";
import { SharedArray } from "k6/data";
import { Counter, Rate, Trend } from "k6/metrics";

export const errors = new Counter("erg_lms_errors");
export const successRate = new Rate("erg_lms_success_rate");
export const attemptLatency = new Trend("erg_lms_attempt_latency");
export const answerLatency = new Trend("erg_lms_answer_latency");
export const submitLatency = new Trend("erg_lms_submit_latency");

const baseURL = (__ENV.BASE_URL || "http://localhost:8080").replace(/\/$/, "");
const tenantID = __ENV.TENANT_ID || "default";
const staticStudentToken = __ENV.AUTH_TOKEN || __ENV.STUDENT_TOKEN || "";
const staticTeacherToken = __ENV.TEACHER_TOKEN || staticStudentToken;
const quizID = __ENV.QUIZ_ID || "";
const assignmentID = __ENV.ASSIGNMENT_ID || "";
const classID = __ENV.CLASS_ID || "";
const packageID = __ENV.PACKAGE_ID || quizID || "load-package";
const packageHashFallback = __ENV.PACKAGE_HASH || "load-package-hash";
const questionsPerAttempt = Number(__ENV.QUESTIONS_PER_ATTEMPT || 5);
const thinkTimeSeconds = Number(__ENV.THINK_TIME_SECONDS || 1);
const loginPath = __ENV.LOGIN_PATH || "/api/lms/auth/login";
const teacherOverviewVUs = Number(__ENV.TEACHER_OVERVIEW_VUS || 1);

const credentials = new SharedArray("erg-lms-credentials", () => {
  const path = __ENV.CREDENTIALS_FILE || "";
  if (!path) {
    return [];
  }
  return open(path)
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter((line) => line && !line.startsWith("#"))
    .map((line) => {
      const [email, password] = line.split(",").map((part) => part.trim());
      return { email, password };
    })
    .filter((item) => item.email && item.password);
});

function buildScenarios() {
  const scenarios = {
    lms_exam_flow: {
      executor: "ramping-vus",
      exec: "studentExamFlow",
      stages: [
        { duration: __ENV.RAMP_DURATION || "2m", target: Number(__ENV.RAMP_VUS || 10) },
        { duration: __ENV.STEADY_DURATION || "5m", target: Number(__ENV.STEADY_VUS || 10) },
        { duration: __ENV.RAMP_DOWN_DURATION || "1m", target: 0 },
      ],
      gracefulRampDown: "60s",
    },
  };
  if (__ENV.LOGIN_EACH_ITERATION === "true" && credentials.length > 0) {
    scenarios.auth_login = {
      executor: "constant-vus",
      exec: "authLogin",
      vus: Number(__ENV.AUTH_VUS || 10),
      duration: __ENV.AUTH_DURATION || "5m",
    };
  }
  if (teacherOverviewVUs > 0 && staticTeacherToken && quizID) {
    scenarios.teacher_overview = {
      executor: "constant-vus",
      exec: "teacherOverview",
      vus: teacherOverviewVUs,
      duration: __ENV.TEACHER_OVERVIEW_DURATION || __ENV.STEADY_DURATION || "5m",
    };
  }
  return scenarios;
}

export const options = {
  scenarios: buildScenarios(),
  thresholds: {
    http_req_failed: ["rate<0.01"],
    http_req_duration: ["p(95)<500", "p(99)<1500"],
    erg_lms_success_rate: ["rate>0.99"],
    erg_lms_answer_latency: ["p(95)<300", "p(99)<1000"],
    erg_lms_submit_latency: ["p(95)<1000", "p(99)<3000"],
  },
};

function headers(token, extra = {}) {
  const h = {
    "Content-Type": "application/json",
    "X-Tenant-ID": tenantID,
    ...extra,
  };
  if (token) {
    h.Authorization = `Bearer ${token}`;
  }
  return h;
}

function dataOf(res) {
  const body = res.json();
  return body?.data || body || {};
}

function pickCredential() {
  if (credentials.length === 0) {
    return null;
  }
  const index = (exec.vu.idInTest - 1) % credentials.length;
  return credentials[index];
}

function login() {
  const cred = pickCredential();
  if (!cred) {
    return "";
  }
  const res = http.post(
    `${baseURL}${loginPath}`,
    JSON.stringify({
      email: cred.email,
      password: cred.password,
      deviceId: `k6-${exec.vu.idInTest}`,
      deviceName: "k6 load generator",
      deviceFingerprint: `k6-${exec.vu.idInTest}-${exec.scenario.iterationInTest}`,
    }),
    { headers: headers("") },
  );
  const ok = check(res, { "login ok": (r) => r.status === 200 });
  successRate.add(ok);
  if (!ok) {
    errors.add(1);
    return "";
  }
  return dataOf(res).accessToken || "";
}

function studentToken() {
  if (staticStudentToken) {
    return staticStudentToken;
  }
  return login();
}

function packageInfo(token) {
  if (!quizID) {
    return { packageHash: packageHashFallback, questionIds: [] };
  }
  const res = http.get(`${baseURL}/api/lms/quizzes/${quizID}/package`, { headers: headers(token) });
  const ok = check(res, { "quiz package ok": (r) => r.status === 200 });
  successRate.add(ok);
  if (!ok) {
    errors.add(1);
    return { packageHash: packageHashFallback, questionIds: [] };
  }
  const payload = dataOf(res);
  const ids = payload?.quiz?.quiz?.questionIds || payload?.quiz?.questionIds || [];
  return {
    packageHash: payload.contentHash || packageHashFallback,
    questionIds: ids,
  };
}

function answerQuestionIds(pkg) {
  const fromEnv = (__ENV.QUESTION_IDS || "")
    .split(",")
    .map((value) => value.trim())
    .filter(Boolean);
  const ids = fromEnv.length > 0 ? fromEnv : pkg.questionIds;
  if (ids.length > 0) {
    return ids.slice(0, questionsPerAttempt);
  }
  return Array.from({ length: questionsPerAttempt }, (_, i) => `load-question-${i}`);
}

export function authLogin() {
  login();
  sleep(thinkTimeSeconds);
}

export function studentExamFlow() {
  const token = studentToken();
  if (!token || !assignmentID || !quizID) {
    errors.add(1);
    successRate.add(false);
    sleep(1);
    return;
  }

  let pkg;
  group("student dashboard", () => {
    http.batch([
      ["GET", `${baseURL}/api/lms/students/me/assignments`, null, { headers: headers(token) }],
      ["GET", `${baseURL}/api/lms/students/me/scores`, null, { headers: headers(token) }],
    ]);
  });

  group("quiz package", () => {
    pkg = packageInfo(token);
  });

  let attemptID = "";
  group("start attempt", () => {
    const start = http.post(
      `${baseURL}/api/lms/attempts`,
      JSON.stringify({
        assignmentId: assignmentID,
        quizId: quizID,
        packageId: packageID,
        packageHash: pkg.packageHash,
      }),
      { headers: headers(token, { "X-Idempotency-Key": `start-${exec.vu.idInTest}` }) },
    );
    attemptLatency.add(start.timings.duration);
    const ok = check(start, { "start attempt ok": (r) => r.status >= 200 && r.status < 300 });
    successRate.add(ok);
    if (!ok) {
      errors.add(1);
      return;
    }
    const body = dataOf(start);
    attemptID = body.id || body.attemptId || "";
  });
  if (!attemptID) {
    errors.add(1);
    sleep(1);
    return;
  }

  const answers = {};
  for (const [i, questionID] of answerQuestionIds(pkg).entries()) {
    group("save answer", () => {
      const answer = `answer-${exec.vu.idInTest}-${exec.scenario.iterationInTest}-${i}`;
      answers[questionID] = answer;
      const res = http.post(
        `${baseURL}/api/lms/attempts/${attemptID}/answers`,
        JSON.stringify({
          questionId: questionID,
          answer,
          clientResult: { source: "k6", sequence: i },
          answeredAt: new Date().toISOString(),
        }),
        { headers: headers(token, { "X-Idempotency-Key": `answer-${attemptID}-${questionID}-${i}` }) },
      );
      answerLatency.add(res.timings.duration);
      const ok = check(res, { "answer saved": (r) => r.status >= 200 && r.status < 300 });
      successRate.add(ok);
      if (!ok) {
        errors.add(1);
      }
    });
    sleep(thinkTimeSeconds);
  }

  group("sync attempt", () => {
    const res = http.post(
      `${baseURL}/api/lms/attempts/${attemptID}/sync`,
      JSON.stringify({
        packageHash: pkg.packageHash,
        quizVersion: String(__ENV.QUIZ_VERSION || "load"),
        attempt: { status: "in_progress" },
        events: [{ type: "sync", at: new Date().toISOString() }],
        client: { source: "k6" },
      }),
      { headers: headers(token, { "X-Idempotency-Key": `sync-${attemptID}` }) },
    );
    const ok = check(res, { "attempt synced": (r) => r.status >= 200 && r.status < 300 });
    successRate.add(ok);
    if (!ok) {
      errors.add(1);
    }
  });

  group("submit attempt", () => {
    const res = http.post(
      `${baseURL}/api/lms/attempts/${attemptID}/submit`,
      JSON.stringify({ answers, submittedAt: new Date().toISOString() }),
      { headers: headers(token, { "X-Idempotency-Key": `submit-${attemptID}` }) },
    );
    submitLatency.add(res.timings.duration);
    const ok = check(res, { "attempt submitted": (r) => r.status >= 200 && r.status < 300 });
    successRate.add(ok);
    if (!ok) {
      errors.add(1);
    }
  });
}

export function teacherOverview() {
  const requests = [
    ["GET", `${baseURL}/api/lms/quizzes/${quizID}/students${classID ? `?classId=${classID}` : ""}`, null, { headers: headers(staticTeacherToken) }],
  ];
  if (assignmentID) {
    requests.push(["GET", `${baseURL}/api/lms/assignments/${assignmentID}/progress`, null, { headers: headers(staticTeacherToken) }]);
  }
  const responses = http.batch(requests);
  for (const res of responses) {
    const ok = res.status >= 200 && res.status < 300;
    successRate.add(ok);
    if (!ok) {
      errors.add(1);
    }
  }
  sleep(thinkTimeSeconds);
}
