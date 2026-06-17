# Runtime

Bare Systems runtime behavior uses Docker Compose for containers and Tardigrade as a host binary for the public reverse proxy. The CLI does not replace Docker or supervise individual containers itself.

Tardigrade is outside the Compose graph. It runs on the edge node so it can bind public ports and proxy traffic into Docker-managed services without being pulled as a deployment-time image.

## Prerequisites

Runtime commands require:

- Docker CLI on `PATH`
- reachable Docker daemon
- modern Docker Compose plugin through `docker compose`
- `tardigrade` on `PATH` for `install`, `start`, `stop`, `restart`, and `update`

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

## Tardigrade Proxy

Runtime artifact writes also generate:

```text
<project-dir>/tardigrade/tardigrade.conf
<project-dir>/tardigrade/public/
<project-dir>/state/tardigrade.pid
```

The generated Tardigrade config listens on `PUBLIC_HTTP_PORT` and proxies to Bear Claw Web through the host loopback port rendered into Compose. By default Bear Claw Web is published as `127.0.0.1:8080:80`, and Tardigrade proxies to `http://127.0.0.1:8080`.

The install script installs the Tardigrade CLI before runtime commands are used. Runtime commands do not download Tardigrade.

## Commands

Implemented runtime commands:

```sh
bare-systems --project-dir ./tmp-edge init
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

| CLI | Compose behavior | Tardigrade behavior |
| --- | --- | --- |
| `install` | render Compose, validate with `docker compose config -q`, then `docker compose pull` | render and validate `tardigrade.conf` |
| `start` | render Compose, then `docker compose up -d` | start Tardigrade with `--daemon`, or reload if already running |
| `stop` | `docker compose stop` | stop Tardigrade when running |
| `restart` | `docker compose restart` | reload Tardigrade after Compose restarts |
| `update` | render Compose, pull images, then `up -d` | reload Tardigrade after Compose updates |
| `status` | `docker compose ps --format json` summarized for humans or JSON | not queried yet |
| `ps` | `docker compose ps`, or structured state with `--json` | not queried yet |
| `logs [service]` | `docker compose logs --tail 200 [service]` | not queried yet |

`doctor` and `bundle` use the same runtime boundaries but are support-oriented. `doctor` summarizes host, config, Compose, runtime, product health, and Portal reporting status. `bundle` writes a redacted tar.gz support artifact with rendered config, runtime state, logs, and doctor output.

`enroll` and `report` are Portal-oriented. Enrollment stores the device identity under the deployment state directory and stores the device credential with `0600` permissions. Reporting sends a heartbeat/status payload to the enrolled Portal URL and spools failed network/server sends under `<project-dir>/state/reports/spool`.

## systemd

`bare-systems service install` writes `bare-systems-edge.service` on Linux/systemd hosts. The unit is oneshot with `RemainAfterExit=yes` and starts the Compose deployment at boot:

```ini
ExecStart=/usr/bin/bare-systems start
ExecStop=/usr/bin/bare-systems stop
ExecReload=/usr/bin/bare-systems restart
WantedBy=multi-user.target
```

Install and uninstall are idempotent. The default unit relies on `/etc/bare-systems/edge.yml`, `/etc/bare-systems/.env`, and `/opt/bare-systems`. Running service management on non-Linux hosts returns a prerequisite error.
