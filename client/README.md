# VPN Client (Cross-platform)

The client pairs a cross-platform desktop UI with a Go-based networking core to establish TUN tunnels into per-room virtual LANs.

## Planned Features

- Login and device token storage.
- Room creation/joining, with transport selection (UDP default, TCP fallback).
- TUN provisioning on Windows/macOS/Linux with per-room routing.
- Status/metrics display (latency, packet loss) and quick reconnect.

## Development Notes

Implementation is upcoming. Initial tasks:

1. Build the Go tunnel/control library under `core/` with bindings for the UI shell.
2. Choose the desktop shell (Tauri preferred) in `ui/` and implement auth + room flows.
3. Provide installers/build scripts for each platform and code-sign where applicable.
4. Add integration tests for tunnel loopback against a local server instance.

## Can the UI be built in Go and still look good?

Yes. Several mature options let us ship a polished, cross-platform UI while keeping most code in Go:

- **Wails (Go + webview):** Uses Go for the app logic and HTML/CSS/JS for the UI, enabling modern visuals with minimal runtime overhead.
- **Fyne/Gio (pure Go):** Rendered entirely in Go for a native binary without a webview; provides theming and responsive layouts, though with a smaller widget ecosystem.
- **Hybrid (Go core + Tauri/Electron shell):** Keep networking in Go and expose it to a Rust/JavaScript shell if we want the broadest UI component library.

We can pick Wails if we prefer an all-Go toolchain with a web-quality interface, or Fyne/Gio for a pure-Go deliverable; Tauri remains a fallback if we favor the broader web UI ecosystem.
