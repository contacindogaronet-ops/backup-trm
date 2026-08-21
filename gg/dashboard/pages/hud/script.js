let poolIntervalTime = 1000; let telemetryTimerId = null; let lastTotalBytes = 0; let lastTimestamp = Date.now(); let globalLogsArray = [];
let localRulesMap = new Map(); let lastSeenLogTime = "";

if ('serviceWorker' in navigator) { navigator.serviceWorker.register('sw.js').catch(err => {}); }

// 🌑 FIX: MEMORI ANTI-AMNESIA (WAJIB DI ATAS SENDIRI!)
if(localStorage.getItem('theme') === 'dark') { document.body.classList.add('dark-mode'); }

async function bootstrapFeature(name) {
    try {
        const htmlRes = await fetch(`features/${name}.html?v=${Date.now()}`);
        document.getElementById(name === 'dashboard' ? 'modul-dashboard' : name === 'traffic' ? 'modul-traffic' : name === 'ai' ? 'modul-ai' : 'modul-control').innerHTML = await htmlRes.text();
        
        const link = document.createElement('link'); link.rel = 'stylesheet'; link.href = `features/${name}.css?v=${Date.now()}`; document.head.appendChild(link);
        const script = document.createElement('script'); script.src = `features/${name}.js?v=${Date.now()}`; document.body.appendChild(script);
    } catch(e) { console.error(`[ERROR] Feature ${name} gagal dimuat:`, e); }
}

window.addEventListener('DOMContentLoaded', async () => {
    await Promise.all([bootstrapFeature('dashboard'), bootstrapFeature('traffic'), bootstrapFeature('ai'), bootstrapFeature('control')]);
    
    const dockBtns = document.querySelectorAll('.dock-btn');
    dockBtns.forEach(btn => {
        btn.addEventListener('click', () => {
            document.querySelector('.dock-btn.active').classList.remove('active'); document.querySelector('.spa-view.active-view').classList.remove('active-view');
            btn.classList.add('active'); const targetId = btn.getAttribute('data-target'); document.getElementById(targetId).classList.add('active-view');
            if (targetId === 'modul-ai' && typeof resizeCanvas === 'function') resizeCanvas();
            if (targetId === 'modul-dashboard' && typeof resizeHistCanvas === 'function') resizeHistCanvas();
        });
    });
    telemetryTimerId = setTimeout(runTelemetry, 500);
});

function spawnToast(message, verdict) {
    const container = document.getElementById('toastContainer'); if (!container) return;
    const toast = document.createElement('div'); toast.className = 'toast';
    let emoji = (verdict === 'DROP') ? '🛡️ [BLOCKED]' : '👑 [BYPASSED]';
    toast.style.borderLeft = (verdict === 'DROP') ? '4px solid var(--red)' : '4px solid var(--blue)';
    toast.innerText = `${emoji} ${message}`; container.appendChild(toast);
    setTimeout(() => { toast.classList.add('fade-out'); toast.addEventListener('animationend', () => { toast.remove(); }); }, 3000);
}

async function quickAct(action, rawTarget) {
    let domain = rawTarget.split(' ').pop();
    try {
        await fetch('/api/action', { method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({target: domain, action: action}) });
        localRulesMap.set(domain, action); if (typeof renderRulesBoard === 'function') renderRulesBoard(); spawnToast(domain, action);
    } catch(e) {}
}

async function runTelemetry() {
    try {
        const res = await fetch('/api/stats'); const s = await res.json();
        if (typeof updateCoreMetrics === 'function') updateCoreMetrics(s);
        if (typeof updateNeuralState === 'function') updateNeuralState(s);

        const lRes = await fetch('/api/logs'); let rawLogs = await lRes.json();
        globalLogsArray = Array.isArray(rawLogs) ? rawLogs : [];
        if(globalLogsArray.length > 0) {
            let latestLog = globalLogsArray[globalLogsArray.length - 1];
            if(lastSeenLogTime !== "" && latestLog.time !== lastSeenLogTime && latestLog.verdict === 'DROP') { spawnToast((latestLog.target || "").split(' ').pop(), 'DROP'); }
            lastSeenLogTime = latestLog.time;
        }
        if (typeof renderFilteredLogs === 'function') renderFilteredLogs();
    } catch(err) {}
    if (poolIntervalTime > 0) telemetryTimerId = setTimeout(runTelemetry, poolIntervalTime);
}
