---
title: Installation
weight: 10
---

# Installation

## Homebrew (macOS)

```sh
brew tap shinagawa-web/tap
brew install pgincident
```

## Linux / macOS (one-line installer)

```sh
curl -fsSL https://raw.githubusercontent.com/shinagawa-web/pgincident/main/install.sh | sh
```

Installs to `~/.local/bin` (or `/usr/local/bin` when run as root). Override with `INSTALL_DIR`:

```sh
# Install to a custom directory
curl -fsSL https://raw.githubusercontent.com/shinagawa-web/pgincident/main/install.sh | INSTALL_DIR=~/bin sh

# Install system-wide (requires root)
curl -fsSL https://raw.githubusercontent.com/shinagawa-web/pgincident/main/install.sh | sudo INSTALL_DIR=/usr/local/bin sh
```

Pin a specific version with `VERSION`:

```sh
curl -fsSL https://raw.githubusercontent.com/shinagawa-web/pgincident/main/install.sh | VERSION=v0.5.1 sh
```

## Download binary

Download the latest release from [GitHub Releases](https://github.com/shinagawa-web/pgincident/releases), extract, and place the binary in your `$PATH`.

## Build from source

```sh
go install github.com/shinagawa-web/pgincident/cmd/pgincident@latest
```

Requires Go 1.25 or later.
