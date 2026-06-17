# Runtime

Bare Systems runtime behavior is Docker Compose based. The CLI does not replace Docker or supervise individual containers itself.

## Prerequisites

Runtime commands require:

- Docker CLI on `PATH`
- reachable Docker daemon
- modern Docker Compose plugin through `docker compose`

Legacy `docker-compose` is not supported.

If a prerequisite is missing, the CLI exits with `ERR_PREREQ` and prints remediation text.

## Compose Project

Runtime commands render the deployment model into:

```text
<project-dir>/compose/generated.compose.yml
```

Commands run from the resolved Compose project directory and pass deterministic module profiles:

```sh
docker compose \
  --project-directory <project-dir>/compose \
  --project-name bare-systems \
  -f <project-dir>/compose/generated.compose.yml \
  --profile core \
  up -d
```

## Commands

Implemented runtime commands:

```sh
bare-systems --project-dir ./tmp-edge init --image-registry localhost:5000/bare --image-tag homelab
bare-systems --project-dir ./tmp-edge validate
bare-systems --project-dir ./tmp-edge config render --write
bare-systems install
bare-systems start
bare-systems stop
bare-systems restart
bare-systems update
bare-systems status
bare-systems ps
bare-systems logs
bare-systems logs <service>
bare-systems doctor
bare-systems bundle
bare-systems enroll --portal https://portal.baresystems.com --token-file /path/to/token
bare-systems report
```

Mappings:

| CLI | Compose behavior |
| --- | --- |
| `install` | render Compose, validate with `docker compose config -q`, then `docker compose pull` |
| `start` | render Compose, then `docker compose up -d` |
| `stop` | `docker compose stop` |
| `restart` | `docker compose restart` |
| `update` | render Compose, pull images, then `up -d` |
| `status` | `docker compose ps --format json` summarized for humans or JSON |
| `ps` | `docker compose ps`, or structured state with `--json` |
| `logs [service]` | `docker compose logs --tail 200 [service]` |

`doctor` and `bundle` use the same runtime boundaries but are support-oriented. `doctor` summarizes host, config, Compose, runtime, product health, and Portal reporting status. `bundle` writes a redacted tar.gz support artifact with rendered config, runtime state, logs, and doctor output.

`enroll` and `report` are Portal-oriented. Enrollment stores the device identity under the deployment state directory and stores the device credential with `0600` permissions. Reporting sends a heartbeat/status payload to the enrolled Portal URL and spools failed network/server sends under `<project-dir>/state/reports/spool`.

## systemd

`bare-systems service install` writes `bare-systems-edge.service` on Linux/systemd hosts. The unit is oneshot with `RemainAfterExit=yes` and starts the Compose deployment at boot:

```ini
ExecStart=/usr/bin/bare-systems --project-dir /opt/bare-systems start
ExecStop=/usr/bin/bare-systems --project-dir /opt/bare-systems stop
ExecReload=/usr/bin/bare-systems --project-dir /opt/bare-systems restart
WantedBy=multi-user.target
```

Install and uninstall are idempotent. Running service management on non-Linux hosts returns a prerequisite error.
