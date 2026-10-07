# symbiosis

Your own cloud platform, running in your own AWS account. A Go CLI takes an app from a
Dockerfile to a live HTTPS URL, and provisions every AWS resource through direct API
calls: no Terraform, CDK or CloudFormation.

It exists for three reasons:

- **To ship real web apps** on infrastructure I control and understand.
- **To learn how the core AWS services fit together**, by calling them directly.
- **To learn how a cloud platform is built on top of AWS**: how products like Heroku
  and Vercel turn raw AWS services into "deploy my app".

**Status:** early. The `foundation` module is in progress and nothing is usable yet.

## What kind of cloud this is

A cloud provider does two jobs. symbiosis does one of them.

| | Who does it here |
|---|---|
| **The control plane:** software that turns "run this app" into the right resources, in the right order, keeps them in the desired state and tears them down cleanly | symbiosis |
| **The infrastructure:** data centres, virtual machines, the physical network, storage | AWS |

That makes symbiosis a personal platform-as-a-service on top of AWS, the same layer
Heroku and Vercel sit at. Using it should feel like having your own cloud: one command,
and your app is live on your own domain, on infrastructure you control and understand.

The core of it is a reconciler: observe what exists in AWS, diff it against what should
exist, and apply the difference. That is the idea behind Terraform, Kubernetes
controllers and AWS's own CloudFormation, built here from scratch to see how it works.

## How the project is organised

- [CAPABILITY-MAP.md](CAPABILITY-MAP.md): the modules, what each depends on, and the build order
- `SPEC-<module-id>.md`: one approved spec per module, starting with [SPEC-foundation.md](SPEC-foundation.md)
- [tasks/plan.md](tasks/plan.md) and [tasks/todo.md](tasks/todo.md): the plan and task list for the module in progress
- `docs/learn/`: a write-up per module of what the AWS services do and why they are wired this way
- `docs/adr/`: short architecture decision records

## Build

```
go build -o bin/symbiosis ./cmd/symbiosis
bin/symbiosis --version
```

## Licence

Apache-2.0. See [LICENSE](LICENSE).
