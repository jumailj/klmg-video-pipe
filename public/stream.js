const startButton = document.getElementById('start-btn');
const statusLabel = document.getElementById('status');

let localStream = null;
let pc = null;
let sessionId = null;
let polling = false;

function setStatus(text, state = 'offline') {
  statusLabel.textContent = text;
  statusLabel.className = `status ${state}`;
}

function getSessionId() {
  const parts = window.location.pathname.split('/');
  return parts[parts.length - 1] || 'player01';
}

async function postJSON(url, body) {
  const response = await fetch(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body)
  });
  return response.json();
}

async function fetchJSON(url) {
  const response = await fetch(url);
  return response.json();
}

async function acquireScreen(quality) {
  try {
    const constraints = {
      audio: true,
      video: {
        frameRate: quality === 'standard' ? 60 : 30,
        width: quality === 'high' ? { ideal: 1920 } : { ideal: 1280 },
        height: quality === 'high' ? { ideal: 1080 } : { ideal: 720 }
      }
    };
    localStream = await navigator.mediaDevices.getDisplayMedia(constraints);
    return true;
  } catch (error) {
    console.error(error);
    return false;
  }
}

async function addIceCandidate(candidate) {
  if (!candidate || !candidate.candidate) return;
  const body = {
    role: 'sender',
    id: sessionId,
    candidate
  };
  await postJSON('/ice', body);
}

async function pollViewerCandidates() {
  if (!pc) return;
  if (polling) return;
  polling = true;

  try {
    const poll = async () => {
      const data = await fetchJSON(`/ice?role=sender&id=${encodeURIComponent(sessionId)}`);
      if (data.status !== 'ok' || !Array.isArray(data.data)) {
        return;
      }
      for (const cand of data.data) {
        try {
          await pc.addIceCandidate(cand);
        } catch (err) {
          console.warn('Failed to add remote ICE candidate', err);
        }
      }
    };

    while (pc && (pc.iceConnectionState === 'new' || pc.iceConnectionState === 'checking' || pc.iceConnectionState === 'connected')) {
      await poll();
      await new Promise(res => setTimeout(res, 1000));
      if (pc.iceConnectionState === 'connected' || pc.iceConnectionState === 'completed') break;
    }
  } finally {
    polling = false;
  }
}

async function pollForAnswer() {
  while (pc && (pc.iceConnectionState === 'new' || pc.iceConnectionState === 'checking')) {
    const result = await fetchJSON(`/signal?role=sender&id=${encodeURIComponent(sessionId)}&type=answer`);
    if (result.status === 'ok' && result.data) {
      await pc.setRemoteDescription(result.data);
      return;
    }
    await new Promise(res => setTimeout(res, 1000));
  }
}

async function updateSessionStatus(status) {
  if (!sessionId) return;
  try {
    await postJSON('/api/session', {
      id: sessionId,
      status
    });
  } catch (error) {
    console.warn('Failed to update session status', error);
  }
}

async function startStreaming() {
  if (!localStream) return;
  setStatus('Connecting...', 'connecting');
  startButton.disabled = true;

  sessionId = getSessionId();
  const sessionData = await fetchJSON(`/api/session?id=${encodeURIComponent(sessionId)}`);
  const quality = sessionData.quality || 'standard';

  pc = new RTCPeerConnection({
    iceServers: [{ urls: 'stun:stun.l.google.com:19302' }]
  });

  const constraints = {
    video: {
      frameRate: quality === 'standard' ? 60 : 30,
      width: quality === 'high' ? { ideal: 1920 } : { ideal: 1280 },
      height: quality === 'high' ? { ideal: 1080 } : { ideal: 720 }
    },
    audio: true
  };

  localStream.getTracks().forEach(track => pc.addTrack(track, localStream));

  pc.onicecandidate = event => {
    if (event.candidate) {
      addIceCandidate(event.candidate.toJSON());
    }
  };

  pc.oniceconnectionstatechange = () => {
    if (!pc) return;
    const state = pc.iceConnectionState;
    if (state === 'connected' || state === 'completed') {
      setStatus('Live', 'live');
      updateSessionStatus('live');
    } else if (state === 'connecting') {
      setStatus('Connecting...', 'connecting');
      updateSessionStatus('connecting');
    } else if (state === 'disconnected' || state === 'failed') {
      setStatus('Reconnecting...', 'reconnecting');
      updateSessionStatus('disconnected');
    } else {
      setStatus('Offline', 'offline');
      updateSessionStatus('offline');
    }
  };

  try {
    const offer = await pc.createOffer();
    await pc.setLocalDescription(offer);

    await postJSON('/signal', {
      role: 'sender',
      id: sessionId,
      message: offer
    });

    await updateSessionStatus('connecting');
    pollViewerCandidates();
    await pollForAnswer();
    setStatus('Live', 'live');
    await updateSessionStatus('live');
  } catch (error) {
    console.error(error);
    setStatus('Failed to start streaming.', 'offline');
    await updateSessionStatus('offline');
  }
}

startButton.addEventListener('click', async () => {
  if (!localStream) {
    const sessionId = getSessionId();
    const sessionData = await fetchJSON(`/api/session?id=${encodeURIComponent(sessionId)}`);
    const quality = sessionData.quality || 'standard';

    const ok = await acquireScreen(quality);
    if (!ok) {
      setStatus('Screen share canceled.', 'offline');
      return;
    }
  }
  await startStreaming();
});
