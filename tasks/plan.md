# Plan: symbiosis / foundation

Implements [SPEC-foundation.md](../SPEC-foundation.md). Status: awaiting review.

## Approach

Build in vertical slices. Each slice ends with something that runs and is tested, and
each one that touches AWS adds its section to `docs/learn/foundation.md` while the
behaviour is fresh.

The engine is built first against in-memory fakes, with no AWS access. That is the one
piece every later module depends on, so it gets its own review before any real resource
is written.

## Components

| Component | Package | Depends on |
|---|---|---|
| CLI shell | `cmd/symbiosis` | everything below |
| Reconciler engine | `internal/reconcile` | nothing |
| AWS session: profile, region, account pin, assume-role, error classification | `internal/awsx` | nothing |
| State store | `internal/state` | `awsx` |
| Foundation resources: table, bucket, role, budget | `internal/foundation` | `reconcile`, `awsx`, `state` |

## Slices, in order

| # | Slice | Delivers | Needs AWS |
|---|---|---|---|
| 0 | Scaffold | Go module, CLI skeleton with `--help`, lint config, CI for unit tests and lint, licence, README stub, `.gitignore`, `symbiosis.example.yaml` | No |
| 1 | Reconciler engine | `Resource` interface, planner, dependency ordering with cycle detection, apply, reverse-order destroy, plan output. Proven with fake resources, including the "second apply makes zero writes" and "interrupted run converges" cases | No |
| 2 | AWS session and `doctor` | Config loading, named profile, account pin check via `sts:GetCallerIdentity`, tool version checks | Yes (read-only) |
| 3 | First real resource | DynamoDB state table wired through `bootstrap`, `plan` and `destroy`, with an integration test for create → re-apply → destroy | Yes |
| 4 | State recording | Ownership and last applied config written to the table in the same operation that creates a resource | Yes |
| 5 | Artifact bucket | Versioning, encryption, public access block, and emptying all versions and delete markers on destroy | Yes |
| 6 | Operator role | Role, trust policy, permission policy, and assume-role for every non-bootstrap command | Yes |
| 7 | Budget alert | Monthly budget with 50/80/100% email alerts | Yes |
| 8 | `status` and teardown proof | `status` command, post-destroy sweep by tag and name prefix, real interrupted-run test | Yes |
| 9 | Wrap-up | Learning write-up completed, decision records, README usage, every success criterion checked off | No |

**Sequential:** 0 → 1 → 2 → 3 → 4, then 8 → 9 at the end.
**Independent of each other:** 5, 6 and 7, once 4 is done.

## Review checkpoints

| After slice | You review | Why here |
|---|---|---|
| 1 | The `Resource` interface and plan output | Every later module is written against this interface, so changing it later is expensive |
| 3 | The first real bootstrap and destroy in your account | First time anything is created in AWS; confirms the account pin and teardown work |
| 6 | The operator role's trust and permission policies | The security boundary for everything that follows |
| 9 | The module against the 11 success criteria | Gate before the `network` spec is written |

## Decisions to record as ADRs

1. **AWS is the source of truth; the state table records ownership only.** Avoids the
   state-drift problems of tools that trust a state file.
2. **Bootstrap resources are found by deterministic name.** The state table cannot
   record its own creation.
3. **`bootstrap` and `destroy` run with the administrator profile; everything else runs
   as the operator role.** A role cannot sensibly create or delete itself.
4. **Operator role trust.** Trusted principal is the account, narrowed by a condition to
   the specific identity that ran bootstrap.
5. **No emulators in tests.**

## Risks

| Risk | Mitigation |
|---|---|
| IAM is eventually consistent: a new role often cannot be assumed for several seconds | Bounded retry with backoff in `awsx`, classified by error code; covered in the learning write-up |
| DynamoDB table creation is asynchronous | Wait for `ACTIVE` before the resource reports as applied |
| A versioned bucket cannot be deleted until every version and delete marker is gone | Paginated purge in the bucket's `Delete`, with an integration test using a non-empty bucket |
| The Budgets API is account-level and served from a global endpoint, unlike the regional services | Separate client configuration; noted in the learning write-up |
| Integration tests leak resources or cost money | Unique suffix per run, cleanup registered before creation, a sweep at the end, and nothing in this module bills by the hour |
| The engine grows into a general-purpose framework | It supports only what the four foundation resources need. Extensions wait for the module that needs them |
| The `Resource` interface turns out wrong for later modules | Checkpoint after slice 1; the interface is sketched against one `network` and one `compute` resource on paper before it is fixed |
| Running against the wrong account | Account pin checked before any write, with a test for the mismatch case |

## Needed from you before slice 2

Slices 0 and 1 need only Go installed. From slice 2 onward:

- Go, Docker and the AWS CLI installed
- A dedicated AWS account
- A named profile with short-lived administrator credentials for bootstrap
- A project folder on this machine, and the private GitHub repo `symbiosis`

## Next phase

Once this plan is approved, it is broken into tasks in `tasks/todo.md`, each with
acceptance criteria, a verification step and the files it touches.
