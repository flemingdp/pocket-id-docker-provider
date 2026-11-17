# Pocket ID Docker Provider

**Experimental Go project.** Discovers running Docker containers using `pocketid.*` labels and reconciles native OIDC clients against Pocket ID.

## Status

- Go standard-library Docker Engine API discovery (running containers, periodic polling)
- Declarative Docker labels, validation, and duplicate detection
- Safe dry-run by default; `--apply` enables API writes
- Explicit group restrictions and non-destructive reconciliation
- Unit tests, Dockerfile, Compose examples, GitHub Actions CI
- **Not implemented:** forward-auth, Docker event streaming, client secret distribution

**WARNING:** A `pocketid.mode=forwardauth` label is currently ignored. This service does **not** protect applications behind Traefik. Keep your existing authentication middleware in place.

## Published Docker images

GitHub Actions builds the image automatically for `linux/amd64` and `linux/arm64` and stores it in [GitHub Container Registry](https://github.com/flemingdp?tab=packages).

```sh
docker pull ghcr.io/flemingdp/pocket-id-docker-provider:latest
```

Tags:
- `latest`: current `main` branch (development image)
- `sha-<shortsha>`: commit-specific builds
- `1.2.3`, `1.2`, `1`: generated from a release tag such as `v1.2.3`

Pull requests are built but never pushed. To trigger manually, open **Actions → Publish Docker image → Run workflow**. Publishing uses the built-in `GITHUB_TOKEN`, with `packages: write`; no personal token is needed.

**Package visibility:** GHCR packages may initially be private, even for public repositories. After the first successful publication, open the package under your GitHub profile → **Package settings → Change visibility → Public** if you want anonymous pulls. This action is separate from making the source repository public.

The images are experimental; `forwardauth` is not yet implemented. Do not use this image as an authentication gateway for protected applications.

## Quick start

Copy `.env.example` to `.env`, set `POCKET_ID_URL` and `POCKET_ID_API_KEY`, then:

```sh
go test ./...
go run ./cmd/provider --once            # dry-run
go run ./cmd/provider --once --apply    # makes Pocket ID changes
```

Or use `docker compose up --build`. The Compose example defaults to a single dry-run. To reconcile continuously, remove `--once`; to apply changes, add `--apply`.

### Docker labels

```yaml
labels:
  pocketid.enable: "true"
  pocketid.mode: "oidc"
  pocketid.client.id: "grafana"
  pocketid.client.name: "Grafana"
  pocketid.client.callback: "https://grafana.example.com/login/generic_oauth"
  pocketid.groups: "admins,family"
```

The named groups **must already exist** in Pocket ID. Clients without groups, unknown groups, and non-HTTPS callbacks are rejected. Container removal does not delete the associated client. The provider does not create users or groups.

## Security considerations

- Docker API socket access is privileged even when bind-mounted with `:ro`. Use a restricted Docker socket proxy in production.
- The Pocket ID API key must remain outside Git and container labels.
- Only trusted Docker administrators should set these labels; they control OIDC callbacks and allowed groups.
- Newly generated confidential-client secrets are neither logged nor stored by this prototype. Manage them separately, or use client configurations compatible with public PKCE.
- Do not treat labels or client registration alone as authentication enforcement.

## Roadmap

1. Docker event streaming with reconnect and resync
2. Pinned Pocket ID integration tests
3. Secure client-secret handoff
4. Established forward-auth component integration for legacy applications
5. Multi-architecture releases, SBOM, documentation and security policy

MIT licensed. Contributions welcome.
