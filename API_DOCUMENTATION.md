# Online Judge System - API Documentation

## Overview

REGS (Rapid Evaluation and Grading System) is a containerized online judge system built with Go (Gin framework) and Docker. It supports multiple judge types: traditional stdio input/output and Catch2 multi-test framework.

**Base URL:** `http://localhost:8080/api`

---

## Authentication

All protected endpoints require a JWT token in the `Authorization` header:

```bash
Authorization: Bearer <token>
```

Tokens are obtained via login and are valid for the session.

---

## User Management

### Register New User
- **Endpoint:** `POST /users/register`
- **Auth:** None (guest)
- **Request:**
```json
{
  "username": "alice",
  "password": "secure123"
}
```
- **Response (201):**
```json
{
  "id": 2,
  "username": "alice",
  "role": "user"
}
```
- **Errors:**
  - `400`: Username already exists
  - `400`: Invalid request format

### Login
- **Endpoint:** `POST /users/login`
- **Auth:** None (guest)
- **Request:**
```json
{
  "username": "alice",
  "password": "secure123"
}
```
- **Response (200):**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```
- **Errors:**
  - `401`: Invalid credentials

### Get Current User Profile
- **Endpoint:** `GET /users/me`
- **Auth:** Required (user, admin)
- **Response (200):**
```json
{
  "id": 2,
  "username": "alice",
  "role": "user"
}
```

### Logout
- **Endpoint:** `POST /users/logout`
- **Auth:** Required (user, admin)
- **Response (200):**
```json
{
  "status": "logged out"
}
```

### List All Users (Admin Only)
- **Endpoint:** `GET /users`
- **Auth:** Required (admin only)
- **Response (200):**
```json
[
  {
    "ID": 1,
    "Username": "admin",
    "Role": "admin"
  },
  {
    "ID": 2,
    "Username": "alice",
    "Role": "user"
  }
]
```
- **Errors:**
  - `403`: User is not admin

### Promote User to Admin
- **Endpoint:** `POST /users/promote`
- **Auth:** Required (admin only)
- **Request:**
```json
{
  "username": "alice"
}
```
- **Response (200):**
```json
{
  "username": "alice",
  "role": "admin"
}
```

### Get User's Submissions (Guest Accessible)
- **Endpoint:** `GET /users/:user_id/submissions`
- **Auth:** None
- **Response (200):**
```json
[
  {
    "operatorId": "550e8400-e29b-41d4-a716-446655440000",
    "problemId": 1,
    "status": "AC",
    "exitCode": 0,
    "createdAt": "2025-01-15T10:30:00Z"
  }
]
```

---

## Problem Management

### List All Problems (Guest Accessible)
- **Endpoint:** `GET /problems`
- **Auth:** None
- **Response (200):**
```json
[
  {
    "id": 1,
    "title": "Add Two Numbers",
    "description": "Read two integers and output their sum",
    "judge_type": "stdio",
    "max_score": 100,
    "testcase_path": "/tmp/problem1",
    "created_at": "2025-01-15T10:00:00Z",
    "updated_at": "2025-01-15T10:00:00Z"
  },
  {
    "id": 2,
    "title": "Algorithm Challenge",
    "description": "Solve the algorithm problem",
    "judge_type": "catch2",
    "max_score": 200,
    "testcase_path": "/tmp/problem2",
    "created_at": "2025-01-15T10:05:00Z",
    "updated_at": "2025-01-15T10:05:00Z"
  }
]
```

### Get Problem Details (Guest Accessible)
- **Endpoint:** `GET /problems/:problem_id`
- **Auth:** None
- **Response (200):**
```json
{
  "id": 1,
  "title": "Add Two Numbers",
  "description": "Read two integers and output their sum",
  "judge_type": "stdio",
  "max_score": 100,
  "testcase_path": "/tmp/problem1"
}
```

### Create or Update Problem (Admin Only)
- **Endpoint:** `PUT /problems`
- **Auth:** Required (admin only)
- **Request (create):**
```json
{
  "title": "New Problem",
  "description": "Problem description",
  "testcase_path": "/path/to/testcases",
  "judge_type": "stdio",
  "max_score": 100
}
```
- **Request (update):**
```json
{
  "id": 1,
  "title": "Updated Problem",
  "description": "Updated description",
  "testcase_path": "/path/to/testcases",
  "judge_type": "stdio"
}
```
- **Response (200):**
```json
{
  "id": 1,
  "title": "Updated Problem",
  "description": "Updated description",
  "judge_type": "stdio",
  "max_score": 100,
  "testcase_path": "/path/to/testcases",
  "created_at": "2025-01-15T10:00:00Z",
  "updated_at": "2025-01-15T10:30:00Z"
}
```

### Delete Problem (Admin Only)
- **Endpoint:** `DELETE /problems/:problem_id`
- **Auth:** Required (admin only)
- **Response:** `204 No Content`

### Download Problem Testcases (Admin Only)
- **Endpoint:** `GET /problems/:problem_id/testcases`
- **Auth:** Required (admin only)
- **Response:** ZIP file attachment

---

## Submission Management

### Submit Code (User Only)
- **Endpoint:** `POST /submissions`
- **Auth:** Required (user)
- **Content-Type:** `multipart/form-data`
- **Fields:**
  - `problem_id` (required): Problem ID
  - `zip` (required): ZIP file containing source code
- **Response (201):**
```json
{
  "operatorId": "550e8400-e29b-41d4-a716-446655440000",
  "status": "pending"
}
```
- **Note:** Returns immediately (non-blocking). Judging happens asynchronously in background.

### List User's Submissions (User Only)
- **Endpoint:** `GET /submissions`
- **Auth:** Required (user)
- **Response (200):**
```json
[
  {
    "operatorId": "550e8400-e29b-41d4-a716-446655440000",
    "problemId": 1,
    "status": "AC",
    "exitCode": 0,
    "createdAt": "2025-01-15T10:30:00Z"
  },
  {
    "operatorId": "660e8400-e29b-41d4-a716-446655440001",
    "problemId": 2,
    "status": "WA",
    "exitCode": 0,
    "createdAt": "2025-01-15T10:35:00Z"
  }
]
```

### Get Submission Details
- **Endpoint:** `GET /submissions/:operatorId`
- **Auth:** Required (user/admin; user can only see own submissions)
- **Response (200):**
```json
{
  "operator_id": "550e8400-e29b-41d4-a716-446655440000",
  "user_id": 2,
  "problem_id": 1,
  "status": "AC",
  "exit_code": 0,
  "score": 0,
  "source_path": "uploads/550e8400-e29b-41d4-a716-446655440000.zip",
  "created_at": "2025-01-15T10:30:00Z"
}
```
- **Status Codes:**
  - `AC` - Accepted (correct output)
  - `WA` - Wrong Answer
  - `CE` - Compile Error
  - `RE` - Runtime Error
  - `TLE` - Time Limit Exceeded
  - `SE` - System Error
  - `PA` - Partial Accept (Catch2 mode only)

### Download Submission Source
- **Endpoint:** `GET /submissions/:operatorId/source`
- **Auth:** Required (user/admin; user can only download own)
- **Response:** ZIP file attachment

---

## Statistics API

### Problem Statistics (Guest Accessible)
- **Endpoint:** `GET /stats/problems/:problem_id`
- **Auth:** None
- **Response (200):**
```json
{
  "problem_id": 1,
  "total_submissions": 15,
  "accepted_count": 10,
  "wrong_answer_count": 3,
  "compile_error_count": 1,
  "runtime_error_count": 1
}
```

### User Statistics (Guest Accessible)
- **Endpoint:** `GET /stats/users/:user_id`
- **Auth:** None
- **Response (200):**
```json
{
  "user_id": 2,
  "total_submissions": 5,
  "ac_count": 3,
  "wa_count": 1,
  "ce_count": 1
}
```

---

## Error Responses

All errors follow this format:

```json
{
  "error": "error message"
}
```

### HTTP Status Codes
- `200` - Success
- `201` - Created
- `204` - No Content
- `400` - Bad Request (validation error)
- `401` - Unauthorized (invalid/missing token)
- `403` - Forbidden (insufficient permissions)
- `404` - Not Found
- `500` - Internal Server Error

---

## Judge Types

### stdio (Traditional Input/Output)
- Reads input from `input.txt`
- Expects output in `answer.txt`
- Script: `/workspace/judge.sh`
- Status: AC, WA, CE, RE, TLE, SE

### catch2 (Catch2 Framework)
- Multi-test framework with templates
- Template directory: `/problem/template/`
- Scores file: `/problem/scores.txt`
- Script: `/workspace/judge_catch2.sh`
- Status: AC, WA, PA (partial), CE, RE, TLE, SE
- Output: `SCORED X/Y` format

---

## Background Queue System

### Architecture
- **Workers:** 4 concurrent worker goroutines
- **Queue Size:** 100 tasks (buffered channel)
- **Processing:** Non-blocking, async
- **Timeout:** Container hard timeout enforcement

### Flow
1. User submits code via `POST /api/submissions`
2. File saved to `uploads/{operatorId}.zip`
3. Task enqueued immediately, returns operatorId
4. Worker picks up task from queue
5. Docker container started with constraints:
   - CPU: 1 core
   - Memory: 512 MB
   - Network: isolated
6. Judge script executes
7. Results written to database
8. Next task dequeued

---

## Default Credentials

```
Username: admin
Password: admin123
Role: admin
```

**Important:** Change these credentials in production!

---

## Database Schema

### Users
```sql
CREATE TABLE users (
  id INTEGER PRIMARY KEY,
  username TEXT UNIQUE NOT NULL,
  password TEXT NOT NULL,
  role VARCHAR(20) NOT NULL
);
```

### Problems
```sql
CREATE TABLE problems (
  id INTEGER PRIMARY KEY,
  title TEXT UNIQUE NOT NULL,
  description TEXT NOT NULL,
  testcase_path TEXT,
  judge_type VARCHAR(20) NOT NULL DEFAULT 'stdio',
  max_score INTEGER DEFAULT 0,
  created_at TIMESTAMP,
  updated_at TIMESTAMP
);
```

### Submissions
```sql
CREATE TABLE submissions (
  operator_id VARCHAR(36) PRIMARY KEY,
  user_id INTEGER NOT NULL,
  problem_id INTEGER NOT NULL,
  status VARCHAR(50) NOT NULL,
  exit_code INTEGER,
  score INTEGER DEFAULT 0,
  source_path TEXT NOT NULL,
  created_at TIMESTAMP
);
```

---

## Rate Limiting & Constraints

- **Queue:** 100 concurrent tasks
- **Workers:** 4 parallel judges
- **Container Timeout:** Problem-specific (default 5s)
- **Memory:** 512 MB per container
- **CPU:** 1 core per container
- **Network:** Disabled (isolated)

---

## Development

### Local Setup
```bash
# Clone repo
git clone <repo>
cd regs

# Build Docker image
docker build -t regs .

# Run container
docker run -p 8080:8080 regs

# Run tests
pytest tests/ -v
```

### Running Tests
```bash
# All tests
pytest tests/ -v

# Specific test
pytest tests/test_pipeline_and_db.py::TestRun::test_run_returns_tle_when_program_exceeds_time_limit -v

# With coverage
pytest tests/ --cov=.
```

---

## Troubleshooting

### Container won't start
```bash
docker logs regs_server
```

### Database locked
```bash
docker rm -f regs_server
rm -f data/regs.db
docker run -p 8080:8080 regs
```

### Queue not processing
Check container logs for `[Queue] Worker` messages

### Submission stuck on pending
Check Docker daemon is running:
```bash
docker ps
```

---

## License

Internal use only.
