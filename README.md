# Bare Systems Installer

Bare Systems Installer is the customer-facing CLI for Bare Systems edge deployments. The public command is `bare-systems`.

The CLI is responsible for installation workflows, local configuration, lifecycle orchestration, support diagnostics, and future Portal enrollment/reporting. It does not replace Docker. Runtime services are expected to run through Docker Compose on the customer host.

## Current status

This repository currently contains the initial Go scaffold:

- `cmd/bare-systems` binary entrypoint
- minimal command routing
- `help` and `version` output
- internal packages for CLI, output, config paths, runtime constants, errors, and version metadata

## Quick start

```sh
go test ./...
go run ./cmd/bare-systems --help
go run ./cmd/bare-systems version
go build -o bare-systems ./cmd/bare-systems
```

More development details are in [docs/development.md](docs/development.md).
