// web/static/js/live-monitor.js
(function () {
  console.log('live-monitor.js loaded');

  // 1) DOM references
  const imgLiveFeed = document.getElementById('live-feed');
  const statusEl = document.getElementById('status');
  const screenshotBtn = document.getElementById('screenshotBtn');
  const screenshotPreview = document.getElementById('screenshotPreview');

  // 2) Set up the WebSocket for live feed
  const wsUrl = 'ws://' + window.location.host + '/ws/liveMonitor';
  const ws = new WebSocket(wsUrl);

  ws.onopen = () => {
    console.log('LiveMonitor WS Connected');
    statusEl.textContent = 'Connected';
    statusEl.style.color = 'green';
  };

  ws.onmessage = (evt) => {
    // Received frame from server
    const blob = new Blob([evt.data], { type: 'image/jpeg' });
    imgLiveFeed.src = URL.createObjectURL(blob);
  };

  ws.onclose = () => {
    console.log('LiveMonitor WS Disconnected');
    statusEl.textContent = 'Disconnected';
    statusEl.style.color = 'orange';
  };

  ws.onerror = (err) => {
    console.error('WebSocket Error:', err);
    statusEl.textContent = 'Error Connecting';
    statusEl.style.color = 'red';
  };

  // 3) Screenshot button handler
  screenshotBtn.addEventListener('click', async () => {
    try {
      const response = await fetch('/api/screenshot');
      if (!response.ok) {
        alert('Error fetching screenshot');
        return;
      }
      const blob = await response.blob();
      const url = URL.createObjectURL(blob);
      screenshotPreview.src = url;
    } catch (err) {
      console.error(err);
      alert('Screenshot failed');
    }
  });

  // 4) Optional: DOMContentLoaded event
  document.addEventListener('DOMContentLoaded', () => {
    console.log('Live monitor page loaded and script ready');
  });
})();

