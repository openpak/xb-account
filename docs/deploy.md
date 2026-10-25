# Deploy: xb-account

Build this directory's Dockerfile as the separate `xb-account` image. Local
verification: `GOWORK=off go test -race ./...` and `GOWORK=off go vet ./...`.
Use a workspace scratch directory as TMPDIR. No production unit is enabled by
this source change.

Configuration (all secrets are service-managed files):

- `XB_LISTEN_ADDR`: private bind address and port, chosen after host port audit.
- `XB_TLS_CERT_FILE`, `XB_TLS_KEY_FILE`: server certificate chain and key.
- `XB_CLIENT_CA_FILE`: dedicated client CA for authorized Xbox workloads.
- `XB_CORE_ADDR`, `XB_CORE_KEY_FILE`: core gRPC target and internal bearer secret.
- `XB_CORE_CA_FILE`, optionally `XB_CORE_SERVER_NAME`: core TLS trust/identity.
- `XB_CORE_PLAINTEXT_INTERNAL=1`: explicit alternative only on the existing
  private container network when core uses its current plaintext gRPC listener.

## Repository

- Module `github.com/openpak/xb-account`, image `ghcr.io/openpak/xb-account` (static, scratch, uid 10001).
- Shared packages: `github.com/openpak/xb-common` (private; see its README for access). Service map: workspace docs `xbox/native-service-architecture.md`.
- Build and test: `GOWORK=off go build ./... && go vet ./... && go test -race ./...` (with `GOPRIVATE=github.com/openpak/*` and `gh auth setup-git`).
- Image: `podman build --secret id=netrc,src=$HOME/.netrc -t ghcr.io/openpak/xb-account:dev .`
- Release: push a `vX.Y.Z` tag; `.github/workflows/release.yml` (tags and manual dispatch only) tests and publishes the multi-arch image. CI reads xb-common with the job's `GITHUB_TOKEN` through a netrc build secret; nothing secret is in git or in the image.
- `deploy/` holds the env example and a Podman quadlet example (digests in it are examples; production pins its own).
