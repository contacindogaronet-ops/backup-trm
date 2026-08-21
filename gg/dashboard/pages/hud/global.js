// 1. PWA & DARK MODE INIT
if ('serviceWorker' in navigator) { navigator.serviceWorker.register('sw.js').catch(err => {}); }
if(localStorage.getItem('theme') === 'dark') document.body.classList.add('dark-mode');

document.addEventListener('DOMContentLoaded', () => {
    const themeBtn = document.getElementById('theme-toggle');
    if(themeBtn) {
        themeBtn.addEventListener('click', () => {
            document.body.classList.toggle('dark-mode');
            localStorage.setItem('theme', document.body.classList.contains('dark-mode') ? 'dark' : 'light');
        });
    }
});

// 2. KENDALI POOLING LINTAS HALAMAN (Biar settingan Control gak hilang)
window.getPoolSpeed = () => parseInt(localStorage.getItem('poolSpeed')) === 0 ? 0 : (parseInt(localStorage.getItem('poolSpeed')) || 1000);
window.setPoolSpeed = (ms) => localStorage.setItem('poolSpeed', ms);

// 3. STORAGE ATURAN FIREWALL (Biar Rules Board Anti Amnesia)
window.getLocalRules = () => JSON.parse(localStorage.getItem('customRules') || '{}');
window.addLocalRule = (domain, verdict) => { let r = getLocalRules(); r[domain] = verdict; localStorage.setItem('customRules', JSON.stringify(r)); };
window.removeLocalRule = (domain) => { let r = getLocalRules(); delete r[domain]; localStorage.setItem('customRules', JSON.stringify(r)); };

// 4. FLOATING TOAST SYSTEM
window.spawnToast = function(message, verdict) {
    const container = document.getElementById('toastContainer'); if (!container) return;
    const toast = document.createElement('div'); toast.className = 'toast';
    let emoji = (verdict === 'DROP') ? '🛡️ [BLOCKED]' : '👑 [BYPASSED]';
    toast.style.borderLeft = (verdict === 'DROP') ? '4px solid var(--red)' : '4px solid var(--blue)';
    toast.innerText = `${emoji} ${message}`; container.appendChild(toast);
    setTimeout(() => { toast.classList.add('fade-out'); toast.addEventListener('animationend', () => { toast.remove(); }); }, 3000);
}

// 5. EKSEKUTOR JALUR PINTAS
window.quickAct = async function(action, rawTarget) {
    let domain = rawTarget.split(' ').pop();
    try {
        await fetch('/api/action', { method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({target: domain, action: action}) });
        addLocalRule(domain, action);
        spawnToast(domain, action);
        if(typeof window.renderRulesBoard === 'function') window.renderRulesBoard();
    } catch(e) { console.error(e); }
}
