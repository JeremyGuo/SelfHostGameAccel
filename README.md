# SelfHostGameAccel

SelfHostGameAccel is a self-hosted VPN solution tailored for multiplayer game acceleration and LAN simulation. The platform contains:

- **VPN Server (Linux service):** Manages identity, room/VPN topology, and data-plane gateways for game traffic.
- **Cross-platform VPN Client (Windows/macOS/Linux UI):** Provides authentication, room creation/joining, and transparent tunneling via a virtual network interface (TUN).

The goal is to let players spin up isolated VPN "rooms" that act like distinct LANs while keeping deployment lightweight for personal servers.

## Technology Stack (proposed)

- **Language:** Go for both server and client core networking (mature TUN/TAP support, easy cross-compilation, strong concurrency model).
- **Control plane:** gRPC over TLS for client ↔ server management (auth, room lifecycle, key distribution).
- **Data plane:**
  - Default: **UDP** transport for low latency; optional TCP fallback for restrictive networks.
  - **TUN** device on both ends for transparent IP tunneling.
  - Reuse **OpenVPN-compatible framing** to simplify future interoperability and allow leveraging existing tooling.
- **Client UI:** Cross-platform desktop using **Tauri** (Rust + webview) or **Wails** (Go + webview) to pair a modern UI with the Go tunnel engine; pure-Go toolkits like **Fyne/Gio** remain optional for a native look without HTML/CSS.
- **Persistence:** Lightweight embedded store on the server (BadgerDB/SQLite) for user accounts, room metadata, and session keys.
- **Auth:** Username/password with salted hashing (Argon2id) plus per-device tokens; ready for upgrade to OIDC in later iterations.

## Features (initial scope)

- Account-based authentication managed on the server; user CRUD available via client UI.
- Create/join multiple VPN rooms; each room maps to an isolated overlay subnet and key set.
- Configurable transport (UDP/TCP) per room for network compatibility.
- Virtual interface provisioning (TUN) on clients and server for transparent routing.
- Minimal server footprint; single binary service with systemd unit template for Linux.

## Repository Layout (planned)

```
SelfHostGameAccel/
├─ README.md
├─ docs/
│  └─ architecture.md      # High-level design, protocols, and flows
│  └─ protocols.md         # Protocol/transport matrix for each capability
├─ server/
│  ├─ README.md            # Server-specific setup and development notes
│  └─ cmd/                 # Go entrypoints (vpn-server)
│
└─ client/
   ├─ README.md            # Client UI + networking core integration notes
   ├─ ui/                  # Tauri/Electron frontend
   └─ core/                # Go tunnel + control-plane library
```

## Quick Start (design phase)

Implementation is forthcoming. The initial milestones:

1. Define protobuf APIs for auth, room management, and session negotiation.
2. Implement Go control-plane service (gRPC + TLS) with SQLite persistence.
3. Implement Go tunnel engine (TUN + UDP/TCP) with OpenVPN-compatible framing.
4. Wrap tunnel engine for the cross-platform UI (Tauri) and build room management flows.

## Why Go + TUN + OpenVPN framing?

- **Go** offers straightforward cross-compilation for all target OSes and excellent concurrency primitives for packet handling.
- **TUN** provides layer-3 virtual interfaces, enabling transparent IP routing without modifying games.
- **OpenVPN-compatible framing** eases debugging with existing tools and allows gradual interoperability; UDP-first design minimizes latency.

## Contributing

Roadmap and task issues will be documented under `docs/` as the codebase grows. Contributions should follow Go formatting guidelines and Rust/TypeScript style guides where applicable once those modules land.
