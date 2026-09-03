(function () {
  'use strict';

  const tracks = [];
  let filtered = [];
  let currentIndex = -1;
  let sound = null;
  let isPlaying = false;
  let shuffle = false;
  let repeat = false;
  let updateTimer = null;

  const els = {
    list: document.getElementById('track-list'),
    search: document.getElementById('search'),
    trackInfo: document.getElementById('track-info'),
    progressContainer: document.getElementById('progress-container'),
    progressBar: document.getElementById('progress-bar'),
    currentTime: document.getElementById('current-time'),
    duration: document.getElementById('duration'),
    btnPlay: document.getElementById('btn-play'),
    btnPrev: document.getElementById('btn-prev'),
    btnNext: document.getElementById('btn-next'),
    btnShuffle: document.getElementById('btn-shuffle'),
    btnRepeat: document.getElementById('btn-repeat'),
    volume: document.getElementById('volume'),
  };

  function formatTime(seconds) {
    if (!isFinite(seconds) || seconds < 0) return '0:00';
    const m = Math.floor(seconds / 60);
    const s = Math.floor(seconds % 60);
    return `${m}:${s.toString().padStart(2, '0')}`;
  }

  function formatSize(bytes) {
    if (!bytes) return '';
    const units = ['B', 'KB', 'MB', 'GB'];
    let i = 0;
    let size = bytes;
    while (size >= 1024 && i < units.length - 1) {
      size /= 1024;
      i++;
    }
    return `${size.toFixed(1)} ${units[i]}`;
  }

  async function loadTracks() {
    try {
      const res = await fetch('api/tracks');
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      tracks.length = 0;
      tracks.push(...data);
      filter(els.search.value);
    } catch (err) {
      console.error('Failed to load tracks:', err);
      els.list.innerHTML = '<li class="empty">Unable to load tracks.</li>';
    }
  }

  function filter(query) {
    const q = query.toLowerCase().trim();
    filtered = q
      ? tracks.filter(t =>
          (t.title || '').toLowerCase().includes(q) ||
          (t.artist || '').toLowerCase().includes(q) ||
          (t.album || '').toLowerCase().includes(q))
      : tracks.slice();
    render();
  }

  function render() {
    els.list.innerHTML = '';
    if (filtered.length === 0) {
      els.list.innerHTML = '<li class="empty">No tracks found.</li>';
      return;
    }
    filtered.forEach((track, idx) => {
      const li = document.createElement('li');
      li.className = 'track-item' + (tracks[currentIndex] && tracks[currentIndex].key === track.key ? ' active' : '');
      li.innerHTML = `
        <div class="track-meta" title="${escapeHtml(track.title || track.key)}">
          <div class="track-title">${escapeHtml(track.title || track.key)}</div>
          <div class="track-sub">${escapeHtml(track.artist || 'Unknown artist')} — ${escapeHtml(track.album || 'Unknown album')}</div>
        </div>
        <div class="track-size">${formatSize(track.size)}</div>
        <div class="track-actions">
          <a href="${escapeHtml(track.downloadUrl)}" download="${escapeHtml(track.key)}" title="Download">&#9660;</a>
        </div>
      `;
      li.addEventListener('click', e => {
        if (e.target.closest('a')) return;
        playTrackInFiltered(idx);
      });
      els.list.appendChild(li);
    });
  }

  function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
  }

  function playTrackInFiltered(filteredIdx) {
    const track = filtered[filteredIdx];
    if (!track) return;
    currentIndex = tracks.findIndex(t => t.key === track.key);
    playTrack(track);
  }

  function playTrack(track) {
    stopSound();
    sound = new Howl({
      src: [track.url],
      html5: true,
      volume: parseFloat(els.volume.value),
      onplay: () => {
        isPlaying = true;
        updatePlayButton();
        startProgress();
      },
      onpause: () => {
        isPlaying = false;
        updatePlayButton();
        stopProgress();
      },
      onend: () => {
        isPlaying = false;
        updatePlayButton();
        stopProgress();
        if (repeat) {
          playTrack(track);
        } else {
          nextTrack();
        }
      },
      onloaderror: (id, err) => {
        console.error('Howl load error', err);
      },
      onplayerror: (id, err) => {
        console.error('Howl play error', err);
        sound.once('unlock', () => sound.play());
      },
    });
    sound.play();
    els.trackInfo.textContent = `${track.artist || 'Unknown artist'} — ${track.title || track.key}`;
    render();
  }

  function stopSound() {
    stopProgress();
    if (sound) {
      sound.stop();
      sound.unload();
      sound = null;
    }
  }

  function togglePlay() {
    if (!sound) {
      if (filtered.length > 0) {
        playTrackInFiltered(0);
      }
      return;
    }
    if (isPlaying) {
      sound.pause();
    } else {
      sound.play();
    }
  }

  function updatePlayButton() {
    els.btnPlay.innerHTML = isPlaying ? '&#9208;' : '&#9654;';
  }

  function nextTrack() {
    if (filtered.length === 0) return;
    let next;
    if (shuffle) {
      next = Math.floor(Math.random() * filtered.length);
    } else {
      const currentFiltered = filtered.findIndex(t => t.key === (tracks[currentIndex] || {}).key);
      next = (currentFiltered + 1) % filtered.length;
    }
    playTrackInFiltered(next);
  }

  function prevTrack() {
    if (filtered.length === 0) return;
    const currentFiltered = filtered.findIndex(t => t.key === (tracks[currentIndex] || {}).key);
    const prev = (currentFiltered - 1 + filtered.length) % filtered.length;
    playTrackInFiltered(prev);
  }

  function startProgress() {
    stopProgress();
    updateTimer = setInterval(() => {
      if (!sound || !isPlaying) return;
      const seek = sound.seek() || 0;
      const dur = sound.duration() || 0;
      els.progressBar.style.width = dur ? `${(seek / dur) * 100}%` : '0%';
      els.currentTime.textContent = formatTime(seek);
      els.duration.textContent = formatTime(dur);
    }, 250);
  }

  function stopProgress() {
    if (updateTimer) {
      clearInterval(updateTimer);
      updateTimer = null;
    }
  }

  function seekTo(clientX) {
    if (!sound) return;
    const rect = els.progressContainer.getBoundingClientRect();
    const frac = Math.max(0, Math.min(1, (clientX - rect.left) / rect.width));
    const dur = sound.duration();
    if (!dur) return;
    sound.seek(dur * frac);
  }

  els.btnPlay.addEventListener('click', togglePlay);
  els.btnNext.addEventListener('click', nextTrack);
  els.btnPrev.addEventListener('click', prevTrack);
  els.btnShuffle.addEventListener('click', () => {
    shuffle = !shuffle;
    els.btnShuffle.classList.toggle('active', shuffle);
  });
  els.btnRepeat.addEventListener('click', () => {
    repeat = !repeat;
    els.btnRepeat.classList.toggle('active', repeat);
  });
  els.volume.addEventListener('input', () => {
    if (sound) sound.volume(parseFloat(els.volume.value));
  });
  els.search.addEventListener('input', e => filter(e.target.value));

  let dragging = false;
  els.progressContainer.addEventListener('mousedown', e => {
    dragging = true;
    seekTo(e.clientX);
  });
  window.addEventListener('mousemove', e => {
    if (dragging) seekTo(e.clientX);
  });
  window.addEventListener('mouseup', () => {
    dragging = false;
  });
  els.progressContainer.addEventListener('click', e => {
    seekTo(e.clientX);
  });

  loadTracks();
})();
