# VOD Dashboard (Go)

Minimal dashboard to generate per-player links. A player opens their link, the
browser asks them to pick something to share, and their video streams live
via WebRTC into an OBS Browser Source labeled with their name.

No database, no external services besides a public STUN server for NAT
traversal. Pure Go standard library + `gorilla/websocket` for signaling only
— actual video never touches the Go process, it flows browser-to-browser
(player → OBS), so it works fine over LAN or the internet.

## How it works

1. Open the dashboard (`/`), type a player name, click **Create link**.
2. You get two links:
   - **Player link** (`/play/<id>`) — send this to the player. When they
     open it and click "Start sharing", the browser's native screen-share
     picker opens.
   - **OBS link** (`/obs/<id>`) — add this as a **Browser Source** in OBS
     (one source per player). It shows their shared video with their name
     overlaid in the corner.
3. The Go server only relays WebRTC signaling messages (SDP/ICE) over a
   WebSocket at `/ws?room=<id>&role=player|obs` — it pairs up the player and
   the matching OBS source automatically.

Note: each room supports one player + one OBS viewer at a time (i.e. one
browser source per player in your OBS scene, which matches the intended
use). If the OBS source reloads, it auto-reconnects and re-negotiates.

## Build

Requires Go 1.21+ (https://go.dev/dl/).

```bash
go mod tidy   # first time only, downloads gorilla/websocket
```

### Linux
```bash
go build -o vodapp
./vodapp -addr :8080
```

### Windows
Build on Windows directly:
```powershell
go build -o vodapp.exe
vodapp.exe -addr :8080
```

Or cross-compile from Linux/Mac:
```bash
GOOS=windows GOARCH=amd64 go build -o vodapp.exe
```

Then open `http://localhost:8080` (or the machine's LAN IP if players are on
other devices, e.g. `http://192.168.1.20:8080`).

## Notes / things you may want to extend later

- Screen sharing requires **HTTPS or localhost** in most browsers. On a LAN,
  either run behind a reverse proxy with TLS, or use a tool like `ngrok`/
  Cloudflare Tunnel when players are remote.
- Player list is in-memory only (resets on restart) — intentionally minimal,
  per the brief. Swap `store.go` for a file/DB-backed store if you need
  persistence.
- "Score" retrieval wasn't fully clear from the brief — this build focuses on
  getting each player's screen share into OBS labeled with their name. If you
  meant an actual game score/stat feed, that would be a separate data source
  layered into the OBS overlay — happy to add it once it's clearer where the
  score comes from (a game API, manual entry, etc).
