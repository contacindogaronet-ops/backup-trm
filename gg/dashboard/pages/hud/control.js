window.renderRulesBoard = function() {
    const listContainer = document.getElementById('rulesBoardList');
    if(!listContainer) return;
    
    let localMap = window.getLocalRules();
    if(Object.keys(localMap).length === 0) { listContainer.innerHTML = '<div class="rules-empty">Belum ada aturan kustom terpasang, Puh.</div>'; return; }
    
    let html = '';
    for (const [domain, verdict] of Object.entries(localMap)) {
        let textClass = (verdict === 'DROP') ? 'v-DROP' : 'v-ALLOW_VVIP';
        let badgeEmoji = (verdict === 'DROP') ? '💀' : '👑';
        html += `<div class="rule-chip ${textClass}">${badgeEmoji} ${domain} <button class="btn-delete-rule" onclick="removeRuleFromBoard('${domain}')" title="Hapus Aturan">×</button></div>`;
    }
    listContainer.innerHTML = html;
}
renderRulesBoard();

window.removeRuleFromBoard = async function(domain) {
    try {
        await fetch('/api/action', { method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({target: domain, action: "REGULAR"}) });
        window.removeLocalRule(domain); renderRulesBoard(); alert(`♻️ Aturan untuk ${domain} dicabut!`);
    } catch(e) {}
}

// Setel tombol Pooling aktif berdasarkan localStorage!
let curSpeed = window.getPoolSpeed();
if (curSpeed === 1000) document.getElementById('btn1s').classList.add('active-refresh');
else if (curSpeed === 3000) document.getElementById('btn3s').classList.add('active-refresh');
else document.getElementById('btnPause').classList.add('active-refresh');

document.querySelectorAll('.btn-refresh').forEach(btn => {
    btn.addEventListener('click', () => { 
        document.querySelectorAll('.btn-refresh').forEach(b => b.classList.remove('active-refresh'));
        btn.classList.add('active-refresh'); 
        window.setPoolSpeed(parseInt(btn.getAttribute('data-time')));
    });
});

document.getElementById('btn-nuke').addEventListener('click', async () => {
    if (confirm("☢️ Format ulang ingatan AI?")) {
        const btnNuke = document.getElementById('btn-nuke');
        btnNuke.innerText = "NUKING..."; await fetch('/api/reset_ai', { method: 'POST' });
        localStorage.removeItem('customRules'); renderRulesBoard();
        setTimeout(() => { alert("✅ AI FORMATED!"); btnNuke.innerText = "FORMAT AI"; }, 500);
    }
});
