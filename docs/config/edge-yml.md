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
      deployment:
        mode: local
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
- Optional modules default to `deployment.mode: local`, which renders their Compose services.
- Optional modules can use `deployment.mode: external` with `deployment.url` when that service is managed outside this installer. External modules remain enabled for BearClaw Web integration, but their local Compose profiles, services, volumes, and required local config are not rendered.
- `core` cannot use `deployment.mode: external`.

Example split deployment:

```yaml
spec:
  runtime:
    profiles:
      - core
  modules:
    core:
      enabled: true
    koala:
      enabled: true
      deployment:
        mode: external
        url: http://jetson:6705
```

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
BEARCLAW_AGENT_URL
BEARCLAW_LLM_BASE_URL
BEARCLAW_LLM_MODEL
BEARCLAW_LLM_PROVIDER
BEARCLAW_URL
BEARCLAW_WEB_BIND_ADDRESS
BEARCLAW_WEB_PORT
KOALA_URL
KODIAK_URL
POLAR_URL
URSA_URL
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

`BEARCLAW_AGENT_URL` controls the host-local Open Claw/BearClaw daemon upstream that Tardigrade proxies under `/bearclaw/*`. It defaults to the loopback daemon:

```text
BEARCLAW_AGENT_URL=http://127.0.0.1:8080
```

`BEARCLAW_URL` is the URL Bear Claw Web uses from inside its Docker container to reach the authenticated Tardigrade BearClaw route. The default uses Docker's host gateway alias:

```text
BEARCLAW_URL=http://host.docker.internal/bearclaw
```

The non-secret LLM selection knobs are:

```text
BEARCLAW_LLM_PROVIDER=ollama
BEARCLAW_LLM_MODEL=qwen2.5:1.5b
BEARCLAW_LLM_BASE_URL=http://127.0.0.1:11434/api/chat
```

External-provider API keys are secrets and should be stored under `/etc/bare-systems/secrets`, not in `.env`.

For modules using `deployment.mode: external`, the CLI derives the matching upstream URL for BearClaw Web:

```text
KOALA_URL
KODIAK_URL
POLAR_URL
URSA_URL
```

Module tokens are still secrets and should be supplied from files or protected service environment, not from `.env`.

`TARDIGRADE_SERVER_NAMES` controls the server names accepted by the generated Tardigrade reverse proxy config. Include the LAN IP or hostname when health checks or users access the device through that address:

```text
TARDIGRADE_SERVER_NAMES='localhost 127.0.0.1 host.docker.internal 192.168.86.53'
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
