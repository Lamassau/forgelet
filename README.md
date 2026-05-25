# Forgelet

Forgelet is a Go CLI for bootstrapping and operating a local k0s-based Kubernetes development environment. It wraps machine setup, cluster install, image builds, app deploys, local DNS, and inner-loop tooling behind a single command surface.

## Highlights

- Full local bootstrap with `forgelet up`
- Native Linux or Podman VM workflow for macOS/WSL
- Local registry + image import flow for k0s/containerd
- CDK8s synth and deploy helpers
- Skaffold-based development loop
- Diagnostics, config inspection, shell completion, and self-update helpers

## How It Works

```text
                   +-----------------------+
                   |     forgelet CLI      |
                   +-----------+-----------+
                               |
           +-------------------+-------------------+
           |                                       |
           v                                       v
+----------------------+                 +----------------------+
| Host / Podman VM     |                 | Project Workspace    |
| - podman machine     |                 | - .devenv/           |
| - k0s controller     |                 | - .infra/ CDK8s      |
| - local registry     |                 | - skaffold.yaml      |
+----------+-----------+                 +----------+-----------+
           |                                         |
           v                                         v
+----------------------+                 +----------------------+
| Kubernetes Services  |<----------------| Build + Synth        |
| - local-path         |                 | - podman build       |
| - MetalLB            |                 | - pnpm / cdk8s synth |
| - Traefik            |                 | - skaffold dev       |
+----------+-----------+                 +----------------------+
           |
           v
+----------------------+
| Local Access         |
| - kubeconfig         |
| - /etc/hosts or      |
|   /etc/resolver      |
| - web/api domains    |
+----------------------+
```

## Requirements

- Go 1.22+
- Podman
- kubectl
- k0s or k0sctl depending on platform
- Node.js + pnpm for CDK8s projects
- Skaffold for the dev loop

Use `forgelet prerequisites` to install or verify most of these tools.

## Installation

### From source

```bash
git clone https://github.com/lnyousif/forgelet.git
cd forgelet
go build -o bin/forgelet ./cmd/forgelet
./bin/forgelet --help
```

### Go install

```bash
go install github.com/lnyousif/forgelet/cmd/forgelet@latest
forgelet version
```

## Quick Start

Example: bootstrap a project named `shop` with domain `shop.local`.

```bash
mkdir shop && cd shop
forgelet init --app-name shop --domain shop.local --skaffold --infra
forgelet up
forgelet check
forgelet dev
```

Typical flow:

1. Create or review `.devenv/forgelet.yaml`
2. Run `forgelet up`
3. Confirm health with `forgelet check` or `forgelet status`
4. Start the inner loop with `forgelet dev`
5. Tail workload logs with `forgelet logs api web`

## Configuration Discovery

Forgelet looks for configuration in this order:

1. `--config /path/to/forgelet.yaml`
2. `.devenv/forgelet.yaml` in the current directory or a parent directory
3. `.forgelet/forgelet.yaml` in the current directory or a parent directory
4. If only `go.mod` is found, Forgelet assumes the project root and a default `.devenv/forgelet.yaml` path

Inspect the resolved configuration with:

```bash
forgelet config show
```

## Environment Variables

Environment-variable overrides are reflected in `forgelet config show`.

| Variable | Purpose |
| --- | --- |
| `APP_ENV` | Selects build/deploy environment |
| `DEVENV_ENV` | Alternate environment selector |
| `FORGELET_ENV` | Alternate environment selector |
| `forgelet_ENV` | Legacy environment selector |
| `APP_NAME` | Overrides `app.name` |
| `DOMAIN` | Overrides `app.domain` |
| `CLUSTER_NAME` | Overrides `cluster.clusterName` |
| `KUBECONFIG_DIR` | Overrides `cluster.kubeConfigDir` |
| `DOCKER_COMPOSE_FILE` | Overrides compose discovery |
| `INFRA_DIR` | Overrides `.infra` directory |
| `DOCKER_REGISTRY` | Overrides `podman.registry` |
| `K0S_VERSION` | Pins k0s install version |
| `METALLB_POOL_RANGE` | Overrides MetalLB IP range |
| `VERSION` | Overrides image tag / build version |
| `CORE_DNS_UPSTREAMS` | Overrides CoreDNS forwarders on native Linux |
| `DOCKER_REGISTRY_USERNAME` | Registry login username |
| `DOCKER_REGISTRY_PASSWORD` | Registry login password |
| `PODMAN_REGISTRY_USERNAME` | Alternate registry login username |
| `PODMAN_REGISTRY_PASSWORD` | Alternate registry login password |
| `CODESPACES` | Enables Codespaces-specific behavior |
| `K0S_MODE` | Forces platform mode detection (for Linux) |
| `HOME` | Used when expanding `${HOME}` in config |

## Commands

### Core lifecycle

- `forgelet init` — scaffold `.devenv/forgelet.yaml` and optional skeleton files
- `forgelet up` — full bootstrap from prerequisites through DNS
- `forgelet prerequisites` — install/check required tools
- `forgelet machine-up` — start the Podman machine or prepare native runtime
- `forgelet k0s-install` — install and start k0s
- `forgelet kubeconfig` — write kubeconfig for the cluster
- `forgelet build [env]` — build, push, and import images
- `forgelet import-image <image>` — import an already-built image into k0s
- `forgelet synth` — synthesize CDK8s manifests
- `forgelet deploy` — deploy platform services and app manifests
- `forgelet dns` — configure local DNS/hosts entries
- `forgelet dev` — run the Skaffold dev loop
- `forgelet logs <service...>` — tail one or more deployment logs
- `forgelet dashboard` — install and open Kubernetes Dashboard
- `forgelet status` — print a quick cluster summary
- `forgelet check` — run health checks and diagnostics
- `forgelet reset` — wipe k0s state but keep runtime host
- `forgelet destroy` — tear down the local environment

### Configuration and tooling

- `forgelet config show` — print resolved configuration as YAML
- `forgelet completion [bash|zsh|fish|powershell]` — generate shell completion scripts
- `forgelet version` — print build metadata
- `forgelet update` — update the CLI with `go install`
- `forgelet init-ci` — generate a starter GitHub Actions workflow

## Global Flags

- `--config <path>` — explicit config file path
- `--dry-run` — print commands without executing them
- `--verbose`, `-V` — print commands before running them

## Example Config

```yaml
cluster:
  clusterName: k0s-dev
  kubeConfigDir: ~/.kube

app:
  name: shop
  domain: ${app.name}.local

podman:
  machine:
    cpus: 4
    memory: 8192
    disk: 50
  registry: localhost:5000
  registryTLSVerify: false

k0s:
  version: ""

metallb:
  version: v0.14.9
  poolRange: ""

traefik:
  image: traefik:v3.2.0
  crdUrl: https://raw.githubusercontent.com/traefik/traefik/v3.2.0/docs/content/reference/dynamic-configuration/kubernetes-crd-definition-v1.yml

build:
  defaultEnvironment: local
  version: local
```

See [docs/config-reference.md](docs/config-reference.md) for a full schema reference and [docs/troubleshooting.md](docs/troubleshooting.md) for common fixes.

## Project Layout

```text
forgelet/
├── cmd/forgelet/          # CLI entrypoint
├── internal/cli/          # Cobra commands and helpers
├── .github/workflows/     # CI/release automation
├── docs/                  # User documentation
├── Makefile               # Build/test/release helpers
├── .goreleaser.yml        # Release packaging config
├── go.mod
└── go.sum
```

## Development

```bash
go test ./...
go vet ./...
go build ./...
```

Or use the Makefile:

```bash
make build
make test
```

## Additional Docs

- [Configuration reference](docs/config-reference.md)
- [Troubleshooting guide](docs/troubleshooting.md)
- [Contributing guide](CONTRIBUTING.md)
- [Release checklist](RELEASING.md)
