# Bare Systems Installer

Bare Systems Installer is the customer-facing CLI for Bare Systems edge deployments. The public command is `bare-systems`.

The CLI is responsible for installation workflows, local configuration, lifecycle orchestration, support diagnostics, and future Portal enrollment/reporting. It does not replace Docker. Runtime services are expected to run through Docker Compose on the customer host.

## Current status

This repository currently contains the initial Go scaffold:

- `cmd/bare-systems` binary entrypoint
- top-level command routing for the planned CLI surface
- `help` and `version` output
- JSON envelope output for supported commands
- documented exit-code constants
- internal packages for CLI, output, config paths, runtime constants, errors, and version metadata
- Portal enrollment, local device identity storage, heartbeat reporting, and offline report spooling

## Quick start

```sh
go test ./...
go run ./cmd/bare-systems --help
go run ./cmd/bare-systems version
go build -o bare-systems ./cmd/bare-systems
```

## Command surface

The initial command router recognizes the planned customer-facing command surface:

```sh
bare-systems version
bare-systems help
bare-systems init
bare-systems validate
bare-systems install
bare-systems start
bare-systems stop
bare-systems restart
bare-systems status
bare-systems ps
bare-systems logs
bare-systems update
bare-systems rollback
bare-systems doctor
bare-systems bundle
bare-systems enroll
bare-systems report
bare-systems config render
bare-systems config diff
bare-systems module list
bare-systems module enable <name>
bare-systems module disable <name>
bare-systems service install
bare-systems service uninstall
```

Implemented commands currently include `help`, `version`, `validate`, `config render`, core Docker Compose lifecycle commands, runtime status/ps/logs, systemd service install/uninstall helpers, Portal enrollment, and Portal reporting. Other recognized commands return a clear not-implemented message until their dedicated tickets add real behavior.

`validate` and `config render` implement the first deployment-model pass. They validate the built-in or configured `edge.yml` schema, built-in module manifests, non-secret `.env` values, and generated Compose YAML.

See [docs/config/edge-yml.md](docs/config/edge-yml.md) and [docs/modules.md](docs/modules.md) for the deployment contract.
See [docs/runtime.md](docs/runtime.md) for Docker Compose and systemd behavior.
See [docs/portal-contract.md](docs/portal-contract.md) for enrollment, identity, heartbeat, and spool behavior.
See [docs/support-runbook.md](docs/support-runbook.md) for doctor and diagnostics bundle usage.

## JSON output

Commands that support `--json` emit a stable envelope:

```json
{
  "ok": true,
  "command": "version",
  "code": "OK",
  "message": "Version information",
  "data": {},
  "warnings": [],
  "errors": [],
  "timestamp": "2026-06-13T12:34:56Z"
}
```

## Exit codes

| Code | Symbol | Meaning |
| ---: | --- | --- |
| 0 | `OK` | Success |
| 1 | `ERR_GENERIC` | Unclassified failure |
| 2 | `ERR_USAGE` | Invalid CLI arguments or unsupported command usage |
| 3 | `ERR_CONFIG` | Invalid config, schema, or value |
| 4 | `ERR_PREREQ` | Missing Docker or system prerequisite |
| 5 | `ERR_AUTH` | Enrollment or authentication failure |
| 6 | `ERR_NETWORK` | Network or Portal connectivity failure |
| 7 | `ERR_RUNTIME` | Compose or runtime failure |
| 8 | `ERR_HEALTH` | Services started but unhealthy |
| 9 | `ERR_UPDATE` | Update failed |
| 10 | `ERR_ROLLBACK` | Rollback failed |
| 11 | `ERR_DIAGNOSTICS` | Diagnostics failed |
| 12 | `ERR_PERMISSIONS` | Filesystem or privilege failure |

More development details are in [docs/development.md](docs/development.md).
