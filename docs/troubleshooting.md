# Forgelet Troubleshooting

## `forgelet up` fails during prerequisites

- Run `forgelet prerequisites --dry-run --verbose` to inspect the commands Forgelet intends to run.
- Ensure your package manager is available (`brew`, `apt-get`, or `dnf`).
- If `pnpm` is still missing after Node install, run `corepack enable && corepack prepare pnpm@latest --activate`.

## `forgelet k0s-install` never reaches Ready

- Check service status with `sudo k0s status`.
- Review node state with `sudo k0s kubectl get nodes -o wide`.
- On native Linux, confirm firewalld rules were applied or temporarily disable conflicting firewall policy.
- Pin a known-good version in `forgelet.yaml` under `k0s.version`.

## `forgelet build` cannot find services

- Confirm Docker Compose exists at `.devenv/docker-compose.yml` or `.devcontainer/docker-compose.yml`.
- Or define `build.services` explicitly in `forgelet.yaml`.
- Use `forgelet config show` to verify `dockerComposeFile`, `build.services`, and environment overrides.

## Podman push fails against `localhost:5000`

- Set `podman.registryTLSVerify: false` for the default local registry.
- Ensure the registry container is running: `podman ps --filter name=registry`.
- Re-run `forgelet deploy` to recreate or start the registry container.

## MetalLB has no assigned IP

- Run `forgelet check` and inspect the MetalLB warning lines.
- Verify `metallb.poolRange` is valid and reachable from your host network.
- Check the controller: `kubectl -n metallb-system get deploy controller`.
- If the pool was auto-generated, verify the underlying VM/native IP is correct.

## Traefik never gets a LoadBalancer IP

- Run `kubectl -n traefik-system get svc traefik -o yaml`.
- Confirm MetalLB is healthy and the address pool exists.
- Re-run `forgelet deploy` after fixing MetalLB.

## DNS does not resolve local domains

- Re-run `forgelet dns`.
- On Linux/WSL, check for `# k0s-<clusterName>` entries in `/etc/hosts`.
- On macOS, verify `/etc/resolver/<domain>` exists.
- Restart local resolvers if needed (`systemd-resolved` on Linux, `dscacheutil -flushcache` on macOS).

## `forgelet dev` fails to start Skaffold

- Ensure `skaffold` is installed and on `PATH`.
- Confirm the cluster is reachable with `forgelet status` or `forgelet check`.
- Confirm `skaffold.yaml` exists if your project expects one.
- Re-run with `--verbose` to inspect the exact command invocation.

## Dashboard token creation fails

- Some Kubernetes versions may not support `kubectl create token`.
- Create the token manually or use a newer kubectl/k0s version.
- The dashboard command still sets up the service account and proxy.

## Need to inspect effective config

Use:

```bash
forgelet config show
```

That output includes defaults plus environment-variable overrides.
