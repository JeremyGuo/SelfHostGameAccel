# Architecture Overview

SelfHostGameAccel combines a Linux VPN server with a cross-platform VPN client to create multiple isolated virtual LAN "rooms" for gaming sessions. The design emphasizes low latency (UDP-first), portability, and simple self-hosting.

## Components

- **Control Plane (gRPC over TLS):** Handles authentication, user management, room lifecycle, and key distribution.
- **Data Plane (TUN + UDP/TCP):** Carries tunneled IP packets between clients and server using OpenVPN-compatible framing. UDP is preferred for latency; TCP fallback is optional.
- **Storage:** Embedded database (SQLite or BadgerDB) on the server for users, rooms, device tokens, and audit logs.
- **Client UI:** Cross-platform desktop shell (Tauri/Electron or Go-forward options like Wails) that drives the Go networking core via IPC/FFI.

## Key Flows

1. **Authentication**
   - Username/password verified server-side (Argon2id hashes).
   - Client stores device token to refresh sessions without re-entering credentials.
   - gRPC interceptors enforce TLS and session validation.

2. **Room Creation & Membership**
   - Authenticated user requests a new room; server allocates an overlay subnet and generates session keys.
   - Joining a room returns peer config (keys, virtual IP, MTU) and transport preference (UDP/TCP).
   - Each room is isolated; routing tables and firewall rules prevent cross-room leakage.

3. **Tunnel Establishment**
   - Client brings up a **TUN** interface, applies IP/route settings for the room subnet.
   - Client and server exchange control messages (over gRPC or inline control channel) to bootstrap data-plane encryption.
   - Data channel uses OpenVPN-like encapsulation over UDP; TCP fallback can be toggled per room.

4. **Packet Handling**
   - Packets from the TUN device are framed, encrypted, and sent over the chosen transport.
   - Incoming packets are decrypted, validated, and injected back into the TUN interface.
   - Keep-alives and lightweight congestion control maintain stable sessions for games.

## Security Considerations

- TLS everywhere on the control plane; rotate certificates regularly.
- Argon2id password hashing with per-user salts; optional 2FA in a future milestone.
- Room-scoped keys to ensure isolation; revocation on user removal.
- Minimal privilege: server service runs as non-root after TUN setup; Linux capabilities for TUN access.

## Operational Notes

- Target OS for the server: modern Linux distributions with `systemd` and TUN support (`/dev/net/tun`).
- Client distribution: code-signed binaries for Windows/macOS; package repositories for Linux.
- Observability: structured logging, Prometheus metrics, and optional tracing (OpenTelemetry) for tunnel performance.

## Protocol reference

For a capability-by-capability view of which protocols and transports are used (e.g., gRPC vs. data-plane framing, UDP vs. TCP), see `docs/protocols.md`.

## Client UI stack decision notes

- **Tauri (Rust + webview):** Small runtime, access to modern UI via HTML/CSS, good cross-platform theming and window chrome.
- **Wails (Go + webview):** Keeps the entire stack in Go while still allowing a polished web-style UI; simplifies data binding with the Go tunnel engine.
- **Fyne/Gio (pure Go):** Native Go rendering with no webview; fewer dependencies and closer control of theming, but smaller widget ecosystem. Suitable if we want a fully Go-only deliverable.

## Roadmap (initial)

1. Define protobuf schemas for auth, rooms, and tunnel negotiation.
2. Implement server control plane with SQLite persistence and migration tooling.
3. Ship Go-based tunnel engine (TUN + UDP/TCP) with configuration compatible with OpenVPN tooling.
4. Integrate tunnel engine into Tauri UI; implement login, room CRUD, and connect/disconnect flows.
5. Add automated tests: unit (Go), integration (tunnel loopback), and UI smoke tests.
