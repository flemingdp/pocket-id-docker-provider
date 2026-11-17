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
