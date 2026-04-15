# SQL Security Audit Report — Honeygen Go Backend

**Scope:** All SQL query construction and execution in `backend/internal/`  
**Database:** SQLite via `modernc.org/sqlite`  
**Date:** 2025-07-18

---

## Executive Summary

The backend demonstrates **strong SQL security discipline overall**. All data-bearing queries use parameterized statements (`?` placeholders). However, the audit identified **4 findings** requiring attention and **2 informational items**.

| Severity | Count | Description |
|----------|-------|-------------|
| **High** | 1 | `fmt.Sprintf` in PRAGMA table inspection (production code) |
| **Medium** | 3 | `fmt.Sprintf` for LIMIT/OFFSET instead of parameterized placeholders |
| **Low** | 2 | Raw `err.Error()` forwarded to HTTP clients in deployment handlers |
| **Informational** | 2 | Test-only PRAGMA concatenation; LIKE wildcard prefix disables index |

**Total SQL queries audited:** ~40 across 8 repository/store files + 1 migration file.

---

## Finding 1 — PRAGMA Table Name Injection (High)

| | |
|---|---|
| **File** | `backend/internal/db/migrations.go:357` |
| **Severity** | **High** |
| **CWE** | CWE-89 (SQL Injection) |

### Vulnerable Code

```go
func columnExists(ctx context.Context, tx *sql.Tx, table, column string) (bool, error) {
    rows, err := tx.QueryContext(ctx, fmt.Sprintf(`PRAGMA table_info(%s)`, table))
```

### Risk

The `table` parameter is interpolated via `fmt.Sprintf` without validation. SQLite's `PRAGMA table_info()` does not support `?` placeholders, so string construction is necessary — but the function accepts an unrestricted `string` argument.

### Current Mitigation

All four call sites pass **hardcoded** table names (`"world_models"`, `"generation_jobs"`, `"assets"`, `"events"`). The function is unexported, so external callers cannot reach it. The practical exploitability today is **nil**.

### Why High

Despite being currently safe, the function signature invites future misuse. A single call with a user-derived value would be a critical injection. Defense-in-depth demands a whitelist.

### Recommended Fix

```go
var validMigrationTables = map[string]bool{
    "world_models": true, "generation_jobs": true,
    "assets": true, "events": true, "settings": true, "deployments": true,
}

func columnExists(ctx context.Context, tx *sql.Tx, table, column string) (bool, error) {
    if !validMigrationTables[table] {
        return false, fmt.Errorf("columnExists: invalid table name %q", table)
    }
    rows, err := tx.QueryContext(ctx, fmt.Sprintf(`PRAGMA table_info(%s)`, table))
    // ...
}
```

---

## Finding 2 — LIMIT/OFFSET via fmt.Sprintf (Medium × 3)

| | |
|---|---|
| **Files** | `assets/repository.go:137`, `events/repository.go:134`, `generation/jobs.go:115` |
| **Severity** | **Medium** |
| **CWE** | CWE-89 (SQL Injection — defense-in-depth violation) |

### Vulnerable Code (all three follow the same pattern)

```go
// assets/repository.go:137
query += fmt.Sprintf(" LIMIT %d OFFSET %d",
    normalizeLimit(options.Limit), normalizeOffset(options.Offset))

// events/repository.go:134
query += fmt.Sprintf(" ORDER BY datetime(timestamp) DESC, ... LIMIT %d OFFSET %d",
    normalizeLimit(options.Limit), normalizeOffset(options.Offset))

// generation/jobs.go:115
query += fmt.Sprintf(" ORDER BY datetime(created_at) DESC LIMIT %d OFFSET %d",
    normalizeJobLimit(options.Limit), normalizeJobOffset(options.Offset))
```

### Current Mitigation

Each repository has `normalizeLimit()` (clamped 1–1000, default 100) and `normalizeOffset()` (clamped ≥ 0). The API layer parses values with `strconv.Atoi`, returning a fallback `int` on error. Because Go's `%d` format verb only accepts integers, and the values are already `int` typed, actual SQL injection is **not exploitable** with the current code.

### Why Medium

- Violates the principle of parameterized queries for **all** dynamic values.
- If a future refactor changes the normalization or type, the `fmt.Sprintf` becomes a live injection point.
- SQLite's Go driver fully supports `?` for LIMIT/OFFSET.

### Recommended Fix

```go
// Replace all three instances with:
query += " ORDER BY ... LIMIT ? OFFSET ?"
args = append(args, normalizeLimit(options.Limit), normalizeOffset(options.Offset))
rows, err := r.db.QueryContext(ctx, query, args...)
```

---

## Finding 3 — Error Message Information Leakage (Low)

| | |
|---|---|
| **File** | `backend/internal/api/deployments.go:182, 211` |
| **Severity** | **Low** |

### Code

```go
// Line 182 — deployment start failure
writeError(w, http.StatusInternalServerError, "start_failed", err.Error())

// Line 211 — deployment stop failure
writeError(w, http.StatusInternalServerError, "stop_failed", err.Error())
```

### Risk

`err.Error()` is sent directly to the HTTP client. If the deployment manager encounters a database error, the message may include SQL syntax, table names, or constraint details. All other API handlers in the codebase use **static, human-friendly messages** — these two are the only exceptions.

### Recommended Fix

```go
writeError(w, http.StatusInternalServerError, "start_failed",
    "failed to start deployment")
// Log the real error server-side for debugging
```

---

## Informational Findings

### Info 1 — Test-Only PRAGMA Concatenation

| | |
|---|---|
| **File** | `backend/internal/db/migrations_test.go:367, 400` |

```go
rows, err := db.QueryContext(context.Background(), `PRAGMA table_info(`+table+`)`)
```

Same pattern as Finding 1 but in test helpers. All callers pass hardcoded table names. No production risk, but fixing it for consistency is recommended.

### Info 2 — LIKE Prefix Wildcard Disables Index

| | |
|---|---|
| **File** | `backend/internal/events/repository.go:116-117` |

```go
conditions = append(conditions, "path LIKE ?")
args = append(args, "%"+strings.TrimSpace(options.Path)+"%")
```

The leading `%` prevents SQLite from using any index on the `path` column. This is a **performance** concern, not a security one — the value is properly parameterized. Noted for completeness.

---

## Positive Findings ✅

The following practices were verified across all repository files:

### Parameterized Queries Used Correctly

Every INSERT, UPDATE, SELECT with WHERE, and DELETE uses `?` placeholders:

| Package | File | Queries | All Parameterized? |
|---------|------|---------|-------------------|
| `db` | `settings.go` | GET, PUT | ✅ |
| `db` | `status.go` | 4 SELECT queries | ✅ |
| `db` | `generation_jobs.go` | UPDATE | ✅ |
| `worldmodels` | `repository.go` | List, Get, Create, Update | ✅ |
| `assets` | `repository.go` | Create, Get, FindByPath, List | ✅ (except LIMIT/OFFSET) |
| `events` | `repository.go` | Create, Get, List | ✅ (except LIMIT/OFFSET) |
| `generation` | `jobs.go` | Create, Get, List, Update | ✅ (except LIMIT/OFFSET) |
| `deployments` | `repository.go` | Create, Get, List, ListByStatus, UpdateStatus, Delete | ✅ |

### Dynamic WHERE Construction Is Safe

The pattern used in `assets`, `events`, and `generation` List methods is secure:

```go
conditions = append(conditions, "world_model_id = ?")
args = append(args, options.WorldModelID)
// ...
query += " WHERE " + strings.Join(conditions, " AND ")
rows, err := r.db.QueryContext(ctx, query, args...)
```

The condition strings are **hardcoded constants** — never derived from user input. Only the `args` values come from the request, and they're bound as parameters.

### No Dynamic ORDER BY / Column / Table Names

All ORDER BY clauses are hardcoded:
- `ORDER BY datetime(updated_at) DESC, datetime(created_at) DESC, name ASC` (world models)
- `ORDER BY path ASC, datetime(created_at) DESC` (assets)
- `ORDER BY datetime(timestamp) DESC, datetime(created_at) DESC` (events)
- `ORDER BY datetime(created_at) DESC` (generation jobs)
- `ORDER BY created_at DESC` (deployments)

No user input influences column selection or sort direction.

### No ORM Misuse

The codebase uses `database/sql` directly — no ORM. All queries are explicit SQL strings with `?` placeholders. No query-builder chaining that could bypass parameterization.

### Database Configuration Is Hardened

`backend/internal/db/sqlite.go`:
- `SetMaxOpenConns(1)` — single writer enforced (appropriate for SQLite)
- `PRAGMA foreign_keys = ON` — referential integrity enforced
- `PRAGMA journal_mode = WAL` — crash-safe writes
- `PRAGMA busy_timeout = 5000` — prevents immediate lock failures

### Transaction Handling

`migrations.go` uses `BeginTx` → deferred `Rollback` → `Commit` correctly. Other operations use auto-committed `ExecContext`/`QueryContext`, which is appropriate for single-statement operations with SQLite's single-connection pool.

### API Error Responses Are Opaque

Except for Finding 3 (deployments), all API handlers return **static, generic messages** via `writeError()`. Internal errors are logged but not exposed. Examples:
- `"assets are temporarily unavailable"` (not the SQL error)
- `"world model is temporarily unavailable"` (not the constraint violation)
- `"generation jobs are temporarily unavailable"` (not the DB error)

### ID Generation

Job IDs use `crypto/rand` (via `generation/service_ids.go`). World model IDs are user-provided but parameterized. Deployment IDs use `uuid.New()`. No sequential/guessable identifiers.

### Migration Safety

All DDL is hardcoded constants. Schema upgrades use the safe pattern: check `columnExists()` → `ALTER TABLE ADD COLUMN` only if absent. Legacy table rebuilds use atomic rename sequences within a transaction.

---

## Complete Query Inventory

| # | File | Line | SQL Type | Parameterized | Finding |
|---|------|------|----------|---------------|---------|
| 1 | `db/migrations.go` | 9–94 | CREATE TABLE | Hardcoded | — |
| 2 | `db/migrations.go` | 96–122 | ALTER TABLE (×13) | Hardcoded | — |
| 3 | `db/migrations.go` | 124–132 | CREATE INDEX (×7) | Hardcoded | — |
| 4 | `db/migrations.go` | 171 | UPDATE events | Hardcoded | — |
| 5 | `db/migrations.go` | 190 | INSERT schema_migrations | Hardcoded | — |
| 6 | `db/migrations.go` | 210–349 | Legacy table rebuild | Hardcoded | — |
| 7 | **`db/migrations.go`** | **357** | **PRAGMA table_info** | **fmt.Sprintf** | **Finding 1** |
| 8 | `db/settings.go` | 20 | SELECT settings | ✅ `?` | — |
| 9 | `db/settings.go` | 32 | INSERT/UPSERT settings | ✅ `?` | — |
| 10 | `db/status.go` | 63 | SELECT COUNT assets | Hardcoded | — |
| 11 | `db/status.go` | 71 | SELECT COUNT events | ✅ `?` | — |
| 12 | `db/status.go` | 83 | SELECT latest job | Hardcoded | — |
| 13 | `db/status.go` | 117 | SELECT recent events | ✅ `?` | — |
| 14 | `db/generation_jobs.go` | 19 | UPDATE generation_jobs | ✅ `?` | — |
| 15 | `worldmodels/repository.go` | 31 | SELECT list | Hardcoded | — |
| 16 | `worldmodels/repository.go` | 58 | SELECT by ID | ✅ `?` | — |
| 17 | `worldmodels/repository.go` | 75 | INSERT | ✅ `?` | — |
| 18 | `worldmodels/repository.go` | 89 | UPDATE | ✅ `?` | — |
| 19 | `assets/repository.go` | 63 | INSERT | ✅ `?` | — |
| 20 | `assets/repository.go` | 77 | SELECT by ID | ✅ `?` | — |
| 21 | `assets/repository.go` | 96 | SELECT by path | ✅ `?` | — |
| 22 | **`assets/repository.go`** | **115–137** | **SELECT list** | **Partial** | **Finding 2** |
| 23 | `events/repository.go` | 71 | INSERT | ✅ `?` | — |
| 24 | `events/repository.go` | 86 | SELECT by ID | ✅ `?` | — |
| 25 | **`events/repository.go`** | **104–134** | **SELECT list** | **Partial** | **Finding 2** |
| 26 | `generation/jobs.go` | 73 | INSERT | ✅ `?` | — |
| 27 | `generation/jobs.go` | 83 | SELECT by ID | ✅ `?` | — |
| 28 | **`generation/jobs.go`** | **100–115** | **SELECT list** | **Partial** | **Finding 2** |
| 29 | `generation/jobs.go` | 173 | UPDATE | ✅ `?` | — |
| 30 | `deployments/repository.go` | 54 | INSERT | ✅ `?` | — |
| 31 | `deployments/repository.go` | 65 | SELECT by ID | ✅ `?` | — |
| 32 | `deployments/repository.go` | 82 | SELECT list | Hardcoded | — |
| 33 | `deployments/repository.go` | 107 | SELECT by status | ✅ `?` | — |
| 34 | `deployments/repository.go` | 133 | UPDATE status | ✅ `?` | — |
| 35 | `deployments/repository.go` | 151 | DELETE by ID | ✅ `?` | — |

---

## Remediation Priority

| Priority | Finding | Effort | Impact |
|----------|---------|--------|--------|
| 1 | Finding 1 — PRAGMA whitelist | 15 min | Eliminates injection-by-pattern risk |
| 2 | Finding 2 — Parameterize LIMIT/OFFSET (×3) | 15 min | Full parameterization compliance |
| 3 | Finding 3 — Sanitize deployment error messages | 5 min | Prevents potential info leakage |

All three can be fixed in under an hour with no behavioral changes.
