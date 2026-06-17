# Modules

Bare Systems modules are declared through module manifests. The CLI uses manifests to map operator intent in `edge.yml` to Compose profiles, services, images, required config, volumes, secrets, and health checks.

Tardigrade is not rendered as a Compose service. It runs as a host binary so it can bind public ports and reverse proxy into the Docker network. The core Bear Claw Web service is published on a loopback host port for Tardigrade to reach.

The built-in manifest schema is versioned:

```yaml
apiVersion: bare.systems/v1alpha1
kind: ModuleManifest
metadata:
  name: koala
  version: 1
module:
  id: koala
  required: false
  defaultEnabled: false
  profiles:
    - koala
  images:
    koala-orchestrator:
      image: ${KOALA_ORCHESTRATOR_IMAGE:-ghcr.io/bare-systems/koala-orchestrator:latest}
    koala-worker:
      image: ${KOALA_WORKER_IMAGE:-ghcr.io/bare-systems/koala-worker:latest}
  services:
    - name: koala-orchestrator
      composeService: koala-orchestrator
      image: ${KOALA_ORCHESTRATOR_IMAGE:-ghcr.io/bare-systems/koala-orchestrator:latest}
      imageRepository: koala-orchestrator
      profiles:
        - koala
      volumes:
        - koala-data:/var/lib/bare-systems/koala
      health:
        type: exec
        command: ["CMD", "/app/healthcheck"]
    - name: koala-worker
      composeService: koala-worker
      image: ${KOALA_WORKER_IMAGE:-ghcr.io/bare-systems/koala-worker:latest}
      imageRepository: koala-worker
      profiles:
        - koala
      volumes:
        - koala-data:/var/lib/bare-systems/koala
      health:
        type: exec
        command: ["CMD", "/app/healthcheck"]
  config:
    required:
      - KOALA_SITE_ID
    optional:
      - KOALA_RETENTION_DAYS
  volumes:
    - koala-data
  secrets: []
```

## Built-In Modules

The initial registry contains:

| Module | Required | Profile | Purpose |
| --- | --- | --- | --- |
| `core` | yes | `core` | Bear Claw web UI; Tardigrade is managed as a host binary outside Compose |
| `koala` | no | `koala` | Camera and home security services |
| `polar` | no | `polar` | Operational monitoring services |
| `kodiak` | no | `kodiak` | Local orchestration services |
| `ursa` | no | `ursa` | Security scanning services |

## Validation

Validation enforces:

- every enabled module exists in the registry
- `core` remains enabled
- every configured profile is known
- every configured profile belongs to an active module
- each enabled module's required config keys are present through derived config or `.env`
- `.env` does not contain secret-looking keys

## Compose Rendering

`bare-systems config render` emits a Compose YAML model from enabled modules. The renderer includes:

- service images
- image repository names for registry/tag overrides
- Compose profiles
- ports
- volumes
- Compose secrets by file path
- health checks
- non-secret environment values derived from `edge.yml`

The rendered Compose file is a generated artifact. Operators should edit `edge.yml`, `.env`, and secret files rather than editing generated Compose YAML directly.

For the required `core` module, Bear Claw Web is exposed to the host at `BEARCLAW_WEB_BIND_ADDRESS:BEARCLAW_WEB_PORT`, defaulting to `127.0.0.1:8080`. The generated Tardigrade config proxies public HTTP traffic to that loopback endpoint.
