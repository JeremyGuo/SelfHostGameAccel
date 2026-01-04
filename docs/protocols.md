# Communication Protocols

This document clarifies which protocols and transports are used between the VPN client and server for each feature area.

## Control Plane (management)

| Capability | Direction | Protocol | Transport | Notes |
| --- | --- | --- | --- | --- |
| Authentication (username/password, device token refresh) | Client ↔ Server | gRPC | TLS over TCP | Argon2id password hashes validated server-side; device token issued via gRPC metadata/response. |
| User CRUD (admin) | Client ↔ Server | gRPC | TLS over TCP | Optional admin scope enforced by gRPC interceptor. |
| Room create/update/delete | Client ↔ Server | gRPC | TLS over TCP | Returns overlay subnet, transport preference (UDP/TCP), MTU, and room keys. |
| Room join / membership listing | Client ↔ Server | gRPC | TLS over TCP | Issues per-device virtual IP and session keys; includes keepalive interval. |
| Health/metrics query | Client ↔ Server | gRPC | TLS over TCP | Exposes tunnel status/latency; gated by authentication. |

### Control-plane envelopes

- **Authentication:** Unary gRPC calls with TLS; session token returned via headers; subsequent calls include token in gRPC metadata.
- **Room lifecycle:** Unary gRPC calls; server persists metadata in SQLite/BadgerDB.
- **Keepalive:** Bidirectional gRPC stream used for lightweight heartbeats and state push (e.g., kicked from room, key rotation notice).

## Data Plane (tunnel)

| Capability | Direction | Protocol | Transport | Notes |
| --- | --- | --- | --- | --- |
| Tunnel bootstrap | Client → Server | gRPC stream | TLS over TCP | Negotiates cipher suite, shares ephemeral keys, and confirms transport (UDP default, TCP fallback). |
| Encapsulated traffic | Client ↔ Server | OpenVPN-compatible framing | UDP (default) / TCP (fallback) | Carries encrypted IP packets from TUN; includes sequence numbers and HMAC for integrity. |
| Keepalive (data channel) | Client ↔ Server | OpenVPN-style PING/PONG | Same as data channel | Ensures NAT bindings stay warm; frequency provided in room settings. |
| Re-key / session refresh | Server → Client | Control message inside gRPC keepalive stream | TLS over TCP | Signals client to rotate keys for the data channel. |

### Transport selection

- **UDP first:** Preferred for low latency; uses DTLS-like profile with HMAC for integrity and optional replay protection.
- **TCP fallback:** Uses the same framing over a TLS-protected TCP stream when UDP is blocked; disables redundant TLS inside the data channel to avoid double encryption.

## Device provisioning

| Capability | Direction | Protocol | Transport | Notes |
| --- | --- | --- | --- | --- |
| Virtual IP assignment | Server → Client | gRPC (room join response) | TLS over TCP | Includes subnet, MTU, and DNS (optional). |
| TUN configuration sync | Server ↔ Client | gRPC stream | TLS over TCP | Pushes route updates or DNS changes; client acks receipt. |

## Observability and admin

| Capability | Direction | Protocol | Transport | Notes |
| --- | --- | --- | --- | --- |
| Metrics scrape | Admin → Server | Prometheus HTTP | TLS over TCP (optional mTLS) | Exposes server metrics; not exposed to regular clients. |
| Logs / audit retrieval | Admin → Server | gRPC | TLS over TCP | Restricted to admins; supports pagination. |
