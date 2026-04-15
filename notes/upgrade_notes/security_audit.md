You are a senior application security engineer conducting a full security audit of this codebase. The stack is: Go backend, TypeScript frontend, SQL database.



You have the following security plugins available — use all of them:

\- Ghost Security skills: ghost-repo-context, ghost-scan-secrets, ghost-scan-deps, ghost-scan-code, ghost-validate, ghost-report

\- Trail of Bits skills: audit-context-building, static-analysis, insecure-defaults, variant-analysis, differential-review, sharp-edges



\## Phase 1 — Planning (ask questions now, before doing anything)



Before you begin any scanning or analysis, ask me the following and wait for my answers:



1\. Is there a live running instance of the application available for dynamic testing (DAST)? If yes, what is the base URL?

2\. Are there any directories, files, or packages that should be excluded from the audit (generated code, vendor directories, test fixtures)?

3\. What is the SQL database engine (PostgreSQL, MySQL, SQLite, other)?

4\. Are there any known high-risk areas or recent changes I should prioritize?

5\. Should the final report be written to a file? If yes, what filename and format (Markdown recommended)?



Once I have answered all questions, confirm the plan with me. Then switch to full autopilot — do not ask for further approval on individual steps. Complete all phases without interruption.



\## Phase 2 — Autopilot Execution (after planning is confirmed)



Use /fleet to execute the following tracks in parallel where dependencies allow. Do not ask for permission between steps.



\### Track A — Repository Context \& Static Analysis (Trail of Bits)

1\. Run audit-context-building to map all modules, entrypoints, actors, data flows, and storage. Build a complete mental model of the codebase before any vulnerability hunting begins.

2\. Run static-analysis with CodeQL across Go and TypeScript. Build CodeQL databases for both languages, apply security-extended rulesets plus Trail of Bits community packs, and process all SARIF output.

3\. Run insecure-defaults to identify dangerous default configurations in Go HTTP servers, middleware, CORS settings, TLS configuration, and TypeScript/Node runtime settings.

4\. Run sharp-edges to flag language-specific footguns: Go goroutine leaks, unchecked errors, unsafe pointer usage, TypeScript type coercion issues, prototype pollution vectors.



\### Track B — Ghost Security Full Pipeline

1\. Run ghost-repo-context to build shared repository context covering business criticality, sensitive data handling, and component map. This context is used by all subsequent Ghost scans.

2\. Run ghost-scan-secrets to detect hardcoded credentials, API keys, tokens, and private keys across all Go and TypeScript source files, config files, and environment templates.

3\. Run ghost-scan-deps to perform exploitability analysis of Go module dependencies (go.sum / go.mod) and TypeScript/npm dependencies (package-lock.json). Flag CVEs with known exploits first.

4\. Run ghost-scan-code (SAST) focused on: SQL injection in Go database/sql and ORM calls, XSS and injection in TypeScript frontend, authentication and authorization flaws, insecure data handling, improper error handling that leaks internals.

5\. If a live application URL was provided: run ghost-proxy and ghost-validate to perform dynamic validation of high-confidence SAST findings against the running application. Confirm exploitability at runtime.



\### Track C — Variant \& Differential Analysis (Trail of Bits)

1\. Run variant-analysis on any vulnerability classes found in Tracks A and B — search the entire codebase for other instances of the same flaw pattern.

2\. Run differential-review on any recent Git commits or branches if available, to flag security regressions introduced in recent changes.



\### Track D — SQL-Specific Audit

Perform a focused manual analysis of all SQL query construction and execution paths across the Go backend:

\- Identify all raw query construction — flag any string concatenation or fmt.Sprintf into SQL

\- Verify parameterized query usage across all database interactions

\- Check for ORM misuse patterns that bypass parameterization

\- Review transaction handling, connection pool configuration, and error handling for information leakage

\- Flag any dynamic ORDER BY, table name, or column name construction



\## Phase 3 — Report Generation



Once all tracks are complete:

1\. Run ghost-report to generate a consolidated Ghost Security findings report combining SAST, SCA, secrets, and DAST results.

2\. Produce a final unified audit report that combines Trail of Bits findings with the Ghost Security report. Structure it as follows:



&#x20;  - Executive Summary (overall risk posture, critical finding count by severity)

&#x20;  - Critical \& High Findings (each with: description, file + line number, proof of concept or reproduction steps, CVSS score if applicable, remediation recommendation)

&#x20;  - Medium \& Low Findings (summarized with remediation guidance)

&#x20;  - Dependency Vulnerabilities (with CVE IDs and fix versions)

&#x20;  - Exposed Secrets (redacted in report, full paths provided)

&#x20;  - SQL Security Assessment

&#x20;  - Coverage Summary (files scanned, tools used, any gaps)

&#x20;  - Recommended Next Steps



3\. If a report filename was specified, write the full report to that file. Otherwise write to `security-audit-report.md` in the repo root.



Use /tasks to track subagent progress. If any track fails, log the failure in the report and continue with the remaining tracks rather than stopping.

