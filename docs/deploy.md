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

## State

State lives in Postgres: schema `xb_account` of the shared `xbox` database, reached through `XB_ACCOUNT_DATABASE_URL` (required; the service refuses to start without it; the pool is capped at 3 connections). The service creates its schema and tables on start. Until v0.1.39 the state was an embedded SQLite file (`XB_DB_PATH`); that file is no longer read, it is kept on the volume as the pre-migration copy. The core's Postgres is never shared. `XB_PEPPER_FILE` holds >=32 bytes of hex used to key the device-proof MAC (proofs are never stored).

## Account erasure

`DELETE /internal/accounts/{account_id}/data` with `Authorization: Bearer $ACCOUNT_ERASE_KEY`, on `ERASE_ADDR` (default `:9099`, container network only; not served without the key). Reply `{"service":"xb-account","rows":N}`. Website step name: `xb-account`; call it AFTER the other Xbox services (xb-social, xb-xmessaging, xb-stfd resolve Xbox ids through the core link, not through this service, so the order is a precaution only).

Erased: `users`, `gamertag_aliases`, `gamertag_audit`, `user_settings`, `auto_identities`, `name_reservations`, `devices`, `provisions`, `enrolments`, `vouchers` and the account's `revocations`. The `account_deleted` event runs the same erase but leaves the newly appended deletion revocation (account uuid and reason, no other data) so gateways drop live sessions. Kept: `cursors` and `processed_events` (event ids only). One log line per erasure.
