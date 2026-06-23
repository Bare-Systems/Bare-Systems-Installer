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
- each locally deployed enabled module's required config keys are present through derived config or `.env`
- optional modules using `deployment.mode: external` include a `deployment.url`
- `core` is always locally deployed
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

Enabled optional modules can be rendered locally or integrated as external services:

```yaml
modules:
  koala:
    enabled: true
    deployment:
      mode: external
      url: http://jetson:6705
```

External modules are reported as enabled but are excluded from the local Compose file. Their URL is passed to Bear Claw Web as the matching non-secret environment value, such as `KOALA_URL`, `POLAR_URL`, `KODIAK_URL`, or `URSA_URL`.

The rendered Compose file is a generated artifact. Operators should edit `edge.yml`, `.env`, and secret files rather than editing generated Compose YAML directly.

For the required `core` module, Bear Claw Web is exposed to the host at `BEARCLAW_WEB_BIND_ADDRESS:BEARCLAW_WEB_PORT`, defaulting to `127.0.0.1:8080`. The generated Tardigrade config proxies public HTTP traffic to that loopback endpoint.

The Open Claw/BearClaw agent is a host-local daemon, not a Compose service. Tardigrade proxies it under `/bearclaw/*` using `BEARCLAW_AGENT_URL`, defaulting to `http://127.0.0.1:8080`. The web container receives `BEARCLAW_URL=http://host.docker.internal/bearclaw` and an `extra_hosts` host-gateway mapping so Rails can reach the authenticated agent route without exposing the daemon directly on the LAN.

LLM selection is represented by non-secret environment keys:

- `BEARCLAW_LLM_PROVIDER`, for example `ollama`, `openai`, `anthropic`, `openrouter`, or `openai-compatible`
- `BEARCLAW_LLM_MODEL`, for example `qwen2.5:1.5b`
- `BEARCLAW_LLM_BASE_URL`, for local or OpenAI-compatible endpoints

Provider API keys are intentionally not stored in `.env`; they belong in the secrets directory or the host daemon's protected environment.
