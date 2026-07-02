# Multi-Streamer P2P Implementation Guide

## What Changed

### 1. Multi-Streamer Architecture
**Before:** One player (streamer) per room, one OBS viewer per room
**After:** Multiple streamers can coexist in one room, one OBS can select/switch between them

**Changes:**
- Hub now tracks multiple streamers per room with unique IDs
- Each streamer generates a random ID: `streamerID = Math.random().toString(36).substr(2, 9)`
- WebSocket role changed: `role=player` → `role=streamer`
- Added `streamerId` parameter to WebSocket: `/ws?room=<id>&role=streamer&streamerId=<uid>`

### 2. TURN Server Configuration
**New file:** `config.go`
- Contains `TURNConfig` struct with public TURN servers
- `GetTURNConfig()` returns list of STUN/TURN servers:
  - Google STUN (free)
  - Twilio TURN (public credentials)
  - Bistri TURN (backup)

**Changes:**
- Main.go passes TURN config to templates: `/api/turn` endpoint
- Player/OBS pages use: `{{.TurnConfig | json}}`
- WebRTC initialization: `new RTCPeerConnection({iceServers: turnConfig.servers})`

### 3. Network Detection
**New:** Automatic local/external network detection

**Implementation:**
- `getNetworkType()` function in hub.go
- Checks if peer IP is loopback/private: "local"
- Otherwise: "external"
- Sent with `streamer-joined` and `streamer-list` messages

### 4. Hub Signaling Logic

**New message types:**
- `streamer-list` - Initial list of all active streamers when OBS joins
- `streamer-joined` - Notification when a new streamer connects
- `streamer-left` - Notification when a streamer disconnects
- `select-streamer` - OBS selecting which streamer to connect to

**Message routing:**
- Streamers send `offer` + `candidate` → Hub enriches with `streamerId` → All OBS viewers get it
- OBS sends `answer` + `candidate` with `targetStreamerId` → Hub routes to that specific streamer

### 5. Frontend Templates

#### player.html (Streamer)
- Displays streamer ID and network status
- Uses unique `streamerID` for identification
- Listens for other streamers joining (informational)
- Sends offer when OBS connects

#### obs.html (Viewer)
- Shows dropdown list of available streamers
- Network badges: 🏠 Local or 🌐 External
- Click to select/switch streamer
- Displays connected streamer ID in status bar
- Auto-reconnects on disconnect

## How to Use

### For Streamers (Screen Sharers)
1. Open streamer link: `http://localhost:8080/play/<player-id>`
2. Click "Start sharing my screen"
3. Select screen/window to share
4. See streamer ID and network status
5. Screen is now available for OBS

### For OBS
1. Add Browser Source: `http://localhost:8080/obs/<player-id>`
2. See list of available streamers with network type
3. Click a streamer name to connect
4. Switch between streamers by clicking different names
5. Source shows their screen in OBS

### Multiple Streamers Scenario
1. Have 3 people open their streamer links
2. All click "Start sharing"
3. OBS shows 3 buttons (one per streamer)
4. OBS can switch between them instantly
5. Clicking a different streamer switches the video feed

## Technical Details

### P2P vs. Relay
- **Signaling:** Goes through server (small JSON messages)
- **Video:** Flows directly between streamer and OBS (P2P)
- Server is NOT in the media path (no bandwidth bottleneck)

### Cross-Network Streaming
1. User1 (Local LAN) and User2 (Different network)
2. Both connect to same room
3. Hub detects: User1 = "local", User2 = "external"
4. OBS shows both with network badges
5. WebRTC + TURN servers handle NAT traversal automatically
6. Video still flows P2P, not through server

### Streamer ID Format
- Random alphanumeric: e.g., "a7f3k2x9"
- Shown in UI (first 8 chars when truncated)
- Persists only while streamer is connected
- New ID generated on each connection

## Files Modified

1. **hub.go**
   - Added `Streamer` struct for tracking
   - Added `network` field to `client`
   - Updated `join()`, `leave()`, `relay()` functions
   - Added `getNetworkType()` function
   - Changed WebSocket parameters in `ServeWS()`

2. **config.go** (NEW)
   - TURN/STUN server configuration
   - `GetTURNConfig()` function

3. **main.go**
   - Added `/api/turn` endpoint
   - Pass TURN config to templates
   - Updated player/OBS page handlers

4. **player.html**
   - Changed role: `player` → `streamer`
   - Added streamer ID generation
   - Added network status display
   - Integrated TURN config
   - Updated WebSocket URL with `streamerId`

5. **obs.html**
   - Multi-streamer dropdown UI
   - Network badges (🏠 Local / 🌐 External)
   - Streamer selection logic
   - Message routing by `streamerId`
   - Integrated TURN config

6. **README.md**
   - Updated with multi-streamer architecture
   - Added TURN configuration info
   - Updated API endpoints documentation
   - Added WebSocket message types

## Troubleshooting

**OBS showing "Waiting for streamers..."**
- Check that at least one streamer has opened `/play/<id>` and clicked "Start sharing"
- Verify they're in the same room (same `<id>`)

**Connection fails from external network**
- TURN servers are configured automatically
- If still failing, try the fallback TURN servers in `config.go`
- Check if ISP blocks WebRTC (some corporate networks do)

**Video freezes when switching streamers**
- This is expected - new WebRTC connection being negotiated
- Usually completes within 2-5 seconds

**Multiple OBS viewers in same source**
- Not supported in this design (one OBS viewer per room)
- Create multiple browser sources with different player IDs for multiple OBS windows

## Future Enhancements

1. **Custom TURN Servers:** Add environment variables to specify private TURN servers
2. **Composite Stream:** Merge multiple streamer feeds into one canvas for single OBS source
3. **Recording:** Add timestamp-based recording per streamer
4. **Statistics:** Dashboard showing active streamers, connection stats
5. **Permissions:** Allow/deny specific OBS viewers from connecting
6. **Audio Only:** Option to share audio without video
