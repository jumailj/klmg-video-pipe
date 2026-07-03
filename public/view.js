const remoteVideo = document.getElementById('remote-video');
let pc = null;
let sessionId = null;

function getSessionId() {
  const parts = window.location.pathname.split('/');
  return parts[parts.length - 1] || 'player01';
}

function setStatus(text) {
  if (!remoteVideo) return;
  let statusLabel = document.getElementById('viewer-status');
  if (!statusLabel) {
    statusLabel = document.createElement('div');
    statusLabel.id = 'viewer-status';
    statusLabel.style.position = 'absolute';
    statusLabel.style.top = '16px';
    statusLabel.style.left = '16px';
    statusLabel.style.padding = '10px 14px';
    statusLabel.style.borderRadius = '14px';
    statusLabel.style.background = 'rgba(0, 0, 0, 0.5)';
    statusLabel.style.color = '#fff';
    statusLabel.style.fontSize = '0.95rem';
    statusLabel.style.zIndex = '2';
    document.body.appendChild(statusLabel);
  }
  statusLabel.textContent = text;
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

async function addIceCandidate(candidate) {
  if (!candidate || !candidate.candidate) return;
  await postJSON('/ice', {
    role: 'viewer',
    id: sessionId,
    candidate
  });
}

async function pollSenderCandidates() {
  while (pc && (pc.iceConnectionState === 'new' || pc.iceConnectionState === 'checking')) {
    const result = await fetchJSON(`/ice?role=viewer&id=${encodeURIComponent(sessionId)}`);
    if (result.status === 'ok' && Array.isArray(result.data)) {
      for (const cand of result.data) {
        try {
          await pc.addIceCandidate(cand);
        } catch (err) {
          console.warn('Failed to add sender ICE candidate', err);
        }
      }
    }
    await new Promise(res => setTimeout(res, 1000));
  }
}

async function initViewer() {
  setStatus('Waiting for stream...');
  sessionId = getSessionId();

  pc = new RTCPeerConnection({
    iceServers: [{ urls: 'stun:stun.l.google.com:19302' }]
  });

  pc.ontrack = event => {
    const [stream] = event.streams;
    if (remoteVideo.srcObject !== stream) {
      remoteVideo.srcObject = stream;
      setStatus('Receiving stream');
    }
  };

  pc.onicecandidate = event => {
    if (event.candidate) {
      addIceCandidate(event.candidate.toJSON());
    }
  };

  pc.oniceconnectionstatechange = () => {
    const state = pc.iceConnectionState;
    if (state === 'connected' || state === 'completed') {
      setStatus('Live');
    } else if (state === 'connecting') {
      setStatus('Connecting...');
    } else if (state === 'disconnected' || state === 'failed') {
      setStatus('Reconnecting...');
    } else {
      setStatus('Offline');
    }
  };

  try {
    const offerResponse = await fetchJSON(`/signal?role=viewer&id=${encodeURIComponent(sessionId)}&type=offer`);
    if (offerResponse.status !== 'ok' || !offerResponse.data) {
      setStatus('Waiting for sender to start...');
      const waitForOffer = async () => {
        const result = await fetchJSON(`/signal?role=viewer&id=${encodeURIComponent(sessionId)}&type=offer`);
        if (result.status === 'ok' && result.data) {
          return result.data;
        }
        await new Promise(res => setTimeout(res, 1000));
        return waitForOffer();
      };
      offerResponse.data = await waitForOffer();
    }

    await pc.setRemoteDescription(offerResponse.data);
    const answer = await pc.createAnswer();
    await pc.setLocalDescription(answer);

    await postJSON('/signal', {
      role: 'viewer',
      id: sessionId,
      message: answer
    });

    pollSenderCandidates();
  } catch (error) {
    console.error(error);
  }
}

initViewer();
