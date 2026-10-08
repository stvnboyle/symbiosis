# Capability Map: symbiosis

symbiosis is my own cloud, built on AWS: a platform-as-a-service that runs in my own
AWS account. A Go CLI takes an app from a Dockerfile to a live HTTPS URL, provisioning
every AWS resource through direct API calls (no Terraform, CDK or CloudFormation).

The project is to build a cloud platform, not to work through AWS services one by one.
Each module below is a layer of that platform, and is specified as one.

Goals, in order:

1. **Build my own cloud.** The control plane that turns raw AWS services into "deploy
   my app", designed and written from scratch, and understood at every layer.
2. **Ship real web apps** on it.
3. **Know AWS well enough to discuss and architect infrastructure with confidence**, as
   a result of building on its services directly.

## Guiding constraints

- **Bare minimum services.** A service is in v1 only if a container cannot reach a
  public HTTPS URL safely without it. Everything else waits for v2.
- **Direct API calls.** All provisioning goes through the AWS SDK for Go v2 and
  symbiosis's own reconciler.
- **Cost is a requirement.** Every resource is tagged, `destroy` returns the account to
  zero, and a budget alert exists before anything billable does.
- **Understanding is a deliverable.** Every module ships `docs/learn/<module-id>.md`,
  covering how that layer of the platform is built and how the AWS services under it
  behave.

## v1: the core path (source → HTTPS URL)

| Module id | Responsibility | AWS services | Depends on |
|---|---|---|---|
| `foundation` | Account bootstrap, state store, reconciler engine, tagging, budget alert, teardown | IAM, STS, DynamoDB, S3, Budgets | — |
| `network` | VPC, public subnets across two AZs, routing, security groups | VPC (EC2 networking APIs) | `foundation` |
| `image` | Build the container locally, push it to a private registry | ECR | `foundation` |
| `config` | Per-app env vars and secrets | SSM Parameter Store | `foundation` |
| `compute` | Run and scale containers, task and execution roles | ECS, Fargate, IAM | `network`, `image`, `config` |
| `edge` | Public URLs, TLS, host-based routing to apps | ALB, Route 53, ACM | `network`, `compute` |
| `release` | Immutable deployments, preview URLs, promote, rollback | ECS deployments, DynamoDB | `compute`, `edge` |
| `observe` | Log tailing, basic metrics, health | CloudWatch Logs, CloudWatch Metrics | `compute` |

**Build order:** `foundation` → `network`, `image`, `config` → `compute` → `edge` → `release`, `observe`

**First usable milestone:** end of `edge`. One command puts a container at an HTTPS URL
on your domain.

v1 covers 12 services: IAM, STS, S3, DynamoDB, Budgets, VPC, ECR, SSM Parameter Store,
ECS/Fargate, ALB, Route 53, ACM, plus CloudWatch.

## v2: breadth (each one optional, each one a new set of services)

| Module id | Responsibility | AWS services | Depends on |
|---|---|---|---|
| `data` | Add-ons an app can attach: Postgres, buckets, queues | RDS, S3, SQS | `network`, `config` |
| `cdn` | Static sites and edge caching in front of apps | CloudFront, S3 | `edge` |
| `control-plane` | Hosted API and git-push deploys via GitHub webhook | API Gateway, Lambda, SQS, EventBridge | `release` |
| `private-network` | Private subnets, NAT versus VPC endpoints | NAT Gateway, VPC endpoints | `network` |

v2 modules are specified only when v1 is shipped. They are listed so v1 interfaces do not
block them.

## Decisions already made

| Decision | Choice | Why |
|---|---|---|
| Compute | ECS on Fargate | Runs any Dockerfile, no servers to patch, teaches task/execution roles and awsvpc networking |
| Provisioning | Own reconciler over the SDK | A cloud's control plane is a reconciler; building one is the point. It is what Terraform does underneath |
| Language | Go | Single-binary CLI, the standard language for infrastructure tooling |
| Interface | CLI first | Runs from a laptop with a named AWS profile; hosted control plane is v2 |
| Image builds | Local Docker, push to ECR | Removes CodeBuild from v1 |
| Secrets | SSM Parameter Store SecureString | Free at standard tier; removes Secrets Manager from v1 |
| Networking | Public subnets, no NAT gateway | A NAT gateway is the largest idle cost; tasks are locked down by security groups instead |
| Scope | One account, one region | Multi-account and multi-region are out of scope |

## Rules for this map

- Module ids are stable and never renamed.
- Dependencies point one way. Two modules that need each other are one module.
- The contract between two modules lives in the provider module's spec.
- Each module has its own spec at the repo root: `SPEC-<module-id>.md`.
