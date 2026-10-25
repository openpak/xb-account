# Xbox Account Server

OpenPak's Xbox Account Server.

| | |
| --- | --- |
| Network | Original Xbox |
| Role | Original Xbox identity adapter: resolves gamertags and devices to account-core identities over mutual TLS |
| Depends on | OpenPak account core (gRPC Links and Identity contracts), `xb-common` (private shared packages) |
| Used by | `xb-auth`, `xb-gateway` |
| Image | `ghcr.io/openpak/xb-account` |
| Based on | [Xombie](https://github.com/XombieOnline/xombie) (AGPL-3.0) |
| Licence | AGPL-3.0-only |

## What it serves

| Endpoint | What |
| --- | --- |
| `POST /internal/v1/resolve` | Gamertag to XUID, account and display name, for an active `xbox` link and active account |
| `GET /livez` | Listener liveness only, over the same mutual TLS |
| gRPC `xb-common/xbaccountapi` | `ResolveUser`, `ResolveGamertag`, `ResolveDevice` and the `Revocations` poll stream |

## Build

```sh
podman build --secret id=netrc,src=$HOME/.netrc -t ghcr.io/openpak/xb-account:dev .
```

## Docs

- [Protocol](docs/protocol.md)
- [Evidence](docs/evidence.md)
- [Deploy](docs/deploy.md)
- [Notes](docs/notes.md)
- [Permanent account handles](docs/handles.md)

## Licence

AGPL-3.0-only. See [LICENSE](LICENSE) and [NOTICE](NOTICE).
