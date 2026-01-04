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

## Quick Start

You can now stand up the control-plane API and exercise it from the cross-platform CLI client. TLS is self-signed by default for easy local testing.

### 1) Run the server

```bash
# From repository root
go run ./server/cmd/vpn-server -addr :8443 -data ./data/state.json
```

- `-addr` controls the HTTPS listener.
- `-data` (optional) persists users, device tokens, and room metadata to JSON so restarts keep state.
- A demo user (`gamer`/`password123`) is seeded automatically; you can also register new accounts via the client.

### 2) Use the client CLI (Windows/macOS/Linux)

Set the server address and, when talking to the self-signed dev server, allow insecure TLS:

```bash
SERVER=https://localhost:8443
CLIENT="go run ./client/cmd/vpn-client --server $SERVER --insecure"

# Register a new user
$CLIENT register

# Login (returns session + device tokens)
$CLIENT login

# Create a room and note the returned room id
$CLIENT create-room

# Join the room (pass SESSION_TOKEN from login)
SESSION_TOKEN=<token-from-login> $CLIENT join-room room-1

# Keepalive and tunnel negotiation probes
$CLIENT keepalive
$CLIENT bootstrap room-1
```

To build native binaries for distribution, use Go cross-compilation (examples):

```bash
# Linux server binary
GOOS=linux GOARCH=amd64 go build -o bin/vpn-server ./server/cmd/vpn-server

# Windows client binary
GOOS=windows GOARCH=amd64 go build -o bin/vpn-client.exe ./client/cmd/vpn-client

# macOS client binary
GOOS=darwin GOARCH=arm64 go build -o bin/vpn-client ./client/cmd/vpn-client
```

## Why Go + TUN + OpenVPN framing?

- **Go** offers straightforward cross-compilation for all target OSes and excellent concurrency primitives for packet handling.
- **TUN** provides layer-3 virtual interfaces, enabling transparent IP routing without modifying games.
- **OpenVPN-compatible framing** eases debugging with existing tools and allows gradual interoperability; UDP-first design minimizes latency.

## Contributing

Roadmap and task issues will be documented under `docs/` as the codebase grows. Contributions should follow Go formatting guidelines and Rust/TypeScript style guides where applicable once those modules land.
