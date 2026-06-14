# Support Runbook

This runbook describes the first-line support workflow for a Bare Systems edge node.

## First Commands

Ask the operator to run:

```sh
bare-systems doctor
bare-systems doctor --json
bare-systems bundle --project-dir /opt/bare-systems
```

Use `--project-dir` for local development or nonstandard installs.

## Doctor

`doctor` prints pass/fail/unknown checks with remediation hints.

Health levels:

| Level | Meaning |
| --- | --- |
| `host` | Docker CLI, Docker daemon, and modern `docker compose` plugin |
| `config` | `edge.yml` and `.env` parse/validation |
| `compose` | generated Compose YAML is valid |
| `runtime` | Compose can report container state |
| `product-health` | module manifest health checks mapped to container health |
| `portal` | Portal reporting status, currently marked unknown until Portal integration lands |

Exit behavior:

- `0`: no failing checks
- `8 / ERR_HEALTH`: at least one required check failed
- unknown Portal/reporting checks do not fail MVP doctor output

## Diagnostics Bundle

`bundle` creates a gzip-compressed tar archive under:

```text
<project-dir>/bundles/diagnostics-YYYYMMDD-HHMMSS.tar.gz
```

Bundle sections:

```text
manifest.json
version.json
system/
config/
runtime/
health/
portal/
logs/
```

Included evidence:

- rendered Compose YAML
- redacted `edge.yml` model
- redacted non-secret env view
- built-in module manifests
- Docker prerequisite report
- `docker compose ps --format json` output when Docker is available
- selected service logs from enabled module manifests
- doctor report and checks
- Portal placeholder state

## Redaction

The bundle redacts secret-like assignments before writing entries. Keys containing these terms are treated as sensitive:

- `token`
- `secret`
- `password`
- `private_key` / `private-key`
- `api_key` / `api-key`
- `tls_key` / `tls-key`

Secret file contents under `/etc/bare-systems/secrets` are not read by the bundle workflow.

## Size Limits

Service logs are truncated to 64 KiB per service by default. Truncated logs include a `[TRUNCATED]` marker. This keeps support artifacts bounded while preserving the most recent evidence captured by `docker compose logs --tail 200`.

## Escalation Checklist

Collect these before escalating:

- `doctor --json` output
- diagnostics bundle path and checksum if transferred
- exact CLI version from `bare-systems version`
- deployment channel and enabled modules from the bundle manifest
- recent operator action: install, start, update, rollback, or manual Docker changes
