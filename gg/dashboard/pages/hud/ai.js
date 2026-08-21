const canvas = document.getElementById('aiCoreChart'); const ctx = canvas.getContext('2d');
let chartDataPoints = Array(30).fill(0.5); let telemetryTimerId = null;

const grid = document.getElementById('neuralGrid');
for(let i=0; i<50; i++) { let d = document.createElement('div'); d.className = 'n-dot'; grid.appendChild(d); }
const nDots = document.querySelectorAll('.n-dot');

function resizeCanvas() { if(!canvas) return; canvas.width = canvas.parentElement.clientWidth; canvas.height = 140; drawChart(); }
window.addEventListener('resize', resizeCanvas); setTimeout(resizeCanvas, 100);

function drawChart() {
    ctx.clearRect(0, 0, canvas.width, canvas.height); const w = canvas.width, h = canvas.height, p = 5, gH = h - (p * 2);
    ctx.strokeStyle = 'rgba(226, 232, 240, 0.2)'; ctx.lineWidth = 1;
    for (let i = 1; i < 4; i++) { ctx.beginPath(); ctx.moveTo(0, p + (gH * (i / 4))); ctx.lineTo(w, p + (gH * (i / 4))); ctx.stroke(); }
    const step = w / (chartDataPoints.length - 1);
    ctx.beginPath(); for (let i = 0; i < chartDataPoints.length; i++) { let x = i * step; let y = p + (gH * (1 - chartDataPoints[i])); if (i === 0) ctx.moveTo(x, y); else ctx.lineTo(x, y); }
    ctx.lineTo(w, h); ctx.lineTo(0, h); ctx.closePath();
    let grad = ctx.createLinearGradient(0, 0, 0, h); grad.addColorStop(0, 'rgba(37, 99, 235, 0.3)'); grad.addColorStop(1, 'rgba(37, 99, 235, 0)'); ctx.fillStyle = grad; ctx.fill();
    ctx.beginPath(); for (let i = 0; i < chartDataPoints.length; i++) { let x = i * step; let y = p + (gH * (1 - chartDataPoints[i])); if (i === 0) ctx.moveTo(x, y); else ctx.lineTo(x, y); }
    ctx.strokeStyle = '#2563eb'; ctx.lineWidth = 3; ctx.stroke();
    let lastVal = chartDataPoints[chartDataPoints.length - 1];
    ctx.beginPath(); ctx.arc((chartDataPoints.length - 1) * step, p + (gH * (1 - lastVal)), 4, 0, 2 * Math.PI); ctx.fillStyle = '#2563eb'; ctx.fill(); ctx.lineWidth = 2; ctx.strokeStyle = '#ffffff'; ctx.stroke();
    ctx.fillStyle = '#64748b'; ctx.font = '800 11px sans-serif'; ctx.textAlign = 'left'; ctx.fillText('LR: 0.050', 10, 20);
    ctx.fillStyle = (lastVal > 0.7 || lastVal < 0.3) ? '#f59e0b' : '#10b981'; ctx.fillText('STATE: ' + lastVal.toFixed(3), 10, 36);
}

async function runAITelemetry() {
    try {
        const res = await fetch('/api/stats'); const s = await res.json();
        const aiStatus = document.getElementById('ai-status-text'); const aiMode = document.getElementById('ai-mode-text'); const aiPulse = document.getElementById('ai-pulse-ui');
        let nextP = 0.45 + (Math.random() * 0.1);
        if(s.ai_learning) {
            aiStatus.innerText = "LEARNING"; aiStatus.style.color = "#f59e0b"; aiMode.innerText = "BACKPROPAGATION"; aiMode.style.color = "#f59e0b"; aiPulse.className = 'ai-pulse active-learn';
            nextP = 0.2 + (Math.random() * 0.7);
            nDots.forEach(d => Math.random() > 0.6 ? d.classList.add('fire') : d.classList.remove('fire'));
        } else {
            aiStatus.innerText = "STANDBY"; aiStatus.style.color = "#10b981"; aiMode.innerText = "IDLE (LIST)"; aiMode.style.color = "#64748b"; aiPulse.className = 'ai-pulse';
            nDots.forEach(d => d.classList.remove('fire'));
        }
        chartDataPoints.push(nextP); chartDataPoints.shift(); drawChart();
    } catch(err) {}
    
    let speed = window.getPoolSpeed();
    if (speed > 0) telemetryTimerId = setTimeout(runAITelemetry, speed);
}
setTimeout(runAITelemetry, 200);
