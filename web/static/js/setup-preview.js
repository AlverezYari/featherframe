// web/static/js/setup-preview.js
console.log('setup-preview.js loaded');

const ws = new WebSocket('ws://' + window.location.host + '/ws/camera');
const img = document.getElementById('camera-feed');
const status = document.getElementById('status');

ws.onopen = () => {
    console.log('Websocket Connected');
    status.textContent = 'Connected';
};

ws.onmessage = (evt) => {
    console.log('Recieved Frame');
    const blob = new Blob([evt.data], {type: 'image/jpeg'});
    img.src = URL.createObjectURL(blob);
};

ws.onclose = () => {
console.log('Websocket Disconnected');
    status.textContent = 'Disconnected';
};

ws.onerror = (err) => {
console.error('Websocket Error:', err);
status.textContent = 'Error Connecting';

};
