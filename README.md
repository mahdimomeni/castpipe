# Castpipe

> Zero-config local developer drop utility. Instantly beam code snippets, terminal commands, and streaming file drops between developer machines—or between multiple local terminal instances during development.

---

## Testing Locally in Separate Terminals

Castpipe includes built-in multi-instance support and an inter-process discovery provider (`os.TempDir()/castpipe_local_peers/`), allowing you to run two or more instances simultaneously on the same machine without port collisions or mDNS multicast loopback blocking.

### Quick Start (Two Terminals)

Open two separate terminals (e.g. PowerShell, Windows Terminal, or Command Prompt):

#### Terminal 1:
```powershell
go run . -name "Dev-Alpha"
```
*Alternatively with npm:*
```bash
npm run dev:peer1
```

#### Terminal 2:
```powershell
go run . -name "Dev-Beta"
```
*Alternatively with npm:*
```bash
npm run dev:peer2
```

Both instances will start immediately:
- **Unique window titles**: Distinctly labeled as `Castpipe - Dev-Alpha` and `Castpipe - Dev-Beta` in the Windows taskbar and Alt+Tab.
- **Isolated drop folders**: Files received by `Dev-Alpha` are saved to `~/Downloads/DevDrop-Dev-Alpha`, while `Dev-Beta` saves to `~/Downloads/DevDrop-Dev-Beta`.
- **Automatic mutual discovery**: `Dev-Alpha` and `Dev-Beta` will instantly see each other in their Peer sidebars.

---

## Live Frontend Development with Wails

If you are developing the Svelte frontend with hot module reload:

#### Terminal 1 (Vite Dev Server + App):
```powershell
wails dev -appargs "-name Dev-Alpha"
```

#### Terminal 2 (Second Instance):
```powershell
go run . -name "Dev-Beta"
```

---

## CLI Flags & Environment Variables

Every instance can be customized via command-line flags or environment variables:

| Flag | Shorthand | Environment Variable | Default | Description |
| :--- | :--- | :--- | :--- | :--- |
| `--name` | `-n` | `CASTPIPE_NAME` | Machine Hostname | Custom instance display name and window title |
| `--port` | `-p` | `CASTPIPE_PORT` | `0` (Ephemeral) | Custom HTTP receiver port |
| `--drop-dir` | `-d` | `CASTPIPE_DROP_DIR` | `~/Downloads/DevDrop[-<Name>]` | Destination directory for inbound drops |

### Example with Custom Port and Download Directory:
```powershell
go run . -name "Worker-1" -port 9090 -drop-dir "./my-drops"
```

---

## Architecture & How Local Testing Works

1. **Ephemeral TCP Ports**: Each instance binds to an ephemeral TCP port (`:0` or specified via `-port`), eliminating `bind: address already in use` conflicts.
2. **Dual Discovery Architecture**:
   - **LAN Network Discovery**: Uses mDNS (`_devdrop._tcp.local`) for cross-device discovery on the physical Wi-Fi/Ethernet network.
   - **Local Inter-Process Discovery**: Uses atomic heartbeat presence files in `os.TempDir()/castpipe_local_peers/` with loopback routing (`127.0.0.1`). This bypasses OS-level multicast loopback restrictions on Windows while enabling instant mutual discovery offline or on airplanes.
3. **Automatic Cleanup & Stale Pruning**:
   - When an instance is closed, its presence file is immediately removed.
   - If an instance is killed abruptly (`Ctrl+C`), other running instances automatically prune the stale entry upon TTL expiration.

---

## Running Automated Tests

Run the full Go backend test suite:
```powershell
go test -v ./backend
```

Run the end-to-end multi-instance dual terminal integration test:
```powershell
go test -v -run TestLocalDualTerminalIntegration ./backend
```

---

## Production Build

To build a standalone redistributable `.exe`:
```powershell
wails build
```
The compiled executable will be output to `build/bin/castpipe.exe`.
