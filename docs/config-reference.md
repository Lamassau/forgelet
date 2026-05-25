# Forgelet Configuration Reference

Forgelet reads YAML from `.devenv/forgelet.yaml` by default. Values may also be overridden by environment variables, and some string values support variable interpolation.

## Variable Expansion

The following placeholders are supported in string fields:

- `${app.name}`
- `${service.name}`
- `${cluster.name}`
- `${env}`
- `${HOME}`

## Top-Level Structure

```yaml
cluster:
  clusterName: k0s-dev
  kubeConfigDir: ~/.kube

app:
  name: my-app
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
  services: []

deploy:
  deployments: []
  platformImages: []

localPath:
  version: v0.0.31
```

## Keys

### `cluster`

#### `cluster.clusterName`
Name of the k0s cluster and Podman machine. Default: `k0s-dev`.

#### `cluster.kubeConfigDir`
Directory for generated kubeconfig files. `~` and `${HOME}` are expanded.

### `app`

#### `app.name`
Logical application name. Used in image naming and namespace naming.

#### `app.domain`
Base local domain used by `forgelet dns`. Supports variable expansion.

### `podman`

#### `podman.machine.cpus`
CPU count for the Podman machine on macOS/WSL. Default: `4`.

#### `podman.machine.memory`
Memory in MB for the Podman machine. Default: `8192`.

#### `podman.machine.disk`
Disk size in GB for the Podman machine. Default: `50`.

#### `podman.registry`
Registry hostname used for local image pushes. Default: `localhost:5000`.

#### `podman.registryTLSVerify`
If `true`, Forgelet pushes with TLS verification enabled. If you keep the default `localhost:5000`, this should usually remain `false`.

### `k0s`

#### `k0s.version`
Optional pinned k0s version. When empty, Forgelet installs the latest version and prints a hint with the detected version.

### `metallb`

#### `metallb.version`
Version of MetalLB to install. Default: `v0.14.9`.

#### `metallb.poolRange`
Static IP range in `START-END` form. If omitted, Forgelet computes a range based on the k0s host IP.

### `traefik`

#### `traefik.image`
Traefik image used by your manifests.

#### `traefik.crdUrl`
URL to the Traefik CRD manifest applied during `forgelet deploy`.

### `build`

#### `build.defaultEnvironment`
Default environment for build and deploy naming. Common values: `local`, `dev`, `prod`.

#### `build.version`
Default image tag. If omitted, Forgelet falls back to the active environment name.

#### `build.services`
Optional explicit service definitions. If omitted, Forgelet tries to parse Docker Compose build entries.

Example:

```yaml
build:
  services:
    - name: api
      image: ${app.name}-${service.name}
      description: API service
      dockerfile: Dockerfile
      context: ./services/api
      devTarget: dev
      prodTarget: prod
      tag: ${env}
```

Supported fields:

- `name`
- `image`
- `description`
- `dockerfile`
- `context`
- `devTarget`
- `prodTarget`
- `tags`
- `tag`

### `deploy`

#### `deploy.deployments`
List of deployment names to wait for and optionally restart after deploy/build.

#### `deploy.platformImages`
Extra images to pull and import into k0s containerd.

### `localPath`

#### `localPath.version`
Version of Rancher local-path-provisioner to install. Default: `v0.0.31`.

## Discovery Rules

Forgelet searches upward from the current working directory for:

1. `.devenv/forgelet.yaml`
2. `.forgelet/forgelet.yaml`
3. `go.mod` as a project-root fallback

Use `--config` to bypass discovery.

## Related Commands

- `forgelet config show`
- `forgelet init`
- `forgelet up`
- `forgelet deploy`
- `forgelet check`
