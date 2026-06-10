# REGS Architecture & Design

## System Overview

```
┌─────────────────────────────────────────────────────────────┐
│                       Client (Web/CLI)                      │
└────────────────────────┬────────────────────────────────────┘
                         │ HTTP/REST
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                    Gin Web Framework                        │
│  ┌──────────────────────────────────────────────────────┐  │
│  │ POST /users/register         (auth)                 │  │
│  │ POST /users/login            (auth)                 │  │
│  │ POST /submissions            (submit code)          │  │
│  │ GET  /problems               (list problems)        │  │
│  │ PUT  /problems               (admin create)         │  │
│  │ GET  /stats/*                (statistics)           │  │
│  └──────────────────────────────────────────────────────┘  │
│                         ▼                                   │
│         ┌──────────────────────────────┐                   │
│         │  Middleware & RBAC           │                   │
│         │ • JWT Authentication         │                   │
│         │ • Permission Checks          │                   │
│         │ • Role Validation            │                   │
│         └──────────────────────────────┘                   │
│                         ▼                                   │
│    ┌────────────────────────────────────────┐              │
│    │  Background Task Queue                 │              │
│    │ ┌──────────────────────────────────┐   │              │
│    │ │ Worker 1: judge task             │   │              │
│    │ │ Worker 2: judge task             │   │              │
│    │ │ Worker 3: judge task             │   │              │
│    │ │ Worker 4: judge task             │   │              │
│    │ └──────────────────────────────────┘   │              │
│    │ (4 concurrent, 100 task buffer)        │              │
│    └────────────────────────────────────────┘              │
│                         │                                   │
│                         ▼                                   │
│    ┌────────────────────────────────────────┐              │
│    │  Docker Container (Per Judge Job)      │              │
│    │ ┌──────────────────────────────────┐   │              │
│    │ │ Mount: /studentcode (RW)         │   │              │
│    │ │ Mount: /problem (RO)             │   │              │
│    │ │ Limit: 1 CPU, 512 MB RAM         │   │              │
│    │ │ Network: Disabled (isolated)     │   │              │
│    │ │                                  │   │              │
│    │ │ Runs: judge.sh or judge_catch2.sh    │              │
│    │ └──────────────────────────────────┘   │              │
│    └────────────────────────────────────────┘              │
│                         │                                   │
│                         ▼                                   │
│         ┌──────────────────────────────┐                   │
│         │  Judge Scripts (Bash)        │                   │
│         │ • CMake configure            │                   │
│         │ • Ninja build                │                   │
│         │ • Program execution          │                   │
│         │ • Output comparison          │                   │
│         │ • Status determination       │                   │
│         └──────────────────────────────┘                   │
│                         │                                   │
│                         ▼                                   │
│         ┌──────────────────────────────┐                   │
│         │  Database (SQLite / Postgres)│                   │
│         │ • Users                      │                   │
│         │ • Problems                   │                   │
│         │ • Submissions                │                   │
│         │ • Permissions                │                   │
│         └──────────────────────────────┘                   │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

---

## Component Architecture

### 1. API Layer (`api/`)

**Responsibility:** Handle HTTP requests, validate input, return responses

**Components:**
- `user.go` - User registration, login, profile
- `problem.go` - Problem CRUD
- `submission.go` - Submission upload & retrieval
- `stats.go` - Statistics endpoints

**Flow:**
```
HTTP Request → Gin Router → Middleware (JWT, RBAC) → Handler → Response
```

### 2. Middleware (`middleware/`)

**Responsibility:** Authentication, authorization, permission checks

**Components:**
- `jwt.go` - JWT token validation
- `rbac.go` - Role-based access control
- Permission checks before handler execution

**Permission Model:**
```
User Role → [Permissions] → Action Allowed/Denied
├── guest: (none, guest endpoints only)
├── user: [submission:create, submission:read]
└── admin: [submission:create, submission:read, admin:all]
```

### 3. Service Layer (`service/`)

**Responsibility:** Business logic, Docker orchestration, background processing

**Components:**
- `judge.go` - Judge orchestration
  - Extract student ZIP
  - Resolve problem directory
  - Create Docker container
  - Enforce hard timeout
  - Parse judge output
  - Update database
  
- `queue.go` - Background task queue
  - TaskQueue structure
  - 4 worker goroutines
  - Non-blocking enqueue
  - Task buffering

**Key Features:**
- Non-blocking submission handling
- Parallel judge execution (4 workers)
- Hard container timeout enforcement
- Docker resource limits (CPU, memory, network)

### 4. Model Layer (`model/`)

**Responsibility:** Database models and schemas

**Models:**
- `User` - User account with role
- `Problem` - Problem definition
  - `JudgeType`: stdio or catch2
  - `MaxScore`: For Catch2 grading
  - `TestcasePath`: Input/answer/template files
  
- `Submission` - Submission record
  - `Status`: AC, WA, CE, RE, TLE, SE, PA
  - `Score`: Catch2 mode scoring
  - `SourcePath`: ZIP file location

**Permissions:**
- Permission (RBAC definition)
- RolePermission (role → permission mapping)

### 5. Database Layer (`db/`)

**Responsibility:** Database connection and initialization

**Supports:**
- SQLite (default, local development)
- PostgreSQL (production)

**Migrations:**
- User table with bcrypt password
- Problem table with judge type
- Submission table with status/score
- Permission & RolePermission tables

---

## Data Flow

### Submission Flow (Non-Blocking)

```
1. User POSTs /api/submissions
   ├─ Save ZIP file to uploads/
   ├─ Create DB submission record (status: pending)
   ├─ Enqueue JudgeTask to queue (returns immediately)
   └─ Return operatorId to client
        ↓ (HTTP 201 response)
   Client receives: {"operatorId": "xxx", "status": "pending"}

2. Background Worker picks task from queue
   ├─ Extract student ZIP to /tmp/regs-judge-xxx/
   ├─ Resolve problem directory (stdio or catch2)
   ├─ Create Docker container with:
   │  ├─ CPU limit: 1 core
   │  ├─ Memory limit: 512 MB
   │  ├─ Network: disabled
   │  └─ Mounts: /studentcode (RW), /problem (RO)
   ├─ Start container → runs judge.sh / judge_catch2.sh
   └─ (Client already got response, doesn't wait)

3. Judge Script Execution
   ├─ CMake configure → configure.log
   ├─ Ninja build → compile.log
   ├─ Run program → output.log
   ├─ Compare output vs answer
   └─ Output status: AC/WA/CE/RE/TLE/SE or SCORED X/Y

4. Container Timeout Enforcement
   ├─ Hard timeout: timeLimitSec + 10s grace
   ├─ If container still running → force stop
   ├─ Mark submission as TLE
   └─ Log event

5. Parse & Save Results
   ├─ Read container stdout (judge output)
   ├─ Parse last line: AC / SCORED 50/100 / etc
   ├─ Update DB: status, score (if catch2)
   └─ Log completed

6. Client Polls for Results
   ├─ GET /api/submissions/{operatorId}
   └─ Returns: {"status": "AC", "score": 100, ...}
```

### Judge Type Routing

```
StartJudge(operatorID, zipPath, problemID, timeLimitSec)
    │
    ├─ Load problem from DB
    ├─ Check JudgeType field
    │
    ├─ if JudgeType == "stdio":
    │  │
    │  ├─ Resolve problem dir: input.txt + answer.txt
    │  ├─ Create container
    │  ├─ Cmd: bash /workspace/judge.sh <dir> <input> <answer> <timeout>
    │  └─ Parse: AC/WA/CE/RE/TLE/SE
    │
    └─ else if JudgeType == "catch2":
       │
       ├─ Resolve problem dir: template/ + scores.txt
       ├─ Create container
       ├─ Cmd: bash /workspace/judge_catch2.sh <dir> <template> <scores> <timeout>
       └─ Parse: SCORED X/Y → AC/PA/WA
```

---

## Queue Architecture

### Task Queue System

```
┌─────────────────────────────────────────────────────┐
│           API Handler (Submit)                      │
│  ├─ Save file                                       │
│  ├─ Create DB record                                │
│  └─ Enqueue task → returns immediately             │
│       │                                              │
│       ▼                                              │
│  [Buffered Channel: 100 tasks]                      │
│  ├─ Task 1: problem_id=1, operator_id=xxx          │
│  ├─ Task 2: problem_id=2, operator_id=yyy          │
│  ├─ Task 3: problem_id=1, operator_id=zzz          │
│  └─ ...                                             │
│       │                                              │
│       ├──────────────────┬──────────────────┐       │
│       ▼                  ▼                  ▼       │
│   Worker 1          Worker 2          Worker 3     │
│   ├─ Docker        ├─ Docker        ├─ Docker      │
│   │ Judge 1        │ Judge 2        │ Judge 3      │
│   │ (5s to 10s)    │ (5s to 10s)    │ (5s to 10s)  │
│   │               │               │               │
│   └─ Next task    └─ Next task    └─ Next task    │
│                                                     │
└─────────────────────────────────────────────────────┘
```

**Characteristics:**
- **Non-blocking:** Enqueue returns in <1ms
- **Async:** Judge runs while API handles other requests
- **Parallel:** 4 workers judge concurrently
- **Resilient:** Buffered queue handles bursts
- **Scalable:** Increase worker count in `main.go`

---

## Docker Isolation & Security

### Container Configuration

```go
HostConfig := &container.HostConfig{
    NetworkMode: "none",           // No network access
    Binds: []string{
        workspace + ":/studentcode",   // Student code (RW)
        problemDir + ":/problem:ro",   // Problem data (RO)
    },
    Resources: container.Resources{
        NanoCPUs: 1_000_000_000,         // 1 CPU core
        Memory: 512 * 1024 * 1024,       // 512 MB
    },
}
```

**Security:**
- No network access (isolated)
- Read-only problem files (can't modify testcases)
- Resource limits (prevent fork bombs, infinite memory)
- Separate container per submission (no cross-contamination)
- Forced termination after timeout

---

## Judge Output Format

### stdio Mode
Judge script outputs status on final line:
```
AC    # Accepted (output matches exactly)
WA    # Wrong Answer (output mismatch)
CE    # Compile Error
RE    # Runtime Error (crash, segfault, nonzero exit)
TLE   # Time Limit Exceeded
SE    # System Error (missing files, etc)
```

### catch2 Mode
Judge script outputs scoring format:
```
SCORED 85/100    # Student scored 85 out of 100
```

Parsed as:
- `got = 85`, `max = 100`
- Status: AC if `got >= max`, PA if `got > 0`, WA if `got == 0`

---

## Performance Characteristics

| Metric | Value | Notes |
|--------|-------|-------|
| Request latency | <100ms | Just enqueue + DB insert |
| Judge latency | 5-30s | Depends on code complexity |
| Queue capacity | 100 tasks | Buffered channel |
| Worker throughput | 4 concurrent | Per 5-30s judge time |
| Container startup | ~1s | Docker overhead |
| DB queries/sec | ~1000 | SQLite typical limit |
| Memory per container | 512 MB | Hard limit |
| CPU per container | 1 core | Hard limit |

---

## Extensibility

### Adding New Judge Type

1. Add constant to `model.JudgeType`:
```go
const JudgeTypeCustom JudgeType = "custom"
```

2. Create judge script: `judge_custom.sh`

3. Add routing in `service/judge.go`:
```go
case model.JudgeTypeCustom:
    // Mount custom directories
    // Set custom Cmd
```

4. Update API documentation

### Adding New Permission

1. Update `model.SeedDefaults()`:
```go
perms := []string{"...", "new:permission"}
```

2. Assign to role in `roleMap`:
```go
RoleAdmin: {"...", "new:permission"}
```

### Adding New Statistics

1. Create handler in `api/stats.go`
2. Add route in `main.go`
3. Query DB for metrics

---

## Monitoring & Observability

### Metrics Available

```bash
# Queue status
docker logs regs | grep "\[Queue\]"

# Worker activity
docker logs regs | grep "Worker.*processing"

# Judge results
docker logs regs | grep "\[Judge\].*→"

# Container stats
docker stats regs

# Database size
docker exec regs du -sh /app/data/regs.db
```

### Logging Points

- `[Queue]` - Queue initialization and task enqueue
- `[Judge]` - Judge startup, container creation, timeout
- Middleware - Authentication/authorization events
- Database - Migration, seeding

---

## Future Enhancements

1. **Distributed Queue** (Redis/RabbitMQ)
2. **Kubernetes Deployment**
3. **WebSocket for Real-time Results**
4. **Email Notifications**
5. **Code Plagiarism Detection**
6. **Leaderboard & Rankings**
7. **Submission History & Replay**
8. **Mobile App**
9. **IDE Integration**
10. **Custom Judge Templates**

---

## License

Internal use only.
