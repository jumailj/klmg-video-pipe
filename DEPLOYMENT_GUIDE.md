# Deployment & Usage Guide

## What You Have

✅ **Multi-Streamer P2P WebRTC System**
- Multiple streamers can share screens simultaneously
- OBS viewers can switch between streamers in real-time
- P2P architecture: video flows directly browser-to-browser
- Server only handles signaling (small JSON messages)
- Cross-network support via TURN servers (ngrok compatible)

## Quick Start (Local Testing)

### 1. Start Server
```powershell
cd c:\Users\jumai\Downloads\new-test\app
.\vodapp.exe
# Should see: "VOD dashboard listening on :8080"
```

### 2. Open Dashboard
```
http://localhost:8080/
```

### 3. Create a Stream Link
- Enter name (e.g., "My Stream")
- Click "Create link"
- Two URLs generated:
  - `/play/<id>` → Send to screensharer
  - `/obs/<id>` → Add to OBS

### 4. Test with Browsers

**Browser 1 (Streamer):**
```
http://localhost:8080/play/abc123xyz
→ Click "Start sharing my screen"
→ Select screen to share
→ See "Streamer ID: ..." in green box
```

**Browser 2 (OBS):**
```
http://localhost:8080/obs/abc123xyz (same ID as Browser 1)
→ See dropdown list with streamer
→ Click the streamer name
→ Video appears on screen
```

## Testing Multiple Streamers

### Scenario: 3 Streamers → 1 OBS

1. **Create 3 player links:**
   - Stream 1: `/play/id1`
   - Stream 2: `/play/id2`  
   - Stream 3: `/play/id3`

2. **All use SAME `/obs/<id>`** (pick any ID, e.g., id1)

3. **Start all 3 streamers:**
   - Browser 1: Open `/play/id1` → Start sharing
   - Browser 2: Open `/play/id2` → Start sharing
   - Browser 3: Open `/play/id3` → Start sharing

4. **OBS combines them:**
   - Browser 4: Open `/obs/id1`
   - See dropdown with all 3 streamers
   - Click to switch between them
   - Add multiple browser sources for layout

## External Network Testing (ngrok)

### Setup ngrok Tunnel
```bash
# Download: https://ngrok.com/download
# Terminal 1: Start ngrok
ngrok http 8080
# Copy URL: https://abc123.ngrok.io
```

### Access via ngrok URL
```
Streamer: https://abc123.ngrok.io/play/id1
OBS:      https://abc123.ngrok.io/obs/id1
```

### Across Different Networks
- Streamer on home WiFi
- OBS on mobile hotspot
- All through ngrok URL
- TURN servers handle NAT automatically

## OBS Integration

### Add as Browser Source

1. **OBS Studio → Scene → Add Source → Browser**
2. **URL:** `http://localhost:8080/obs/<id>` (or ngrok URL)
3. **Width:** 1920, **Height:** 1080
4. **Refresh rate:** 30 Hz
5. **Click "Interact" if nothing appears**

### Multiple Streamers in OBS

**Option 1: Multiple Browser Sources**
- Add 3 separate browser sources
- Each uses: `http://localhost:8080/obs/<id>`
- Use same room ID for all
- Arrange in scene layout

**Option 2: Scene Switching**
- Create scene per streamer
- Each scene has one browser source
- Switch scenes to change streamers

## Quality Settings

### Default Configuration
- **Resolution:** 1920×1080
- **Frame Rate:** 30-60 fps
- **Video Bitrate:** 5 Mbps (1-5 Mbps range)
- **Audio Bitrate:** 256 Kbps

### For Slow Networks
Edit `player.html`, find `makeOffer()`:
```javascript
// Change this line:
params.encodings[0].maxBitrate = 5000000;
// To:
params.encodings[0].maxBitrate = 2000000; // 2 Mbps
```

### For High Quality
```javascript
params.encodings[0].maxBitrate = 10000000; // 10 Mbps (requires good bandwidth)
```

## Troubleshooting

### "OBS shows 'Waiting for streamers...'"
- ✓ Streamer opened `/play/<id>` link?
- ✓ Streamer clicked "Start sharing"?
- ✓ Same room ID used for OBS?
→ Check server logs for `[JOIN]` message

### "OBS and Streamer don't connect"
- ✓ Open browser F12 console
- ✓ Click streamer button
- ✓ Look for `"OBS selecting streamer"`
- ✓ Check server logs for `relay: OBS->Streamer`
→ If not appearing, WebSocket may be blocked

### "Video is choppy"
- Try reducing bitrate (see Quality Settings)
- Check network bandwidth
- Close other apps using network

### "Port 8080 already in use"
```powershell
Get-Process vodapp | Stop-Process -Force
```

## Architecture Reminder

```
┌──────────────────────┐
│   Streamer 1         │
│   (Screen Share)     │
└──────────┬───────────┘
           │ WebRTC P2P (video)
           │ WebSocket (signaling)
    ┌──────▼──────┐
    │ Server Hub  │  ← Only handles signaling (tiny messages)
    │ (Go App)    │
    └──────┬──────┘
           │ WebRTC P2P (video)
           │ WebSocket (signaling)
┌──────────▼───────────┐
│   OBS Browser        │
│   (Multiple streams) │
└──────────────────────┘
```

## Files & Directories

```
/app
├── vodapp.exe                    # Compiled application
├── *.go                          # Server code
├── go.mod, go.sum              # Dependencies
├── web/
│   ├── templates/
│   │   ├── dashboard.html      # Player list
│   │   ├── player.html         # Streamer page
│   │   └── obs.html            # OBS viewer page
│   └── static/                 # CSS/JS (if added)
├── README.md                   # Full documentation
├── IMPLEMENTATION.md           # Technical details
├── TROUBLESHOOTING.md         # Debugging guide
├── FIX_SUMMARY.md             # What was fixed
└── QUICK_REFERENCE.txt        # Quick commands
```

## Production Deployment

### Option 1: Cloud VPS (DigitalOcean, Linode, AWS)
```bash
# SSH into server
ssh user@your-server.com

# Download and run
wget https://your-repo/vodapp
chmod +x vodapp
./vodapp &

# Access via: http://your-server.com:8080
```

### Option 2: Docker
```dockerfile
FROM golang:1.22 as build
WORKDIR /app
COPY . .
RUN go build

FROM alpine:latest
COPY --from=build /app/vodapp /
COPY --from=build /app/web /web
EXPOSE 8080
CMD ["/vodapp"]
```

### Option 3: Reverse Proxy (Nginx)
```nginx
server {
    listen 443 ssl http2;
    server_name your-domain.com;
    
    location / {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

## Monitoring

### Watch Server Logs
```bash
.\vodapp.exe 2>&1 | Tee-Object -FilePath server.log
```

### Key Metrics to Watch
- Active connections per room
- Message relay count
- Error rate
- Connection duration

## Stopping the Server

```powershell
# Gracefully (Ctrl+C in terminal)
# Or:
Get-Process vodapp | Stop-Process
```

## Support & Debugging

1. **Check browser console:** F12 → Console tab
2. **Check server logs:** Look for `[RELAY]` messages
3. **Network tab:** F12 → Network → WS (WebSocket)
4. **Connection state:** Console: `console.log(pc.connectionState)`

## Performance Notes

- **Bandwidth:** ~1-5 Mbps per stream (configurable)
- **Latency:** 50-200ms typical (P2P depends on network)
- **Concurrent Streams:** Limited by network, not server
- **Server CPU:** Minimal (only signaling)
- **Server Memory:** ~50-100MB for 100+ concurrent viewers

## Security Considerations

- ✓ No authentication (for development)
- ✓ No SSL/TLS in default setup
- → Use Nginx/reverse proxy in production
- → Add password protection if needed
- → Use HTTPS (Let's Encrypt)
- → Whitelist room IDs if public

## Credits

Built with:
- **Go** - Server implementation
- **Gorilla WebSocket** - Real-time signaling
- **WebRTC** - Peer-to-peer streaming
- **Google STUN/TURN** - NAT traversal

---

**Version:** 2.0 (Multi-Streamer P2P)  
**Last Updated:** 2026-07-03  
**Status:** Ready for Testing
