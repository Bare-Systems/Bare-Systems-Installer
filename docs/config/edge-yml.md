# edge.yml

`edge.yml` is the operator-owned deployment intent file for a Bare Systems edge node. It is versioned so future schema changes can be migrated intentionally.

```yaml
apiVersion: bare.systems/v1alpha1
kind: EdgeDeployment
metadata:
  name: local-edge
spec:
  channel: stable
  projectName: bare-systems
  runtime:
    composeProjectDirectory: /opt/bare-systems/compose
    dockerContext: default
    profiles:
      - core
  modules:
    core:
      enabled: true
    koala:
      enabled: false
    polar:
      enabled: false
    kodiak:
      enabled: false
    ursa:
      enabled: false
  networking:
    publicHttpPort: 80
    publicHttpsPort: 443
    adminBindAddress: 0.0.0.0
  storage:
    root: /opt/bare-systems
    backups: /opt/bare-systems/backups
```

## Rules

- `apiVersion` must be `bare.systems/v1alpha1`.
- `kind` must be `EdgeDeployment`.
- `metadata.name` is required.
- `spec.channel` and `spec.projectName` are required.
- `core` is always required and cannot be disabled.
- Unknown module names fail validation.
- Unknown runtime profiles fail validation.
- A profile is valid only when an active module declares it.

## Paths

Production defaults:

```text
/etc/bare-systems/edge.yml
/etc/bare-systems/.env
/etc/bare-systems/secrets/
/opt/bare-systems/compose/
/opt/bare-systems/manifests/
/opt/bare-systems/state/
/opt/bare-systems/bundles/
/var/log/bare-systems/
```

## .env

`.env` is for non-secret Docker Compose interpolation values only. The CLI derives these values from `edge.yml` when possible:

```text
BARE_IMAGE_REGISTRY
BARE_IMAGE_TAG
BARE_CHANNEL
BARE_PROJECT_NAME
PUBLIC_HTTP_PORT
PUBLIC_HTTPS_PORT
ADMIN_BIND_ADDRESS
BEARCLAW_WEB_BIND_ADDRESS
BEARCLAW_WEB_PORT
BARE_COMPOSE_DIR
BARE_STORAGE_ROOT
```

By default the CLI renders images from GitHub Container Registry:

```text
BARE_IMAGE_REGISTRY=ghcr.io/bare-systems
BARE_IMAGE_TAG=latest
```

GHCR packages must be public for anonymous pulls. If packages remain private, log the edge node in with a token that has `read:packages` before running `install`, `start`, or `update`.

For a temporary local homelab registry, initialize or edit `.env` with:

```text
BARE_IMAGE_REGISTRY=localhost:5000/bare
BARE_IMAGE_TAG=homelab
```

Rendered Compose image names use `BARE_IMAGE_REGISTRY/<imageRepository>:BARE_IMAGE_TAG` unless a per-service override such as `BEARCLAW_WEB_IMAGE`, `KOALA_ORCHESTRATOR_IMAGE`, or `KOALA_WORKER_IMAGE` is set. Tardigrade is not rendered as a Compose image; it runs as a host binary.

`PUBLIC_HTTP_PORT` controls the generated Tardigrade listen port. `BEARCLAW_WEB_BIND_ADDRESS` and `BEARCLAW_WEB_PORT` control the loopback host port that Bear Claw Web publishes for Tardigrade to proxy to. The defaults are:

```text
BEARCLAW_WEB_BIND_ADDRESS=127.0.0.1
BEARCLAW_WEB_PORT=8080
```

Do not put tokens, passwords, API keys, private keys, or TLS keys in `.env`. Validation flags secret-looking keys such as `PORTAL_TOKEN`, `PASSWORD`, `SECRET`, `API_KEY`, and `PRIVATE_KEY`.

## Secrets

Secrets are modeled as files under `/etc/bare-systems/secrets`. Module manifests reference those files by name and path, and rendered Compose mounts them as Compose secrets. Secret contents are not read or printed by `validate` or `config render`.

## Commands

Initialize a local editable project:

```sh
bare-systems --project-dir ./tmp-edge init
```

Validate the deployment model:

```sh
bare-systems --project-dir ./tmp-edge validate
```

Render canonical runtime artifacts:

```sh
bare-systems --project-dir ./tmp-edge config render
bare-systems --project-dir ./tmp-edge config render --write
```
