# Spec: symbiosis / foundation

Module id: `foundation` (see [CAPABILITY-MAP.md](CAPABILITY-MAP.md)). Depends on nothing.
Every other module depends on it.

## Objective

symbiosis is my own cloud, built on AWS. `foundation` is its base layer: the part every
cloud platform needs before it can run anything, which is an identity to act as, a
record of what it owns, and an engine that makes real infrastructure match what was
asked for.

This module gives symbiosis a safe, repeatable footing in one AWS account, and builds
the reconciler engine that every later module uses to create, update and delete AWS
resources.

When this module is done, three things are true:

1. `symbiosis bootstrap` turns an empty sandbox account into one that is ready for the
   other modules, and running it again changes nothing.
2. `symbiosis destroy` removes everything symbiosis created.
3. You can explain how the platform's base layer is built, how IAM, STS, DynamoDB and
   S3 work underneath it, and why each is here.

### User stories

- As the operator, I run one command against a fresh account and get a state store, an
  artifact bucket, a least-privilege operator role and a budget alert.
- As the operator, I can preview what any command would change before it changes it.
- As the operator, I cannot accidentally run symbiosis against the wrong AWS account.
- As the operator, I can tear everything down and confirm nothing billable is left.
- As a later module's author, I add a new AWS resource by implementing one small
  interface, and get planning, ordering, idempotency and teardown for free.

### What foundation creates in AWS

| Resource | Service | Purpose |
|---|---|---|
| State table `symbiosis-state` | DynamoDB (on-demand) | Records every resource symbiosis owns and its last applied config |
| Artifact bucket `symbiosis-<account-id>-<region>` | S3 | Later modules store deployment artifacts here. Versioned, encrypted, all public access blocked |
| Operator role `symbiosis-operator` | IAM | The only identity the CLI acts as after bootstrap. Each module adds its permissions to it |
| Monthly cost budget `symbiosis-monthly` | Budgets | Emails you at 50%, 80% and 100% of the ceiling |

### The reconciler

Desired state is declared in Go. The engine does the rest:

1. **Observe**: read each resource's actual state from AWS.
2. **Plan**: diff desired against actual, producing create / update / delete / no-op.
3. **Apply**: execute the plan in dependency order. Destroy runs in reverse order.

Rules the engine guarantees:

- Applying twice in a row makes zero write calls the second time.
- An interrupted run converges when re-run.
- AWS is the source of truth. The state table records ownership and last applied
  config; it is never trusted over what `Observe` returns.
- The bootstrap resources are found by deterministic name, because the state table
  cannot record its own creation.

### Identity model

- `bootstrap` runs once with your administrator profile and creates the operator role.
- Every other command calls `sts:AssumeRole` into `symbiosis-operator` and works with
  those short-lived credentials.
- Every command first calls `sts:GetCallerIdentity` and refuses to continue if the
  account id differs from the one pinned in `symbiosis.yaml`.

## Tech Stack

- Go, current stable release, pinned in `go.mod` (verify the version at scaffold time)
- `github.com/aws/aws-sdk-go-v2` and the per-service clients for IAM, STS, DynamoDB, S3
  and Budgets
- `github.com/spf13/cobra` for the CLI
- `golangci-lint` for linting
- No other runtime dependencies

## Commands

```
Build:        go build -o bin/symbiosis ./cmd/symbiosis
Unit tests:   go test ./... -race -cover
Integration:  SYMBIOSIS_TEST_PROFILE=<profile> go test ./... -tags=integration -timeout 20m
Lint:         golangci-lint run ./...
Vet:          go vet ./...
Format:       gofmt -l -w .
```

CLI surface delivered by this module:

```
symbiosis bootstrap --profile <name> --region <region> --budget <usd> --email <address>
symbiosis plan                # show what would change; makes no write calls
symbiosis status              # list owned resources and whether they match desired state
symbiosis doctor              # check credentials, pinned account, region, tool versions
symbiosis destroy [--yes]     # remove everything, then verify nothing is left
```

## Project Structure

```
cmd/symbiosis/         → CLI entrypoint and command wiring only
internal/reconcile/    → Engine: Resource interface, planner, dependency ordering, apply
internal/state/        → State table read/write
internal/config/       → Loading and validating symbiosis.yaml
internal/testaws/      → Integration test harness: unique suffixes, cleanup, call counting
internal/awsx/         → Client construction, assume-role, tagging, error classification
internal/foundation/   → This module's resources (table, bucket, role, budget)
internal/<module-id>/  → One package per module in the capability map
docs/learn/            → One learning write-up per module
docs/adr/              → Short architecture decision records
tasks/                 → plan.md and todo.md for the module in progress
CAPABILITY-MAP.md      → Index of modules
SPEC-<module-id>.md    → One spec per module
symbiosis.yaml         → Local config: pinned account id, region, profile name, alert email.
                         Git-ignored; symbiosis.example.yaml is committed instead
```

## Code Style

Standard Go: `gofmt`, small packages, errors wrapped with `%w`, `context.Context` as the
first argument of anything that calls AWS. No global clients. Each resource depends on a
narrow interface that lists only the SDK calls it makes, so unit tests use hand-written
fakes and no mocking framework.

```go
// Resource is one AWS thing symbiosis owns. Implementations must be idempotent.
type Resource interface {
	// ID is the stable logical id, for example "foundation/state-table".
	ID() string
	DependsOn() []string
	// Observe reads actual state from AWS. A missing resource is not an error.
	Observe(ctx context.Context) (Actual, error)
	// Diff compares desired config against actual and reports what would change.
	Diff(actual Actual) Change
	Apply(ctx context.Context, change Change) error
	Delete(ctx context.Context) error
}

// tableAPI is the slice of DynamoDB that the state table resource needs.
type tableAPI interface {
	DescribeTable(ctx context.Context, in *dynamodb.DescribeTableInput, opts ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error)
	CreateTable(ctx context.Context, in *dynamodb.CreateTableInput, opts ...func(*dynamodb.Options)) (*dynamodb.CreateTableOutput, error)
	DeleteTable(ctx context.Context, in *dynamodb.DeleteTableInput, opts ...func(*dynamodb.Options)) (*dynamodb.DeleteTableOutput, error)
}
```

Conventions:

- Logical ids are `<module-id>/<kebab-name>`.
- AWS resource names start with `symbiosis-`.
- Every taggable resource carries `symbiosis:managed=true` and `symbiosis:module=<module-id>`.
- IAM policies are written out in full in Go. No wildcard actions, and resources are
  scoped to the `symbiosis-` prefix wherever the service allows it.
- Comments explain why, not what. AWS behaviour that surprised us goes in
  `docs/learn/`, not in code comments.

## Testing Strategy

| Level | What it covers | Where | Runs |
|---|---|---|---|
| Unit | Planner, dependency ordering, diff logic, each resource against a fake | `*_test.go` next to the code | Every change, no AWS access needed |
| Integration | Real bootstrap → re-apply → destroy against the sandbox account | `*_integration_test.go`, build tag `integration` | Before a task is marked done |
| Manual | CLI output reads clearly; budget email arrives | Checklist in `tasks/todo.md` | Once per module |

- No LocalStack or other emulators. The goal is to learn real AWS behaviour, including
  eventual consistency and real error codes.
- Each integration run uses a unique name suffix and deletes what it created, including
  on failure.
- Coverage target: 90% for `internal/reconcile`, 80% elsewhere.
- Tests are written before the code they cover.

## Boundaries

**Always**

- Run unit tests and lint before a task is marked done.
- Tag every resource and record it in the state table in the same operation that creates it.
- Verify the pinned account id before any write call.
- Give every create path a matching delete path and an integration test for both.
- Update the spec first when a decision changes.

**Ask first**

- Adding any Go dependency beyond the Tech Stack list.
- Adding any AWS service not listed for the module in the capability map.
- Creating any resource with an hourly charge.
- Widening the operator role's permissions beyond what the current module needs.
- Running `destroy`, or integration tests, against any account.

**Never**

- Handle, print, log or commit AWS credentials. You configure the profile yourself.
- Use Terraform, CDK, CloudFormation or the AWS CLI to provision resources.
- Use wildcard IAM actions or attach `AdministratorAccess` to anything symbiosis creates.
- Delete a resource that lacks the `symbiosis:managed` tag or the `symbiosis-` name prefix.
- Skip or delete a failing test to get a task through.

## Success Criteria

1. On an account with no symbiosis resources, `symbiosis bootstrap` creates exactly the
   four resources in the table above and exits 0.
2. A second `bootstrap` reports "0 to create, 0 to update, 0 to delete" and makes no
   write API calls. An integration test asserts this by counting calls.
3. `symbiosis plan` makes no write API calls in any state.
4. Killing `bootstrap` part-way and re-running it converges to the same end state.
5. With a different account id pinned in `symbiosis.yaml`, every command exits non-zero
   before making any call other than `sts:GetCallerIdentity`.
6. After bootstrap, non-bootstrap commands run as the assumed `symbiosis-operator` role,
   shown by `symbiosis doctor`.
7. The artifact bucket has versioning on, default encryption on, and all four public
   access block settings on. An integration test reads these back.
8. `symbiosis destroy` removes all four resources, including a non-empty versioned
   bucket, and a follow-up sweep by tag and by name prefix finds nothing.
9. The budget exists at the configured ceiling with alerts at 50%, 80% and 100% addressed
   to the configured email. An integration test reads these back through the API.
10. `go test ./... -race` and `golangci-lint run` pass with the coverage targets met.
11. `docs/learn/foundation.md` exists and explains, with reference to this code: how the
    reconciler works and why a control plane is built around one; how IAM
    evaluates a request, trust policy versus permission policy, how `AssumeRole` and
    request signing work, DynamoDB keys and consistency, and S3 bucket policy versus
    public access block.

## Prerequisites (yours to do; none are installed or set up yet)

- Install Go, Docker and the AWS CLI. The AWS CLI is used only for `aws configure sso`
  and for you to inspect resources while learning.
- Create a dedicated AWS account for symbiosis, separate from anything else you own.
- Set up a named profile with short-lived credentials (IAM Identity Center is the
  recommended route).
- A domain is not needed until the `edge` module.

## Resolved Decisions (approved 2026-10-06)

| Question | Decision |
|---|---|
| Region | `eu-west-2` (London) |
| Budget ceiling | $40 a month |
| Licence | Apache-2.0 |
| Budget alert email | The operator's own address, passed with `--email` and kept in the local, git-ignored `symbiosis.yaml`. It is never committed |
| Repo | Private repo `symbiosis` on the operator's personal GitHub account. Going public is a later, separate decision |
| Audience | Personal use only, no real end users. High availability, multi-tenancy and uptime targets are out of scope |

## Open Questions

None for this module.

### Cost note

Approximate costs to confirm against current AWS pricing when the `edge` spec is written:

| State | Rough monthly cost | What drives it |
|---|---|---|
| After `foundation` only | Under $1 | On-demand DynamoDB and a near-empty bucket |
| v1 running one small app | About $30 | Load balancer (~$18), one small Fargate task (~$9), public IPv4 addresses, hosted zone |
| After `destroy` | $0 | A Route 53 hosted zone (~$0.50) if you keep the domain's zone |

A NAT gateway would add roughly $32 a month on its own, which is why v1 uses public
subnets with strict security groups.
