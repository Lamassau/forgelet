# Forgelet CLI Enhancements

This plan outlines the implementation of four major productivity features for the Forgelet CLI.

## User Review Required

- Please review the commands and their functionality.
- For secrets, the approach is to read `.env.local` if it exists, and apply it as a generic opaque Kubernetes Secret before `forgelet deploy` or `forgelet dev` spin up the workloads.

## Proposed Changes

### 1. Kubernetes Dashboard Integration (`forgelet dashboard`)
A new command to deploy and access the Kubernetes Dashboard securely.

#### [NEW] internal/cli/dashboard.go
- Create `dashboardCmd`.
- Apply the official Kubernetes Dashboard manifest.
- Create a `ServiceAccount` and `ClusterRoleBinding` for `forgelet-admin`.
- Fetch the token for the `ServiceAccount`.
- Start `kubectl proxy` or a port-forward and optionally open the user's browser using the token.

### 2. Native Secret Management (`.env` to Secrets)
Automatically inject local environment variables as Kubernetes Secrets into the cluster prior to deployment.

#### [MODIFY] internal/cli/helpers.go
- Add a helper function `applyEnvSecrets(cfg)` that looks for `.env.local` or `.env` in the `ProjectDir`.
- If found, parse the key-value pairs (or string map) and create a generic Kubernetes Secret: `kubectl create secret generic forgelet-secrets --from-env-file=.env.local -n <BuildEnv> --dry-run=client -o yaml | kubectl apply -f -`.

#### [MODIFY] internal/cli/deploy.go
- Call `applyEnvSecrets(cfg)` before applying the CDK8s synthesized manifests so that if workloads reference `forgelet-secrets`, they exist.

#### [MODIFY] internal/cli/dev.go
- Ensure `applyEnvSecrets(cfg)` executes before Skaffold runs.

### 3. GitHub Actions Generation (`forgelet init-ci`)
Generate boilerplate CI/CD manifests to use Forgelet inside GitHub Actions.

#### [NEW] internal/cli/initci.go
- Create `initciCmd`.
- Generate a file at `.github/workflows/forgelet-build.yml`.
- The workflow will install go, build [forgelet](file:///home/laith/Repos/Projects/forgelet/internal/cli/helpers.go#27-50), and run `forgelet build prod` (or similar).

### 4. Simplified Observability (`forgelet logs`)
Provide a fast way to tail logs without needing `kubectl` namespace or deployment syntax.

#### [NEW] internal/cli/logs.go
- Create `logsCmd` accepting a service name as an argument.
- Use [helpers.go](file:///home/laith/Repos/Projects/forgelet/internal/cli/helpers.go) to determine the active `BuildEnv` (namespace).
- Map the provided service name to its deployment (e.g. `api-deployment`).
- Run [runKctl(cfg, "logs", "-f", "deployment/<service-deployment>", "-n", cfg.BuildEnv)](file:///home/laith/Repos/Projects/forgelet/internal/cli/helpers.go#273-280).

## Verification Plan

### Automated Tests
- Build test to ensure all new [.go](file:///home/laith/Repos/Projects/forgelet/internal/cli/up.go) files compile and hook into [root.go](file:///home/laith/Repos/Projects/forgelet/internal/cli/root.go).

### Manual Verification
- Test `forgelet dashboard` output and token generation.
- Create a dummy `.env` file and test `forgelet deploy` to ensure the secret is generated in the cluster.
- Run `forgelet init-ci` and verify the file contents.
- Run `forgelet logs api` (assuming an api service exists in `forgelet.yaml`).
