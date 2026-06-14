# Development

## Requirements

- Go 1.24 or newer

The project keeps dependencies minimal; YAML parsing uses `gopkg.in/yaml.v3`.

## Test

```sh
go test ./...
```

## Run locally

```sh
go run ./cmd/bare-systems --help
go run ./cmd/bare-systems version
go run ./cmd/bare-systems --json version
go run ./cmd/bare-systems --project-dir ./tmp-edge init
go run ./cmd/bare-systems --project-dir ./tmp-edge validate
go run ./cmd/bare-systems --project-dir ./tmp-edge config render
go run ./cmd/bare-systems --project-dir ./tmp-edge config render --write
go run ./cmd/bare-systems --json status
go run ./cmd/bare-systems doctor
go run ./cmd/bare-systems --project-dir ./tmp-edge bundle
go run ./cmd/bare-systems --project-dir ./tmp-edge enroll --portal http://127.0.0.1:8080 --token-file ./token.txt
go run ./cmd/bare-systems --project-dir ./tmp-edge --json report
```

## Build

Build a local binary named `bare-systems`:

```sh
go build -o bare-systems ./cmd/bare-systems
```

Release builds can inject version metadata with linker flags:

```sh
go build \
  -trimpath \
  -ldflags "-X main.versionValue=0.1.0 -X main.commitValue=$(git rev-parse --short HEAD) -X main.dateValue=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  -o bare-systems \
  ./cmd/bare-systems
```

## Release Validation

Release automation is defined in `.goreleaser.yaml` and `.github/workflows/release.yml`. When GoReleaser is installed locally, run:

```sh
goreleaser check
goreleaser release --snapshot --clean
```

The install script can be syntax-checked without downloading artifacts:

```sh
bash -n scripts/install.sh
sh scripts/install.sh --help
```
