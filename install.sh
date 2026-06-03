#!/bin/sh
set -eu

REPO="shinagawa-web/pgincident"
BINARY="pgincident"

# Allow overrides via environment
VERSION="${VERSION:-}"
INSTALL_DIR="${INSTALL_DIR:-}"

# ── helpers ──────────────────────────────────────────────────────────────────

info()  { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
ok()    { printf '\033[1;32m  ✓\033[0m %s\n' "$*"; }
die()   { printf '\033[1;31merror:\033[0m %s\n' "$*" >&2; exit 1; }

need() {
    command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

# ── detect OS / arch ─────────────────────────────────────────────────────────

detect_os() {
    case "$(uname -s)" in
        Linux)  echo "linux"  ;;
        Darwin) echo "darwin" ;;
        *)      die "unsupported OS: $(uname -s)" ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64)          echo "amd64" ;;
        amd64)           echo "amd64" ;;
        aarch64|arm64)   echo "arm64" ;;
        *)               die "unsupported architecture: $(uname -m)" ;;
    esac
}

# ── resolve version ───────────────────────────────────────────────────────────

resolve_version() {
    if [ -n "$VERSION" ]; then
        echo "$VERSION"
        return
    fi
    need curl
    version=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
        | grep '"tag_name"' \
        | sed 's/.*"tag_name": *"\(.*\)".*/\1/')
    [ -n "$version" ] || die "could not determine latest version"
    echo "$version"
}

# ── resolve install dir ───────────────────────────────────────────────────────

resolve_install_dir() {
    if [ -n "$INSTALL_DIR" ]; then
        echo "$INSTALL_DIR"
        return
    fi
    if [ "$(id -u)" = "0" ]; then
        echo "/usr/local/bin"
    else
        echo "${HOME}/.local/bin"
    fi
}

# ── verify checksum ───────────────────────────────────────────────────────────

verify_checksum() {
    local archive="$1"
    local checksums_file="$2"
    local filename expected actual

    filename=$(basename "$archive")
    expected=$(awk -v f="$filename" '$2 == f {print $1}' "$checksums_file")
    [ -n "$expected" ] || die "checksum not found for $filename"

    if command -v sha256sum >/dev/null 2>&1; then
        actual=$(sha256sum "$archive" | awk '{print $1}')
    elif command -v shasum >/dev/null 2>&1; then
        actual=$(shasum -a 256 "$archive" | awk '{print $1}')
    else
        die "no sha256 tool found (sha256sum or shasum)"
    fi

    if [ "$actual" != "$expected" ]; then
        printf '\033[1;31merror:\033[0m checksum mismatch for %s\n  expected: %s\n  actual:   %s\n' \
            "$filename" "$expected" "$actual" >&2
        exit 1
    fi
}

# ── main ──────────────────────────────────────────────────────────────────────

main() {
    need curl
    need tar
    need install

    OS=$(detect_os)
    ARCH=$(detect_arch)
    VERSION=$(resolve_version)
    INSTALL_DIR=$(resolve_install_dir)

    # Strip leading 'v' for filename
    ver="${VERSION#v}"
    archive="${BINARY}_${ver}_${OS}_${ARCH}.tar.gz"
    base_url="https://github.com/${REPO}/releases/download/${VERSION}"

    tmpdir=$(mktemp -d "${TMPDIR:-/tmp}/pgincident-XXXXXX")
    trap 'rm -rf "$tmpdir"' EXIT

    info "Installing ${BINARY} ${VERSION} (${OS}/${ARCH})"

    info "Downloading archive..."
    curl -fsSL "${base_url}/${archive}" -o "${tmpdir}/${archive}"

    info "Downloading checksums..."
    curl -fsSL "${base_url}/checksums.txt" -o "${tmpdir}/checksums.txt"

    info "Verifying checksum..."
    verify_checksum "${tmpdir}/${archive}" "${tmpdir}/checksums.txt"
    ok "checksum verified"

    info "Extracting..."
    tar -xzf "${tmpdir}/${archive}" -C "$tmpdir"

    info "Installing to ${INSTALL_DIR}..."
    mkdir -p "$INSTALL_DIR"
    install -m 755 "${tmpdir}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
    ok "installed ${INSTALL_DIR}/${BINARY}"

    # PATH hint
    # shellcheck disable=SC2016  # $PATH literal is intentional: printed for the user to eval in their own shell
    case ":${PATH}:" in
        *":${INSTALL_DIR}:"*) ;;
        *) printf '\n\033[1;33mnote:\033[0m add %s to your PATH:\n  export PATH="%s:$PATH"\n' "$INSTALL_DIR" "$INSTALL_DIR" ;;
    esac

    printf '\n'
    "${INSTALL_DIR}/${BINARY}" --version
}

main
