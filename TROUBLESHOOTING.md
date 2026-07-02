# P2P Streaming Troubleshooting Guide

## Key Points for ngrok + P2P Streaming

### 1. **WebSocket Connection Issues**
If OBS and Streamer can't see each other:
- Check browser console (F12) for connection errors
- Look for "WebSocket connection failed" messages
- Verify ngrok tunnel URL matches in all pages

### 2. **TURN Server Debugging**
When using ngrok externally, TURN servers are **critical**:
- The app now includes 4 fallback TURN servers
- Check console for "ICE gathering" messages
- If all IPs appear external, TURN should activate automatically

### 3. **Message Flow for Debugging**

```
Streamer connects:
[JOIN] room=xxx role=streamer network=external
→ Sends "streamer-joined" to all OBS

OBS selects streamer:
OBS sends: {"type":"select-streamer","targetStreamerId":"xxx"}
→ Server routes to streamer

Streamer receives select-streamer:
→ Calls makeOffer()
→ Creates RTCPeerConnection
→ Sends {"type":"offer","sdp":{...}}

OBS receives offer:
→ Calls handleOffer()
→ Creates answer
→ Sends {"type":"answer","sdp":{...},"targetStreamerId":"xxx"}

Connection established:
pc.ontrack fires → video appears
```

### 4. **Common Issues & Fixes**

#### "OBS Waiting for streamers..."
- Streamer hasn't opened `/play/<id>` link
- Streamer clicked "Start sharing" but nothing shows in browser permission
- **Fix:** Check server logs for `[JOIN] room=xxx role=streamer`

#### "Streamer says 'sharing' but OBS doesn't connect"
- OBS not selecting the streamer from dropdown
- WebSocket message not reaching streamer
- **Fix:** Open OBS browser console, click streamer button, look for `select-streamer` in network tab

#### "Connection Failed" on both sides
- TURN servers not connecting
- Firewall blocking WebRTC
- **Fix:** Try different TURN server (check config.go)
- Test with: `Turn=stun:stun.l.google.com:19302` in console

#### "High latency/choppy video"
- Bandwidth too low
- Player.html settings: maxBitrate = 5000000 (5 Mbps)
- **Fix:** Reduce bitrate or increase network capacity

### 5. **Server Logs to Watch**

Start server with verbose output:
```bash
cd c:\Users\jumai\Downloads\new-test\app
.\vodapp.exe 2>&1 | Tee-Object -FilePath debug.log
```

Look for:
- `[JOIN]` - Peer connected
- `[STREAMER LIST]` - Initial peer list sent
- `[NOTIFY OBS]` - OBS notified of new streamer
- `relay: OBS->Streamer` - Message routing works
- `relay: Streamer->OBS` - Offer/Answer flowing

### 6. **Testing with ngrok**

If using ngrok tunnel (e.g., `https://abc123.ngrok.io`):

1. **Streamer page:** `https://abc123.ngrok.io/play/<id>`
2. **OBS page:** `https://abc123.ngrok.io/obs/<id>`
3. Both must use **same domain** (ngrok URL)

### 7. **Quality Streaming Settings**

Current defaults:
- **Video:** 5000 Kbps max (1920x1080, 30-60 fps)
- **Audio:** 256 Kbps
- **Frame rate:** Ideal 30 fps, max 60 fps

For slower networks, modify in `player.html` / `obs.html`:
```javascript
params.encodings[0].maxBitrate = 2000000; // Reduce to 2 Mbps
```

### 8. **Browser Console Commands**

Test from browser console (F12 → Console):

```javascript
// Check WebSocket connection
console.log(ws.readyState); // 0=connecting, 1=open, 2=closing, 3=closed

// Check TURN config loaded
console.log(turnConfig);

// Check ICE candidates
pc.onicecandidate = (e) => console.log(e.candidate);

// Check connection state
console.log(pc.connectionState); // connecting/connected/failed/closed
```

## Quick Start Checklist

- [ ] Server running on correct port (8080)
- [ ] ngrok URL correct and active
- [ ] Streamer opens `/play/<id>` page
- [ ] Streamer clicks "Start sharing"
- [ ] OBS opens `/obs/<id>` page
- [ ] OBS sees streamer in dropdown
- [ ] OBS clicks streamer name
- [ ] Browser console shows `Streamer received: {type:"select-streamer"...}`
- [ ] Video appears in OBS window

If any step fails, check server logs for corresponding message.
