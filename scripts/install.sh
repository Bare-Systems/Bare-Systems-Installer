#!/usr/bin/env sh
set -eu

REPO="${BARE_SYSTEMS_REPO:-Bare-Systems/Bare-Systems-Installer}"
VERSION="${BARE_SYSTEMS_VERSION:-latest}"
INSTALL_DIR="${BARE_SYSTEMS_INSTALL_DIR:-/usr/local/bin}"
BINARY_NAME="bare-systems"
TARDIGRADE_REPO="${BARE_SYSTEMS_TARDIGRADE_REPO:-Bare-Systems/Tardigrade}"
TARDIGRADE_VERSION="${BARE_SYSTEMS_TARDIGRADE_VERSION:-latest}"
TARDIGRADE_RELEASE_BASE_URL="${BARE_SYSTEMS_TARDIGRADE_RELEASE_BASE_URL:-https://github.com/$TARDIGRADE_REPO/releases}"
TARDIGRADE_ASSET="${BARE_SYSTEMS_TARDIGRADE_ASSET:-tardigrade-linux-x86_64.tar.gz}"
SKIP_TARDIGRADE="${BARE_SYSTEMS_SKIP_TARDIGRADE:-0}"

usage() {
  cat <<'USAGE'
Install the Bare Systems CLI from GitHub Releases.

Environment:
  BARE_SYSTEMS_VERSION      Release tag to install, for example v0.1.0. Defaults to latest.
  BARE_SYSTEMS_REPO         GitHub repo owner/name. Defaults to Bare-Systems/Bare-Systems-Installer.
  BARE_SYSTEMS_INSTALL_DIR  Install directory. Defaults to /usr/local/bin.
  BARE_SYSTEMS_SKIP_TARDIGRADE  Set to 1 to install only bare-systems.
  BARE_SYSTEMS_TARDIGRADE_VERSION  Tardigrade release tag. Defaults to latest.
  BARE_SYSTEMS_TARDIGRADE_ASSET    Tardigrade archive asset. Defaults to tardigrade-linux-x86_64.tar.gz.
USAGE
}

case "${1:-}" in
  -h|--help)
    usage
    exit 0
    ;;
esac

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "bare-systems install: missing required command: $1" >&2
    exit 1
  fi
}

download() {
  url="$1"
  out="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$out"
    return
  fi
  if command -v wget >/dev/null 2>&1; then
    wget -q "$url" -O "$out"
    return
  fi
  echo "bare-systems install: curl or wget is required" >&2
  exit 1
}

detect_os() {
  os="$(uname -s)"
  case "$os" in
    Linux) echo "linux" ;;
    Darwin) echo "darwin" ;;
    *)
      echo "bare-systems install: unsupported OS: $os" >&2
      exit 1
      ;;
  esac
}

detect_arch() {
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64) echo "amd64" ;;
    arm64|aarch64) echo "arm64" ;;
    *)
      echo "bare-systems install: unsupported architecture: $arch" >&2
      exit 1
      ;;
  esac
}

resolve_latest_version() {
  tmp="$1"
  api_url="https://api.github.com/repos/${REPO}/releases/latest"
  download "$api_url" "$tmp/latest.json"
  tag="$(sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$tmp/latest.json" | head -n 1)"
  if [ -z "$tag" ]; then
    echo "bare-systems install: could not resolve latest release tag" >&2
    exit 1
  fi
  echo "$tag"
}

verify_checksum() {
  archive="$1"
  checksums="$2"
  grep " ${archive}\$" "$checksums" > "${checksums}.one" || {
    echo "bare-systems install: checksum entry missing for $archive" >&2
    exit 1
  }
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum -c "${checksums}.one"
    return
  fi
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 -c "${checksums}.one"
    return
  fi
  echo "bare-systems install: sha256sum or shasum is required for checksum verification" >&2
  exit 1
}

verify_checksum_by_basename() {
  archive="$1"
  checksums="$2"
  asset="$3"
  expected="$(awk -v target="$asset" '
    {
      n = split($2, parts, "/")
      if (parts[n] == target) {
        print $1
        exit
      }
    }
  ' "$checksums")"
  if [ -z "$expected" ]; then
    echo "bare-systems install: checksum entry missing for $asset" >&2
    exit 1
  fi
  if command -v sha256sum >/dev/null 2>&1; then
    actual="$(sha256sum "$archive" | awk '{print $1}')"
  elif command -v shasum >/dev/null 2>&1; then
    actual="$(shasum -a 256 "$archive" | awk '{print $1}')"
  else
    echo "bare-systems install: sha256sum or shasum is required for checksum verification" >&2
    exit 1
  fi
  if [ "$actual" != "$expected" ]; then
    printf 'bare-systems install: checksum mismatch for %s\n  expected: %s\n  got:      %s\n' "$asset" "$expected" "$actual" >&2
    exit 1
  fi
}

ensure_install_dir() {
  if [ ! -d "$INSTALL_DIR" ]; then
    if [ -w "$(dirname "$INSTALL_DIR")" ]; then
      mkdir -p "$INSTALL_DIR"
    elif command -v sudo >/dev/null 2>&1; then
      sudo mkdir -p "$INSTALL_DIR"
    else
      echo "bare-systems install: cannot create $INSTALL_DIR; rerun with permissions or set BARE_SYSTEMS_INSTALL_DIR" >&2
      exit 1
    fi
  fi
}

install_file() {
  src="$1"
  dst="$2"
  ensure_install_dir
  if [ -w "$INSTALL_DIR" ]; then
    install -m 0755 "$src" "$dst"
  elif command -v sudo >/dev/null 2>&1; then
    sudo install -m 0755 "$src" "$dst"
  else
    echo "bare-systems install: cannot write to $INSTALL_DIR; rerun with permissions or set BARE_SYSTEMS_INSTALL_DIR" >&2
    exit 1
  fi
}

install_binary() {
  src="$1"
  dst="${INSTALL_DIR}/${BINARY_NAME}"
  install_file "$src" "$dst"
  echo "bare-systems installed to $dst"
}

install_tardigrade() {
  if [ "$SKIP_TARDIGRADE" = "1" ] || [ "$SKIP_TARDIGRADE" = "true" ]; then
    echo "tardigrade install skipped"
    return
  fi

  if [ "$TARDIGRADE_VERSION" = "latest" ]; then
    tardigrade_base_url="${TARDIGRADE_RELEASE_BASE_URL}/latest/download"
  else
    tardigrade_base_url="${TARDIGRADE_RELEASE_BASE_URL}/download/${TARDIGRADE_VERSION}"
  fi

  tardigrade_archive="$tmp_dir/$TARDIGRADE_ASSET"
  tardigrade_checksums="$tmp_dir/tardigrade-checksums.txt"
  download "${tardigrade_base_url}/${TARDIGRADE_ASSET}" "$tardigrade_archive"
  download "${tardigrade_base_url}/tardigrade-checksums.txt" "$tardigrade_checksums"
  verify_checksum_by_basename "$tardigrade_archive" "$tardigrade_checksums" "$TARDIGRADE_ASSET"

  tardigrade_dir="$tmp_dir/tardigrade"
  mkdir -p "$tardigrade_dir"
  tar -xzf "$tardigrade_archive" -C "$tardigrade_dir"
  if [ ! -f "$tardigrade_dir/tardigrade" ]; then
    echo "bare-systems install: Tardigrade archive did not contain tardigrade" >&2
    exit 1
  fi
  install_file "$tardigrade_dir/tardigrade" "${INSTALL_DIR}/tardigrade"
  install_file "$tardigrade_dir/tardigrade" "${INSTALL_DIR}/tardi"
  echo "tardigrade installed to ${INSTALL_DIR}/tardigrade"
}

need_cmd uname
need_cmd tar
need_cmd grep
need_cmd sed
need_cmd awk

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT INT TERM

if [ "$VERSION" = "latest" ]; then
  VERSION="$(resolve_latest_version "$tmp_dir")"
fi

os="$(detect_os)"
arch="$(detect_arch)"
archive="bare-systems_${os}_${arch}.tar.gz"
base_url="https://github.com/${REPO}/releases/download/${VERSION}"

cd "$tmp_dir"
download "${base_url}/${archive}" "$archive"
download "${base_url}/checksums.txt" checksums.txt
verify_checksum "$archive" checksums.txt
tar -xzf "$archive"
if [ ! -f "$BINARY_NAME" ]; then
  echo "bare-systems install: archive did not contain $BINARY_NAME" >&2
  exit 1
fi
install_binary "$tmp_dir/$BINARY_NAME"
install_tardigrade
