# Portal Contract

Bare Systems edge nodes enroll with the Bare Systems Portal by exchanging a short-lived bootstrap token for a long-lived device identity and device credential. The CLI owns the local enrollment files and the report payload shape; the Portal server implementation is out of scope for this repository.

## Commands

```sh
bare-systems enroll --portal https://portal.baresystems.com --token-file /path/to/token
bare-systems report
```

`enroll` also accepts `--portal` and `--token-file` as global flags before the command. `report` uses the persisted Portal URL and device credential from enrollment.

## Enrollment

Endpoint:

```text
POST /api/v1/devices/enroll
```

Request:

```json
{
  "enrollment_token": "one-time-bootstrap-token",
  "hostname": "edge-01",
  "platform": "linux",
  "arch": "amd64",
  "bare_systems_version": "0.1.0"
}
```

Response:

```json
{
  "device_id": "dev_abc123",
  "device_token": "portal-issued-device-token",
  "portal_url": "https://portal.baresystems.com"
}
```

Security behavior:

- the bootstrap enrollment token is read from the token file and sent only to the enrollment endpoint
- the bootstrap token is never persisted after a successful exchange
- enrollment requests are not retried automatically because bootstrap tokens can be one-time use
- the Portal response must include a non-empty `device_id` and `device_token`
- the CLI never prints the bootstrap token or device token

## Local Identity

Enrollment persists files under the deployment state directory:

```text
<project-dir>/state/device-identity.json
<project-dir>/state/device-token
```

For the default install root this resolves to:

```text
/opt/bare-systems/state/device-identity.json
/opt/bare-systems/state/device-token
```

`device-identity.json` stores the non-secret device identity, Portal URL, credential path, and enrollment timestamp. `device-token` stores the Portal credential with `0600` permissions. The state directory is created with restrictive permissions when needed.

## Report

Endpoint:

```text
POST /api/v1/devices/{device_id}/heartbeat
Authorization: Bearer <device-token>
```

Payload:

```json
{
  "deviceId": "dev_abc123",
  "timestamp": "2026-06-13T12:34:56Z",
  "cliVersion": {
    "version": "0.1.0",
    "commit": "abc123",
    "date": "2026-06-13T12:00:00Z"
  },
  "configRevision": "sha256:...",
  "deployment": {
    "name": "local-edge",
    "customer": "",
    "channel": "stable"
  },
  "enabledModules": ["core"],
  "profiles": ["core"],
  "serviceStatus": {
    "summary": {
      "total": 3,
      "byState": {"running": 3},
      "byHealth": {"healthy": 3}
    },
    "containers": []
  },
  "healthSummary": {
    "status": "pass",
    "total": 10,
    "counts": {"pass": 10},
    "message": "All required doctor checks passed"
  }
}
```

The report payload intentionally sends summarized runtime and health state, not high-volume metrics or logs. Credentials are never included.

## Offline Spool

If a report fails because the Portal is unreachable or returns a server error, the CLI writes the attempted payload and redacted failure metadata under:

```text
<project-dir>/state/reports/spool/report-YYYYMMDD-HHMMSS.json
```

Spool files are written with `0600` permissions. Explicit `bare-systems report` exits with `ERR_NETWORK` when a send fails, but local runtime commands do not depend on Portal reporting and continue to use only local Docker/Compose state.

Authentication failures are reported as `ERR_AUTH` because retrying the same credential is not expected to recover until the device is re-enrolled or the Portal credential is fixed.

## Optional Policy Fetch

The future policy/config fetch contract is reserved for:

```text
GET /api/v1/devices/{device_id}/policy
Authorization: Bearer <device-token>
```

The first implementation does not apply remote policy or execute remote commands. Any future policy response must be validated locally before it can affect runtime configuration.
