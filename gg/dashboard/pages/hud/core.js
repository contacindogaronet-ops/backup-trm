const histCanvas = document.getElementById('trafficHistogram'); const hCtx = histCanvas.getContext('2d');
let histDataPoints = Array(40).fill(0);
let lastTotalBytes = 0; let lastTimestamp = Date.now(); let telemetryTimerId = null;

function resizeHistCanvas() { if(!histCanvas) return; histCanvas.width = histCanvas.parentElement.clientWidth; histCanvas.height = 80; drawHistogram(); }
window.addEventListener('resize', resizeHistCanvas); setTimeout(resizeHistCanvas, 100);

function drawHistogram() {
    hCtx.clearRect(0, 0, histCanvas.width, histCanvas.height); const w = histCanvas.width, h = histCanvas.height;
    const maxVal = Math.max(...histDataPoints, 1); const barW = (w / histDataPoints.length) - 2;
    for (let i = 0; i < histDataPoints.length; i++) {
        let barH = (histDataPoints[i] / maxVal) * h; if (barH < 2) barH = 2; let x = i * (w / histDataPoints.length); let y = h - barH;
        let grad = hCtx.createLinearGradient(0, y, 0, h); grad.addColorStop(0, '#3b82f6'); grad.addColorStop(1, '#1d4ed8');
        hCtx.fillStyle = grad; hCtx.beginPath(); hCtx.roundRect(x, y, barW, barH, [4, 4, 0, 0]); hCtx.fill();
    }
}

// 💥 INI DIA MESIN POMPA YANG SEMPAT KEHAPUS BABI!
async function runCoreTelemetry() {
    try {
        const res = await fetch('/api/stats'); const s = await res.json();
        let now = Date.now(); let timePassed = (now - lastTimestamp) / 1000; if (timePassed <= 0) timePassed = 1;
        let byteDelta = s.total_bytes - lastTotalBytes; if (byteDelta < 0) byteDelta = 0;
        
        let currentSpeedMBs = (byteDelta / 1024 / 1024) / timePassed;
        if(document.getElementById('live-speed')) document.getElementById('live-speed').innerHTML = currentSpeedMBs.toFixed(2) + ' <span class="unit text-light">MB/s</span>';
        histDataPoints.push(currentSpeedMBs); histDataPoints.shift(); drawHistogram();

        let totalProcessedMB = s.total_bytes / 1024 / 1024;
        if(document.getElementById('total-data')) document.getElementById('total-data').innerHTML = (totalProcessedMB > 1024) ? (totalProcessedMB / 1024).toFixed(2) + ' <span class="unit">GB</span>' : totalProcessedMB.toFixed(2) + ' <span class="unit">MB</span>';
        lastTotalBytes = s.total_bytes; lastTimestamp = now;

        if(document.getElementById('uptime')) document.getElementById('uptime').innerText = s.uptime; 
        if(document.getElementById('active-conns')) document.getElementById('active-conns').innerText = s.active_conns;
        if(document.getElementById('memory')) document.getElementById('memory').innerText = (s.mem_alloc_bytes < 1048576) ? (s.mem_alloc_bytes / 1024).toFixed(2) + ' KB' : (s.mem_alloc_bytes / 1024 / 1024).toFixed(2) + ' MB';
        if(document.getElementById('val-ads')) document.getElementById('val-ads').innerText = s.ads_blocked; 
        if(document.getElementById('val-vip')) document.getElementById('val-vip').innerText = s.vip_allowed;
        if(document.getElementById('val-tele')) document.getElementById('val-tele').innerText = s.tele_hits; 
        if(document.getElementById('val-cache')) document.getElementById('val-cache').innerText = s.cache_hits;

        // ⏱️ LATENCY MONITOR
        if(document.getElementById('val-latency')) document.getElementById('val-latency').innerText = (s.latency || 0) + " ms";

        // 🏆 TOP TALKERS (DENGAN VAKSIN ANTI-NULL SILENT CRASH!)
        let topHtml = "";
        let topData = Array.isArray(s.top_talkers) ? s.top_talkers : [];
        if(topData.length > 0) {
            topData.forEach(t => {
                let mb = (t.Value / 1024 / 1024).toFixed(1);
                // Bersihin port dari IP/Domain biar cantik
                let cleanName = t.Key.split(':')[0];
                topHtml += `<div style="display:flex; justify-content:space-between; margin-bottom:4px;"><span>${cleanName}</span> <b style="color:var(--text-main);">${mb}MB</b></div>`;
            });
        } else {
            topHtml = `<div style="color:var(--text-muted); font-style:italic;">Belum ada traffic, Puh...</div>`;
        }
        if(document.getElementById('val-top')) document.getElementById('val-top').innerHTML = topHtml;

    } catch(err) { console.error("Core Engine Error:", err); }
    
    // Looping Mesin
    let speed = window.getPoolSpeed();
    if (speed > 0) telemetryTimerId = setTimeout(runCoreTelemetry, speed);
}
setTimeout(runCoreTelemetry, 200); // Starter Engine!
