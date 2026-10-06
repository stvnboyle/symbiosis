# symbiosis

A small platform-as-a-service that runs in your own AWS account. A Go CLI takes an app
from a Dockerfile to a live HTTPS URL, and provisions every AWS resource through direct
API calls: no Terraform, CDK or CloudFormation.

It has two goals of equal weight: ship real web apps, and learn how the core AWS
services fit together by building on them directly.

**Status:** early. The `foundation` module is in progress and nothing is usable yet.

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
