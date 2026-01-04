# VPN Server (Linux)

The server is a Go service that exposes a gRPC control plane for authentication, room lifecycle, and key distribution, and hosts the data-plane gateway for TUN-based tunnels.

## Planned Features

- User and room management backed by SQLite/BadgerDB.
- gRPC API with TLS and token-based session validation.
- TUN-based IP forwarding with OpenVPN-compatible UDP/TCP transport.
- Systemd unit and basic observability (structured logs, Prometheus metrics).

## Development Notes

Implementation is upcoming. Initial tasks:

1. Define protobufs under `server/api/` for auth and room services.
2. Implement command entrypoint in `cmd/vpn-server/` with configuration via environment or config file.
3. Add tunnel engine leveraging `golang.org/x/net` and `github.com/songgao/water` for TUN handling.
4. Provide migrations/tooling for the chosen embedded database.
