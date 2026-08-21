self.addEventListener('install', (e) => {
    self.skipWaiting();
});

self.addEventListener('activate', (e) => {
    console.log('[System] Service Worker NganjukCore Active!');
});

self.addEventListener('fetch', (e) => {
    // Biarkan semua request telemetri lewat tanpa di-cache browser
});
