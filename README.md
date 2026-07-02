# VOD Dashboard (Go) - Multi-Streamer P2P with TURN

Advanced dashboard supporting multiple concurrent streamers feeding into a single OBS instance.
Each streamer is identified uniquely and detected for local/external network connectivity.

**Key Features:**
- **Multi-Streamer Support** - Multiple users can stream simultaneously to one OBS instance
- **Peer-to-Peer (P2P)** - No media flows through the server (just WebRTC signaling)
- **TURN Server Integration** - Automatic NAT traversal using Google/Twilio TURN servers for cross-network streaming
- **Network Detection** - Automatic detection of whether peers are on local or external networks
- **No External Services** - Built entirely with Go + `gorilla/websocket`

## How it works

### Dashboard Setup
1. Open the dashboard (`/`), type a player name, click **Create link**.
2. You receive two links:
   - **Streamer link** (`/play/<id>`) — Send to each screen-sharer. When opened and "Start sharing" is clicked, the browser's native screen-share picker opens.
   - **OBS link** (`/obs/<id>`) — Add as a **Browser Source** in OBS. Displays available streamers with network info (🏠 Local or 🌐 External). Click any streamer to connect.

### Multi-Streamer Workflow
1. Multiple users open their **Streamer link** and start sharing
2. Open the **OBS link** in a browser source - a dropdown shows all active streamers
3. Click a streamer to connect and receive their screen
4. Switch between streamers by clicking different buttons
5. The server relays only signaling messages (SDP/ICE); video flows P2P

### Network Detection
- **Local Network** (🏠) - Peer uses private IP (e.g., 192.168.x.x) on your LAN
- **External Network** (🌐) - Peer uses public IP from outside your network
  - Automatically uses TURN servers for NAT traversal

## Architecture

```
┌─────────────────┐
│  Streamer 1     │
│  (Browser)      │
└────────┬────────┘
         │ WebRTC P2P
         │
    ┌────┴─────────────┐
    │  Server (Hub)    │  ← Relays only WebRTC signaling
    │  - Tracks rooms  │     (SDP, ICE candidates)
    │  - Tracks peers  │
    └────┬─────────────┘
         │ WebRTC P2P
    ┌────┴────────────────────────┐
    │  OBS Browser Source         │
    │  (Connected to Streamer 1)  │
    └─────────────────────────────┘

    └─────────────────────────────────────┐
    │  Another Streamer can join anytime  │
    │  OBS can switch between them        │
    └─────────────────────────────────────┘
```

## TURN Server Configuration

The app automatically uses public TURN servers:
- **Google STUN** - For basic NAT detection (free, no credentials needed)
- **Twilio TURN** - For NAT traversal (public server, credentials provided)
- **Bistri TURN** - Backup option

These enable streaming between peers on different networks without needing a private TURN server.

## API Endpoints

- `GET /` - Dashboard
- `POST /api/players` - Create new player (JSON: `{"name": "Player Name"}`)
- `GET /api/players` - List all players
- `DELETE /api/players?id=<id>` - Delete player
- `GET /api/turn` - Get TURN/STUN configuration
- `GET /play/<id>` - Streamer page
- `GET /obs/<id>` - OBS viewer page
- `GET /ws?room=<id>&role=streamer&streamerId=<uid>` - WebSocket for streamers
- `GET /ws?room=<id>&role=obs` - WebSocket for OBS viewers

## WebSocket Message Types

### From Streamer:
- `{"type": "offer", "sdp": {...}}` - WebRTC offer
- `{"type": "candidate", "candidate": {...}}` - ICE candidate

### From OBS:
- `{"type": "answer", "sdp": {...}, "targetStreamerId": "<uid>"}` - WebRTC answer
- `{"type": "candidate", "candidate": {...}, "streamerId": "<uid>"}` - ICE candidate

### From Hub to OBS:
- `{"type": "streamer-list", "streamerId": "<uid>", "network": "local|external"}`
- `{"type": "streamer-joined", "streamerId": "<uid>", "network": "local|external"}`
- `{"type": "streamer-left", "streamerId": "<uid>"}`

### From Hub to Streamer:
- `{"type": "streamer-list", "streamerId": "<uid>", "network": "local|external"}` - Other active streamers

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
