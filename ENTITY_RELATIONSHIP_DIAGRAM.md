# REGS - Entity Relationship Diagram (ERD)

## Database Schema Visualization

```
┌─────────────────────────────────────────────────────────────────┐
│                                                                 │
│                     REGS DATABASE SCHEMA                        │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘


                            ┌──────────────────┐
                            │      Users       │
                            └──────────────────┘
                                   │ (1)
                                   │
                    ┌──────────────┼──────────────┐
                    │              │              │
                    │ (1)          │ (1)          │ (N)
                    ▼              ▼              ▼
            ┌──────────────┐  ┌──────────────┐  ┌──────────────┐
            │Permissions  │  │RolePermission│  │Submissions   │
            └──────────────┘  └──────────────┘  └──────────────┘
                  (N)              │ (N)              │ (N)
                  │                │                 │
                  │                └─────────┬───────┘
                  │                          │
                  │                    (N)   ▼
                  │                   ┌──────────────┐
                  └──────────────────▶│   Problems   │
                        (N)           └──────────────┘


┌──────────────────────────────────────────────────────────────────────────────┐
│                                                                              │
│                            TABLE DETAILS                                     │
│                                                                              │
└──────────────────────────────────────────────────────────────────────────────┘


╔═══════════════════════════════════════════════════════════════════════════╗
║                              USERS TABLE                                   ║
╠═══════════════════════════════════════════════════════════════════════════╣
║ PK  id            INTEGER          PRIMARY KEY                            ║
║ UQ  username      TEXT             UNIQUE NOT NULL                        ║
║     password      TEXT             NOT NULL (bcrypt hash)                 ║
║     role          VARCHAR(20)      NOT NULL (guest/user/admin)            ║
║                                                                            ║
║ Relationships:                                                             ║
║   - (1:N) One User has many Submissions                                   ║
║                                                                            ║
║ Constraints:                                                               ║
║   - Username must be unique                                               ║
║   - Role restricted to: guest, user, admin                                ║
║                                                                            ║
╚═══════════════════════════════════════════════════════════════════════════╝


╔═══════════════════════════════════════════════════════════════════════════╗
║                            PROBLEMS TABLE                                  ║
╠═══════════════════════════════════════════════════════════════════════════╣
║ PK  id              INTEGER         PRIMARY KEY                           ║
║ UQ  title           TEXT            UNIQUE NOT NULL                       ║
║     description     TEXT            NOT NULL                              ║
║     testcase_path   TEXT            (path to zip or directory)            ║
║     judge_type      VARCHAR(20)     NOT NULL DEFAULT 'stdio'              ║
║     max_score       INTEGER         DEFAULT 0 (for catch2 mode)           ║
║     created_at      TIMESTAMP       Automatically set                     ║
║     updated_at      TIMESTAMP       Automatically updated                 ║
║                                                                            ║
║ Relationships:                                                             ║
║   - (1:N) One Problem has many Submissions                                ║
║                                                                            ║
║ Constraints:                                                               ║
║   - Title must be unique                                                  ║
║   - JudgeType restricted to: stdio, catch2                                ║
║   - MaxScore >= 0                                                         ║
║                                                                            ║
╚═══════════════════════════════════════════════════════════════════════════╝


╔═══════════════════════════════════════════════════════════════════════════╗
║                          SUBMISSIONS TABLE                                 ║
╠═══════════════════════════════════════════════════════════════════════════╣
║ PK  operator_id     VARCHAR(36)     PRIMARY KEY (UUID)                    ║
║ FK  user_id         INTEGER         Foreign Key → Users(id)               ║
║ FK  problem_id      INTEGER         Foreign Key → Problems(id)            ║
║     status          VARCHAR(50)     NOT NULL (AC/WA/CE/RE/TLE/SE/PA)     ║
║     exit_code       INTEGER         Program exit code                     ║
║     score           INTEGER         DEFAULT 0 (catch2 mode score)         ║
║     source_path     TEXT            NOT NULL (path to zip file)           ║
║     created_at      TIMESTAMP       Automatically set                     ║
║                                                                            ║
║ Relationships:                                                             ║
║   - (N:1) Many Submissions belong to one User                             ║
║   - (N:1) Many Submissions belong to one Problem                          ║
║   - Cascade delete on User/Problem deletion                               ║
║                                                                            ║
║ Constraints:                                                               ║
║   - Status restricted to: AC, WA, CE, RE, TLE, SE, PA                    ║
║   - OperatorID is globally unique                                         ║
║   - Score >= 0                                                            ║
║                                                                            ║
║ Status Meanings:                                                           ║
║   AC  = Accepted (correct answer)                                         ║
║   WA  = Wrong Answer                                                      ║
║   CE  = Compile Error                                                     ║
║   RE  = Runtime Error                                                     ║
║   TLE = Time Limit Exceeded                                               ║
║   SE  = System Error                                                      ║
║   PA  = Partial Accept (Catch2 mode only)                                 ║
║                                                                            ║
╚═══════════════════════════════════════════════════════════════════════════╝


╔═══════════════════════════════════════════════════════════════════════════╗
║                        PERMISSIONS TABLE (RBAC)                            ║
╠═══════════════════════════════════════════════════════════════════════════╣
║ PK  id              INTEGER         PRIMARY KEY                           ║
║ UQ  name            TEXT            UNIQUE NOT NULL                       ║
║                                                                            ║
║ Predefined Permissions:                                                    ║
║   - submission:create   = Can submit code                                 ║
║   - submission:read     = Can view submissions                            ║
║   - admin:all           = All admin actions                               ║
║                                                                            ║
║ Relationships:                                                             ║
║   - (1:N) One Permission has many RolePermissions                         ║
║                                                                            ║
╚═══════════════════════════════════════════════════════════════════════════╝


╔═══════════════════════════════════════════════════════════════════════════╗
║                      ROLEPERMISSION TABLE (RBAC)                           ║
╠═══════════════════════════════════════════════════════════════════════════╣
║ PK  id              INTEGER         PRIMARY KEY                           ║
║     role            VARCHAR(20)     NOT NULL (guest/user/admin)           ║
║ FK  permission_id   INTEGER         Foreign Key → Permissions(id)         ║
║     (idx on role)   INDEX           For fast lookups                      ║
║                                                                            ║
║ Role → Permission Mappings:                                                ║
║   guest  → (none - guest only)                                            ║
║   user   → submission:create, submission:read                             ║
║   admin  → submission:create, submission:read, admin:all                  ║
║                                                                            ║
║ Relationships:                                                             ║
║   - (N:1) Many RolePermissions belong to one Permission                   ║
║   - Cascade delete on Permission deletion                                 ║
║                                                                            ║
║ Constraints:                                                               ║
║   - Role restricted to: guest, user, admin                                ║
║   - (role, permission_id) is functionally unique                          ║
║                                                                            ║
╚═══════════════════════════════════════════════════════════════════════════╝


┌──────────────────────────────────────────────────────────────────────────────┐
│                          RELATIONSHIP SUMMARY                                │
└──────────────────────────────────────────────────────────────────────────────┘

Users (1) ─────────► (N) Submissions
  │
  │ (1)
  └─────────► (N) RolePermission ─────────► (N) Permissions
  
Submissions (N) ─────────► (1) Problems
  │
  └─ Foreign Key: user_id (ON DELETE SET NULL)
  └─ Foreign Key: problem_id (ON DELETE SET NULL)


┌──────────────────────────────────────────────────────────────────────────────┐
│                          DATABASE STATISTICS                                 │
└──────────────────────────────────────────────────────────────────────────────┘

Tables:              5
  - Users
  - Problems
  - Submissions
  - Permissions
  - RolePermission

Primary Keys:        5
Foreign Keys:        3
  - Submissions.user_id      → Users.id
  - Submissions.problem_id   → Problems.id
  - RolePermission.permission_id → Permissions.id

Unique Constraints:  3
  - Users.username
  - Problems.title
  - Permissions.name

Indexes:             2
  - RolePermission.role (for fast RBAC checks)


┌──────────────────────────────────────────────────────────────────────────────┐
│                          COMMON QUERIES                                      │
└──────────────────────────────────────────────────────────────────────────────┘

-- Get all submissions for a user
SELECT s.* FROM submissions s
JOIN users u ON s.user_id = u.id
WHERE u.id = ?;

-- Get all submissions for a problem
SELECT s.* FROM submissions s
WHERE s.problem_id = ?;

-- Check user permissions (RBAC)
SELECT p.name FROM rolepermission rp
JOIN permissions p ON rp.permission_id = p.id
WHERE rp.role = ? AND p.name = ?;

-- Get all problems created by admins
SELECT * FROM problems WHERE id IN (
  SELECT problem_id FROM submissions
  WHERE user_id IN (SELECT id FROM users WHERE role = 'admin')
);

-- Count submissions by status
SELECT status, COUNT(*) as count FROM submissions
GROUP BY status;

-- Get user stats
SELECT 
  COUNT(*) as total_submissions,
  SUM(CASE WHEN status = 'AC' THEN 1 ELSE 0 END) as ac_count,
  SUM(CASE WHEN status = 'WA' THEN 1 ELSE 0 END) as wa_count
FROM submissions WHERE user_id = ?;


┌──────────────────────────────────────────────────────────────────────────────┐
│                          REFERENTIAL INTEGRITY                               │
└──────────────────────────────────────────────────────────────────────────────┘

Foreign Key Constraints:
  - Submissions.user_id → Users.id
    ├─ ON DELETE: SET NULL (submission remains, user_id becomes null)
    └─ ON UPDATE: CASCADE
  
  - Submissions.problem_id → Problems.id
    ├─ ON DELETE: SET NULL (submission remains, problem_id becomes null)
    └─ ON UPDATE: CASCADE
  
  - RolePermission.permission_id → Permissions.id
    ├─ ON DELETE: CASCADE (remove role-permission mapping)
    └─ ON UPDATE: CASCADE


┌──────────────────────────────────────────────────────────────────────────────┐
│                          NORMALIZATION                                       │
└──────────────────────────────────────────────────────────────────────────────┘

Schema Normalization: 3NF (Third Normal Form)

Users:
  ✓ 1NF: All values are atomic (no repeating groups)
  ✓ 2NF: All non-key attributes depend on full primary key
  ✓ 3NF: No transitive dependencies

Problems:
  ✓ 1NF: All values are atomic
  ✓ 2NF: All non-key attributes depend on id
  ✓ 3NF: No transitive dependencies

Submissions:
  ✓ 1NF: All values are atomic
  ✓ 2NF: All non-key attributes depend on operator_id
  ✓ 3NF: No transitive dependencies

Permissions:
  ✓ 1NF: Simple permission list (atomic)
  ✓ 2NF: Single key dependency
  ✓ 3NF: No transitive dependencies

RolePermission:
  ✓ 1NF: Atomic values
  ✓ 2NF: Multi-valued dependency table (junction table)
  ✓ 3NF: Properly normalized many-to-many relationship


┌──────────────────────────────────────────────────────────────────────────────┐
│                          PERFORMANCE CONSIDERATIONS                          │
└──────────────────────────────────────────────────────────────────────────────┘

Indexed Columns:
  ✓ Users.id (PK - auto indexed)
  ✓ Users.username (UNIQUE - auto indexed)
  ✓ Problems.id (PK - auto indexed)
  ✓ Problems.title (UNIQUE - auto indexed)
  ✓ Submissions.operator_id (PK - auto indexed)
  ✓ Submissions.user_id (FK - recommended)
  ✓ Submissions.problem_id (FK - recommended)
  ✓ RolePermission.role (INDEX - for RBAC lookups)

Query Optimization Tips:
  - Add index on Submissions.user_id for fast user submission lookup
  - Add index on Submissions.problem_id for fast problem submission lookup
  - Add index on Submissions.status for filtering by status
  - Add index on Submissions.created_at for time-range queries

Estimated Query Times (SQLite, 1M submissions):
  - Get user submissions (indexed): ~1ms
  - Get problem submissions (indexed): ~1ms
  - Get user stats (aggregation): ~50ms
  - RBAC permission check (indexed): ~1ms
