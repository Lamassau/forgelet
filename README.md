# Forgelet

Forgelet is a Go CLI to bootstrap and manage a local k0s-based Kubernetes development environment.

## Highlights

- Single-command environment bootstrap (`forgelet up`)
- Native Linux or Podman VM workflow (macOS/WSL)
- Image build + import flow for local k0s
- CDK8s synth + deploy helpers

## Requirements

- Go 1.22+
- `podman`, `kubectl`, `k0s`/`k0sctl`
- `pnpm`, `npx` (for CDK8s synth)

## Install

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
forgelet --help
```

## Quick Start

1. Ensure project config exists at `.forgelet/forgelet.yaml` (or in a parent directory).
2. Run full bootstrap:

```bash
forgelet up
```

3. Check cluster status:

```bash
forgelet status
```

## Core Commands

- `forgelet up` — full local bootstrap
- `forgelet prerequisites` — install/check required tools
- `forgelet machine-up` — start runtime host
- `forgelet k0s-install` — install/start k0s
- `forgelet kubeconfig` — write kubeconfig
- `forgelet build [env]` — build/push/import images
- `forgelet deploy` — deploy platform services
- `forgelet dns` — configure local DNS
- `forgelet dev` — run skaffold dev loop
- `forgelet reset` — wipe/reinstall k0s and redeploy
- `forgelet destroy` — tear down environment

## Project Layout

```text
forgelet/
├── cmd/forgelet/          # CLI entrypoint
├── internal/cli/          # Cobra commands + app internals
├── go.mod
└── go.sum
```

## Open Source Docs

- Contributing guide: [CONTRIBUTING.md](CONTRIBUTING.md)
- Release checklist: [RELEASING.md](RELEASING.md)
