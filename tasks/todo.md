# Tasks: symbiosis / foundation

Breaks down [plan.md](plan.md), which implements [SPEC-foundation.md](../SPEC-foundation.md).
Status: approved 2026-10-06. In progress.

Rules for every task:

- Tests are written first and fail before the code is written.
- A task is done when its Verify step passes, plus `go test ./... -race`, `go vet ./...`
  and `golangci-lint run ./...`.
- One commit per task, referencing the task id.
- Integration tests run only with your go-ahead, against the sandbox account.

Unit verify commands below are shortened to the package under test.

---

## Slice 0: Scaffold (no AWS)

- [x] **T0.1 Repository and spec commit**
  - Acceptance: `~/symbiosis` is a git repo on `main`; the capability map, spec, plan and this task list are committed; a private GitHub repo `symbiosis` exists as `origin`.
  - Verify: `git log --oneline` shows the commit; `gh repo view --json visibility` reports `PRIVATE`.
  - Files: `CAPABILITY-MAP.md`, `SPEC-foundation.md`, `tasks/plan.md`, `tasks/todo.md`

- [x] **T0.2 Go module and project files**
  - Acceptance: module initialised on the current stable Go; binary builds and prints a version; Apache-2.0 licence; `.gitignore` excludes `bin/` and `symbiosis.yaml`.
  - Verify: `go build -o bin/symbiosis ./cmd/symbiosis && bin/symbiosis --version`
  - Files: `go.mod`, `cmd/symbiosis/main.go`, `.gitignore`, `LICENSE`, `README.md`

- [x] **T0.3 CLI skeleton**
  - Acceptance: root command plus `bootstrap`, `plan`, `status`, `doctor`, `destroy` exist as stubs that return "not implemented"; `--help` lists all five; global `--profile` and `--region` flags parse.
  - Verify: `go test ./cmd/symbiosis/...`
  - Files: `cmd/symbiosis/root.go`, `cmd/symbiosis/root_test.go`, `go.mod`, `go.sum`

- [x] **T0.4 Lint and CI**
  - Acceptance: lint config in place; a GitHub Actions workflow runs build, vet, unit tests and lint on every push; no AWS credentials in CI.
  - Verify: `golangci-lint run ./...` passes locally; the workflow is green on the first push.
  - Files: `.golangci.yml`, `.github/workflows/ci.yml`

## Slice 1: Reconciler engine (no AWS)

- [x] **T1.1 Resource interface and types**
  - Acceptance: `Resource`, `Actual` and `Change` are defined as in the spec; a configurable fake resource exists for tests and counts its read and write calls; ADR 0001 sketches one `network` and one `compute` resource against the interface to show it fits.
  - Verify: `go test ./internal/reconcile/...`
  - Files: `internal/reconcile/resource.go`, `internal/reconcile/fake_test.go`, `docs/adr/0001-resource-interface.md`

- [x] **T1.2 Dependency ordering**
  - Acceptance: resources sort so dependencies come first; the order is deterministic; a cycle, a duplicate id and a dependency on an unknown id each return a clear error.
  - Verify: `go test ./internal/reconcile/... -run Graph`
  - Files: `internal/reconcile/graph.go`, `internal/reconcile/graph_test.go`

- [x] **T1.3 Planner**
  - Acceptance: observes every resource and returns a plan of create, update, delete and no-op entries; planning makes zero write calls; an observe error aborts with the resource id in the message.
  - Verify: `go test ./internal/reconcile/... -run Plan`
  - Files: `internal/reconcile/plan.go`, `internal/reconcile/plan_test.go`

- [x] **T1.4 Apply and destroy**
  - Acceptance: apply runs in dependency order and stops at the first error; destroy runs in reverse order; a second apply makes zero write calls; an apply that fails part-way converges when re-run; context cancellation stops between resources.
  - Verify: `go test ./internal/reconcile/... -run 'Apply|Destroy' -race`
  - Files: `internal/reconcile/apply.go`, `internal/reconcile/apply_test.go`

- [x] **T1.5 Plan output**
  - Acceptance: a plan renders as readable text with one line per resource and a summary line in the form "N to create, N to update, N to delete"; output is stable across runs.
  - Verify: `go test ./internal/reconcile/... -run Render`; coverage for the package is at least 90%.
  - Files: `internal/reconcile/render.go`, `internal/reconcile/render_test.go`

**CHECKPOINT A: you review the `Resource` interface and the plan output.** Passed 2026-10-06.

## Slice 2: AWS session and `doctor` (read-only AWS)

- [ ] **T2.1 Local config**
  - Acceptance: `symbiosis.yaml` loads and validates account id, region, profile and alert email; a missing or malformed file gives an actionable error; an example file is committed and the real one is git-ignored.
  - Verify: `go test ./internal/config/...`
  - Files: `internal/config/config.go`, `internal/config/config_test.go`, `symbiosis.example.yaml`

- [ ] **T2.2 Session and account pin**
  - Acceptance: SDK config loads from the named profile and region; the caller identity is fetched through a narrow STS interface; a pinned account that differs from the caller's returns an error before any other call.
  - Verify: `go test ./internal/awsx/... -run 'Session|Pin'`
  - Files: `internal/awsx/session.go`, `internal/awsx/pin.go`, `internal/awsx/session_test.go`, `internal/awsx/pin_test.go`

- [ ] **T2.3 Error classification and retry**
  - Acceptance: helpers identify not-found, already-exists, access-denied and not-yet-consistent errors by AWS error code; a bounded backoff retry honours context cancellation and retries only errors marked retryable.
  - Verify: `go test ./internal/awsx/... -run 'Error|Retry'`
  - Files: `internal/awsx/errors.go`, `internal/awsx/retry.go`, `internal/awsx/errors_test.go`, `internal/awsx/retry_test.go`

- [ ] **T2.4 `doctor` command**
  - Acceptance: reports the profile, region, caller identity, pin match, and Go, Docker and AWS CLI versions; exits non-zero if any check fails; makes no write calls.
  - Verify: `go test ./cmd/symbiosis/... -run Doctor`, then a manual `bin/symbiosis doctor` against the sandbox account.
  - Files: `cmd/symbiosis/doctor.go`, `cmd/symbiosis/doctor_test.go`

## Slice 3: First real resource

- [ ] **T3.1 State table resource**
  - Acceptance: implements `Resource` for an on-demand DynamoDB table; create waits for `ACTIVE`; standard tags are applied through a shared tag helper; observe treats a missing table as absent, not an error; delete is idempotent.
  - Verify: `go test ./internal/foundation/... -run Table ./internal/awsx/... -run Tags`
  - Files: `internal/foundation/table.go`, `internal/foundation/table_test.go`, `internal/awsx/tags.go`, `internal/awsx/tags_test.go`

- [ ] **T3.2 Wire `bootstrap`, `plan` and `destroy`**
  - Acceptance: the three commands build the foundation resource set and run it through the engine after the account pin check; `destroy` asks for confirmation unless `--yes` is given; `plan` makes no write calls.
  - Verify: `go test ./cmd/symbiosis/... ./internal/foundation/...`
  - Files: `internal/foundation/foundation.go`, `cmd/symbiosis/bootstrap.go`, `cmd/symbiosis/plan.go`, `cmd/symbiosis/destroy.go`, `cmd/symbiosis/commands_test.go`

- [ ] **T3.3 Integration harness and table lifecycle test**
  - Acceptance: a test harness gives each run a unique name suffix, registers cleanup before creating anything, and counts write API calls; the table test proves create, zero-write re-apply and destroy against real AWS; the DynamoDB section of the learning write-up is written.
  - Verify: `SYMBIOSIS_TEST_PROFILE=<profile> go test ./internal/foundation/... -tags=integration -run Table`
  - Files: `internal/testaws/harness.go`, `internal/foundation/table_integration_test.go`, `docs/learn/foundation.md`

**CHECKPOINT B: you run the first real bootstrap and destroy in your account.**

## Slice 4: State recording

- [ ] **T4.1 State store**
  - Acceptance: put, get, list and delete of ownership records keyed by logical id, storing module id, AWS identifier and last applied config; a conditional write prevents two records for one id.
  - Verify: `go test ./internal/state/...`
  - Files: `internal/state/store.go`, `internal/state/store_test.go`

- [ ] **T4.2 Engine records ownership**
  - Acceptance: the engine writes a record after each successful create or update and removes it after delete, through a recorder interface; the state table's own record is written once the table exists; a record is never trusted over what observe returns.
  - Verify: `go test ./internal/reconcile/... ./internal/foundation/...`
  - Files: `internal/reconcile/recorder.go`, `internal/reconcile/apply.go`, `internal/reconcile/apply_test.go`, `internal/foundation/foundation.go`

## Slices 5 to 7: independent of each other

- [ ] **T5.1 Artifact bucket resource**
  - Acceptance: creates the bucket in the configured region with versioning, default encryption, all four public access block settings and tags; diff detects drift in each setting separately and apply corrects only what drifted.
  - Verify: `go test ./internal/foundation/... -run Bucket`
  - Files: `internal/foundation/bucket.go`, `internal/foundation/bucket_test.go`, `internal/foundation/foundation.go`

- [ ] **T5.2 Bucket teardown and integration test**
  - Acceptance: delete purges every object version and delete marker, with pagination, before removing the bucket; the integration test reads back each setting and destroys a non-empty bucket; the S3 section of the learning write-up is written.
  - Verify: `SYMBIOSIS_TEST_PROFILE=<profile> go test ./internal/foundation/... -tags=integration -run Bucket`
  - Files: `internal/foundation/bucket.go`, `internal/foundation/bucket_test.go`, `internal/foundation/bucket_integration_test.go`, `docs/learn/foundation.md`

- [ ] **T6.1 Operator role resource**
  - Acceptance: creates `symbiosis-operator` with a trust policy limited to the identity that ran bootstrap and a permission policy covering only this module's resources; policies are written out in Go; a unit test fails if any statement has a wildcard action or lacks a resource scope where the service supports one.
  - Verify: `go test ./internal/foundation/... -run 'Role|Policy'`
  - Files: `internal/foundation/role.go`, `internal/foundation/policy.go`, `internal/foundation/role_test.go`, `internal/foundation/policy_test.go`, `internal/foundation/foundation.go`

- [ ] **T6.2 Assume the operator role**
  - Acceptance: every command except `bootstrap` and `destroy` assumes the operator role and uses its short-lived credentials; a just-created role that is not yet assumable is retried; `doctor` shows the assumed identity.
  - Verify: `go test ./internal/awsx/... -run Assume ./cmd/symbiosis/...`
  - Files: `internal/awsx/assume.go`, `internal/awsx/assume_test.go`, `cmd/symbiosis/root.go`, `cmd/symbiosis/doctor.go`

- [ ] **T6.3 Role integration test**
  - Acceptance: against real AWS, the role is created and assumed, an in-scope call succeeds and an out-of-scope call is denied; the IAM, STS and request-signing sections of the learning write-up are written; ADR 0004 records the trust decision.
  - Verify: `SYMBIOSIS_TEST_PROFILE=<profile> go test ./internal/foundation/... -tags=integration -run Role`
  - Files: `internal/foundation/role_integration_test.go`, `docs/learn/foundation.md`, `docs/adr/0004-operator-role-trust.md`

**CHECKPOINT C: you review the operator role's trust and permission policies.**

- [ ] **T7.1 Budget alert**
  - Acceptance: creates the monthly cost budget at the configured ceiling with alerts at 50%, 80% and 100% to the configured email; a changed ceiling or email is detected and updated; the integration test reads the budget and its alerts back, then deletes it; the Budgets section of the learning write-up is written.
  - Verify: `go test ./internal/foundation/... -run Budget`, then the same with `-tags=integration`.
  - Files: `internal/foundation/budget.go`, `internal/foundation/budget_test.go`, `internal/foundation/budget_integration_test.go`, `internal/foundation/foundation.go`, `docs/learn/foundation.md`

## Slice 8: `status` and teardown proof

- [ ] **T8.1 `status` command**
  - Acceptance: lists every owned resource with its module, AWS identifier and whether it matches desired state; makes no write calls; runs as the operator role.
  - Verify: `go test ./cmd/symbiosis/... -run Status`
  - Files: `cmd/symbiosis/status.go`, `cmd/symbiosis/status_test.go`

- [ ] **T8.2 Post-destroy sweep**
  - Acceptance: after `destroy`, a sweep lists tables, buckets, roles and budgets by the `symbiosis-` name prefix and by tag where supported; `destroy` exits non-zero and names anything left behind; the sweep never deletes.
  - Verify: `go test ./internal/foundation/... -run Sweep`
  - Files: `internal/foundation/sweep.go`, `internal/foundation/sweep_test.go`, `cmd/symbiosis/destroy.go`

- [ ] **T8.3 Full lifecycle integration test**
  - Acceptance: one test covers bootstrap, zero-write re-apply, a cancelled bootstrap that converges on re-run, the wrong-account refusal, destroy and a clean sweep.
  - Verify: `SYMBIOSIS_TEST_PROFILE=<profile> go test ./... -tags=integration -timeout 20m`
  - Files: `internal/foundation/lifecycle_integration_test.go`

## Slice 9: Wrap-up (no AWS)

- [ ] **T9.1 Decision records**
  - Acceptance: ADRs exist for source of truth, bootstrap resources found by name, which commands use the administrator profile, and no emulators in tests.
  - Verify: each ADR states context, decision and consequences in under a page.
  - Files: `docs/adr/0002-aws-is-source-of-truth.md`, `docs/adr/0003-bootstrap-by-name.md`, `docs/adr/0005-admin-vs-operator.md`, `docs/adr/0006-no-emulators.md`

- [ ] **T9.2 Learning write-up, README and sign-off**
  - Acceptance: the learning write-up covers every topic in success criterion 11 with links to the code; the README covers prerequisites, install, the five commands and costs; all 11 success criteria are checked off with evidence.
  - Verify: walk the success criteria in the spec one by one; coverage targets met.
  - Files: `docs/learn/foundation.md`, `README.md`, `tasks/todo.md`

**CHECKPOINT D: you review the module against the spec. The `network` spec starts only after this.**
