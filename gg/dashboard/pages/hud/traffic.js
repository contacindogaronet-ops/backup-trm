let globalLogsArray = []; let lastSeenLogTime = ""; let telemetryTimerId = null;
const searchInput = document.getElementById('matrix-search');
searchInput.addEventListener('input', renderFilteredLogs);

async function runTrafficTelemetry() {
    try {
        const lRes = await fetch('/api/logs'); let rawLogs = await lRes.json();
        globalLogsArray = Array.isArray(rawLogs) ? rawLogs : [];
        
        if(globalLogsArray.length > 0) {
            let latestLog = globalLogsArray[globalLogsArray.length - 1];
            if(lastSeenLogTime !== "" && latestLog.time !== lastSeenLogTime && latestLog.verdict === 'DROP') {
                window.spawnToast((latestLog.target || "").split(' ').pop(), 'DROP');
            }
            lastSeenLogTime = latestLog.time;
        }
        renderFilteredLogs();
    } catch(err) { console.error("Matrix Engine Timeout"); }
    
    let speed = window.getPoolSpeed();
    if (speed > 0) telemetryTimerId = setTimeout(runTrafficTelemetry, speed);
}

function renderFilteredLogs() {
    const kw = searchInput.value.toLowerCase().trim(); let html = '';
    if (!Array.isArray(globalLogsArray)) globalLogsArray = [];
    let filtered = [...globalLogsArray].reverse().filter(l => {
        let t = l.target ? l.target.toLowerCase() : ""; let v = l.verdict ? l.verdict.toLowerCase() : "";
        return t.includes(kw) || v.includes(kw);
    });

    if(filtered.length > 0) {
        for(let i=0; i<filtered.length; i++) {
            html += '<tr><td>' + filtered[i].time + '</td><td style="font-family:monospace;font-size:11px;">' + filtered[i].target + '</td><td class="v-' + filtered[i].verdict + '">' + filtered[i].verdict + '</td>' + 
            '<td class="td-action"><button class="btn-act" onclick="quickAct(\'DROP\', \'' + filtered[i].target + '\')" title="Force Block">💀</button><button class="btn-act" onclick="quickAct(\'ALLOW_VVIP\', \'' + filtered[i].target + '\')" title="Force Bypass">👑</button></td></tr>';
        }
    } else { html = '<tr><td colspan="4" class="waiting">Tidak ada packet...</td></tr>'; }
    
    document.getElementById('logs').innerHTML = html;
    let autoScrollCheckbox = document.getElementById('autoScroll');
    if(autoScrollCheckbox && autoScrollCheckbox.checked) { 
        const w = document.getElementById('tableWrapper'); if(w) w.scrollTop = w.scrollHeight; 
    }
}
setTimeout(runTrafficTelemetry, 200);
