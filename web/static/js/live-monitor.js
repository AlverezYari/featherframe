// web/static/js/live-monitor.js
(function () {
    console.log('live-monitor.js loaded');

    const ws = new WebSocket('ws://' + window.location.host + '/ws/camera');
    const img = document.getElementById('live-feed'); // Ensure this matches your HTML ID
    const status = document.getElementById('status');

    ws.onopen = () => {
        console.log('WebSocket Connected');
        status.textContent = 'Connected';
        status.style.color = 'green';
    };

    ws.onmessage = (evt) => {
        // console.log('Received Frame');
        const blob = new Blob([evt.data], { type: 'image/jpeg' });
        img.src = URL.createObjectURL(blob);
    };

    ws.onclose = () => {
        console.log('WebSocket Disconnected');
        status.textContent = 'Disconnected';
        status.style.color = 'orange';
    };

    ws.onerror = (err) => {
        console.error('WebSocket Error:', err);
        status.textContent = 'Error Connecting';
        status.style.color = 'red';
    };

    document.addEventListener('DOMContentLoaded', () => {
        console.log('Live monitor page loaded');
    });
})();
