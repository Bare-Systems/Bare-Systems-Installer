# Modules

Bare Systems modules are declared through module manifests. The CLI uses manifests to map operator intent in `edge.yml` to Compose profiles, services, images, required config, volumes, secrets, and health checks.

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
    koala-agent:
      image: ${KOALA_IMAGE:-registry.example.com/bare/koala-agent:unspecified}
  services:
    - name: koala-agent
      composeService: koala-agent
      image: ${KOALA_IMAGE:-registry.example.com/bare/koala-agent:unspecified}
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
| `core` | yes | `core` | Tardigrade reverse proxy, Bear Claw web UI, and Bear Claw agent integration |
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
- Compose profiles
- ports
- volumes
- Compose secrets by file path
- health checks
- non-secret environment values derived from `edge.yml`

The rendered Compose file is a generated artifact. Operators should edit `edge.yml`, `.env`, and secret files rather than editing generated Compose YAML directly.
