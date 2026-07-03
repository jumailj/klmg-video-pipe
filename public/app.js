const playerList = document.getElementById('player-list');
const statTotal = document.getElementById('stat-total');
const statLive = document.getElementById('stat-live');
const statOffline = document.getElementById('stat-offline');
const statLatency = document.getElementById('stat-latency');
const createPlayerButton = document.getElementById('create-player');
const nameInput = document.getElementById('new-player-name');
const teamInput = document.getElementById('new-player-team');
const template = document.getElementById('player-card-template');

const storageKey = 'klmg-streamlink-players';
let players = [];

function randomId() {
  return Math.random().toString(36).slice(2, 10);
}

function getStatusClasses(status) {
  switch (status) {
    case 'live': return ['live', 'LIVE'];
    case 'connecting': return ['connecting', 'CONNECTING'];
    case 'reconnecting': return ['reconnecting', 'RECONNECTING'];
    case 'disconnected': return ['disconnected', 'DISCONNECTED'];
    default: return ['offline', 'OFFLINE'];
  }
}

function formatLink(path) {
  return `${window.location.origin}${path}`;
}

function getQualityStats(quality) {
  switch (quality) {
    case 'high':
      return { resolution: '1920×1080', fps: '30', bitrate: '6.5 Mbps' };
    case 'low':
      return { resolution: '1280×720', fps: '30', bitrate: '3.5 Mbps' };
    default:
      return { resolution: '1280×720', fps: '60', bitrate: '5.2 Mbps' };
  }
}

function savePlayers() {
  localStorage.setItem(storageKey, JSON.stringify(players));
}

function loadPlayersFromStorage() {
  const raw = localStorage.getItem(storageKey);
  if (!raw) return [];
  try {
    return JSON.parse(raw);
  } catch (error) {
    console.warn('Failed to parse saved players', error);
    localStorage.removeItem(storageKey);
    return [];
  }
}

async function fetchServerPlayers() {
  try {
    const response = await fetch('/api/stream-links');
    if (!response.ok) return [];
    return await response.json();
  } catch (error) {
    console.warn('Failed to load players from server', error);
    return [];
  }
}

async function loadPlayers() {
  const storedPlayers = loadPlayersFromStorage();
  if (storedPlayers.length) {
    players = storedPlayers;
    return;
  }

  const serverPlayers = await fetchServerPlayers();
  if (serverPlayers.length) {
    players = serverPlayers.map(player => ({
      id: player.id,
      name: player.name,
      team: player.team || '',
      status: player.status || 'offline',
      quality: player.quality || 'standard'
    }));
    savePlayers();
  }
}

function renderStats() {
  const total = players.length;
  const liveCount = players.filter(p => p.status === 'live').length;
  const offlineCount = total - liveCount;
  statTotal.textContent = total;
  statLive.textContent = liveCount;
  statOffline.textContent = offlineCount;
  statLatency.textContent = players.length ? '28 ms' : '—';
}

function renderPlayers() {
  playerList.innerHTML = '';
  players.forEach(player => {
    const clone = template.content.cloneNode(true);
    const name = clone.querySelector('.player-name');
    const team = clone.querySelector('.player-team');
    const statusDot = clone.querySelector('.status-dot');
    const statusLabel = clone.querySelector('.status-label');
    const resolution = clone.querySelector('.resolution');
    const fps = clone.querySelector('.fps');
    const bitrate = clone.querySelector('.bitrate');
    const latency = clone.querySelector('.latency');
    const packetLoss = clone.querySelector('.packet-loss');
    const runtime = clone.querySelector('.runtime');
    const qualitySelect = clone.querySelector('.quality-select');
    const streamLink = clone.querySelector('.stream-link');
    const viewerLink = clone.querySelector('.viewer-link');
    const copyButtons = clone.querySelectorAll('.copy-btn');
    const resetButton = clone.querySelector('.reset-btn');
    const deleteButton = clone.querySelector('.delete-btn');

    name.textContent = player.name;
    team.textContent = player.team || 'No team';
    const [statusClass, statusText] = getStatusClasses(player.status);
    statusDot.className = `status-dot ${statusClass}`;
    statusLabel.textContent = statusText;

    qualitySelect.value = player.quality || 'standard';
    const stats = getQualityStats(player.quality);
    resolution.textContent = stats.resolution;
    fps.textContent = stats.fps;
    bitrate.textContent = stats.bitrate;
    latency.textContent = player.latency || '34 ms';
    packetLoss.textContent = player.packetLoss || '0.1 %';
    runtime.textContent = player.runtime || '00:00:00';
    streamLink.value = formatLink(`/player/${player.id}`);
    viewerLink.value = formatLink(`/view/${player.id}`);

    qualitySelect.addEventListener('change', async () => {
      player.quality = qualitySelect.value;
      savePlayers();
      await fetch('/api/session', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id: player.id, quality: player.quality })
      });
      renderStats();
    });

    copyButtons.forEach(button => {
      button.addEventListener('click', () => {
        const type = button.dataset.copy;
        const value = type === 'stream' ? streamLink.value : viewerLink.value;
        navigator.clipboard.writeText(value);
      });
    });

    resetButton.addEventListener('click', () => {
      player.status = 'disconnected';
      savePlayers();
      renderPlayers();
      renderStats();
    });

    deleteButton.addEventListener('click', () => {
      players = players.filter(p => p.id !== player.id);
      savePlayers();
      renderPlayers();
      renderStats();
    });

    playerList.appendChild(clone);
  });
}

createPlayerButton.addEventListener('click', async () => {
  const name = nameInput.value.trim();
  if (!name) {
    nameInput.focus();
    return;
  }

  const id = randomId();
  const team = teamInput.value.trim();
  players.push({
    id,
    name,
    team,
    status: 'offline',
    quality: 'standard'
  });

  savePlayers();

  await fetch('/api/session', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ id, name, team, quality: 'standard', status: 'offline' })
  });

  nameInput.value = '';
  teamInput.value = '';
  renderPlayers();
  renderStats();
});

(async () => {
  await loadPlayers();
  renderPlayers();
  renderStats();
})();
