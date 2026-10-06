# ADR 0001: The Resource interface

Status: proposed, for review at Checkpoint A
Date: 2026-10-06

## Context

Every AWS resource symbiosis manages, in every module, is written against one interface
in `internal/reconcile`. Changing it later means touching every resource, so it needs to
fit resources much harder than the four in `foundation` before it is fixed.

## Decision

```go
type Resource interface {
	ID() string
	DependsOn() []string
	Observe(ctx context.Context) (Actual, error)
	Diff(actual Actual) Change
	Apply(ctx context.Context, change Change) error
	Delete(ctx context.Context) error
}
```

- `Observe` is the only read. `Diff` is pure, so planning can never write.
- `Apply` handles both create and update, and blocks until the resource is usable.
- `Delete` succeeds if the resource is already gone.
- The engine observes each resource again immediately before applying it. It does not
  apply a plan computed earlier.

## Does it fit harder resources?

### A subnet (`network`)

A subnet cannot be created without its VPC's id, which only exists after the VPC does.

```go
type Subnet struct {
	api  subnetAPI
	vpc  *VPC      // the Go value of the resource this depends on
	name string
	cidr string
	az   string
}

func (s *Subnet) ID() string          { return "network/subnet-" + s.name }
func (s *Subnet) DependsOn() []string { return []string{s.vpc.ID()} }

func (s *Subnet) Observe(ctx context.Context) (reconcile.Actual, error) {
	// DescribeSubnets filtered by the symbiosis name tag. No match means absent.
}

func (s *Subnet) Apply(ctx context.Context, change reconcile.Change) error {
	vpcID, err := s.vpc.AWSID(ctx) // looks the VPC up by tag; it exists by now
	// CreateSubnet with vpcID, then wait until the subnet is "available".
}
```

It fits. Outputs flow between resources through ordinary Go references plus a lookup
against AWS, not through the engine. Because the engine re-observes just before each
apply, the VPC is guaranteed to exist by the time the subnet asks for its id.

### An ECS service (`compute`)

A service depends on five other resources, updates in place when a new image is
deployed, and takes minutes to become stable.

```go
func (s *Service) DependsOn() []string {
	return []string{s.cluster.ID(), s.taskDef.ID(), s.targetGroup.ID(), s.subnetA.ID(), s.securityGroup.ID()}
}

func (s *Service) Diff(actual reconcile.Actual) reconcile.Change {
	// Absent → create. Task definition or desired count differs → update, with
	// Details like "task definition: web:3 → web:4" and Data naming what differs.
}

func (s *Service) Apply(ctx context.Context, change reconcile.Change) error {
	// create → CreateService; update → UpdateService with only the drifted fields.
	// Then wait for the deployment to reach steady state, honouring ctx.
}
```

It fits. `Change.Data` is what lets `Apply` update only what drifted, and the blocking
`Apply` is what lets a later resource rely on the service being up.

## Consequences

**Good**

- A new resource is one small type. Ordering, planning, idempotency and teardown come
  from the engine.
- Planning cannot write, by construction.
- No stored state to drift out of sync with AWS.

**Costs accepted**

- `Actual.Attrs` and `Change.Data` are `any`. Each resource type-asserts its own data.
  Generics would make this type-safe but would complicate holding mixed resources in
  one list.
- A resource states its dependencies twice: once as a Go reference and once in
  `DependsOn`. Forgetting the second is a bug the engine cannot catch.
- A plan is a preview, not a contract. A resource whose dependency does not exist yet
  is shown as "create" without its final settings.
- Resources are applied one at a time. Slow resources such as a load balancer make a
  run slower than a parallel engine would. Parallelism can be added inside the engine
  later without changing this interface.
- Each lookup of another resource's AWS id is an extra read call. Resources may cache
  it for the length of one run.

## Alternatives considered

- **Engine passes outputs between resources.** More machinery in the engine, and the
  outputs would need the same `any` typing. Rejected for now.
- **Separate `Create` and `Update` methods.** Clearer per method, but every resource
  would repeat the "wait until usable" logic twice. Rejected.
- **Trust a stored state file, as Terraform does.** Faster plans, but drift between
  state and reality is the most common failure of that design. Rejected; see ADR 0002.
