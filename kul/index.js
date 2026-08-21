const { TelegramClient, Api } = require('telegram');
const { StringSession } = require('telegram/sessions');
const input = require('input');
const fs = require('fs');
const path = require('path');
const config = require('./config');

if (!fs.existsSync(config.downloadDir)) fs.mkdirSync(config.downloadDir, { recursive: true });

const client = new TelegramClient(
    new StringSession(config.sessionStr),
    config.apiId,
    config.apiHash,
    { connectionRetries: 3, requestRetries: 3, timeout: 10000 }
);

client.addEventHandler(async (update) => {
    if (update.className === 'UpdateNewMessage' && update.message) {
        const msg = update.message;

        if (msg.message === '.dl') {
            (async () => {
                const taskId = Math.floor(Math.random() * 1000);
                
                let replyId = msg.replyTo ? (msg.replyTo.replyToMsgId || msg.replyTo.reply_to_msg_id) : null;
                if (!replyId) return;

                try {
                    await client.editMessage(msg.peerId, { message: msg.id, text: "⚡ **Target dikunci. Mengerahkan 4 Worker...**" }).catch(()=>{});
                    
                    const fetched = await client.getMessages(msg.peerId, { ids: [replyId] });
                    const replyMsg = fetched[0];
                    if (!replyMsg || !replyMsg.media) return;

                    const doc = replyMsg.document || (replyMsg.media && replyMsg.media.document);
                    if (!doc) return;

                    const size = doc.size;
                    const chunkSize = 1024 * 1024; 
                    const totalChunks = Math.ceil(size / chunkSize);
                    
                    const targetDcId = doc.dcId || client.session.dcId;
                    const filePath = path.join(config.downloadDir, `FAST_${replyMsg.id}_${Date.now()}.mp4`);
                    
                    console.log(`\n🚀 [TASK ${taskId}] Eksekusi Langsung -> Target DC: ${targetDcId} | Size: ${(size/1024/1024).toFixed(2)} MB`);

                    const fd = fs.openSync(filePath, 'w'); 
                    const inputFileLocation = new Api.InputDocumentFileLocation({
                        id: doc.id,
                        accessHash: doc.accessHash,
                        fileReference: doc.fileReference,
                        thumbSize: ""
                    });

                    let downloaded = 0;
                    let lastUpdate = 0;
                    const chunkQueue = Array.from({length: totalChunks}, (_, i) => i);

                                        const workerTask = async (workerId) => {
                        while (chunkQueue.length > 0) {
                            const currentIndex = chunkQueue.shift(); 
                            if (currentIndex === undefined) break;

                            const offset = currentIndex * chunkSize;
                            
                            try {
                                // Eksekusi murni tanpa parameter dcId karena jalur socket DC 1-5 udah open di background
                                const res = await client.invoke(new Api.upload.GetFile({
                                    location: inputFileLocation,
                                    offset: offset,
                                    limit: chunkSize,
                                    precise: true
                                }));
                                
                                fs.writeSync(fd, res.bytes, 0, res.bytes.length, offset);
                                downloaded += res.bytes.length;

                                const now = Date.now();
                                if (now - lastUpdate > 1000) {
                                    process.stdout.write(`\r⚙️ [WORKER ${workerId}] Sedot: ${(downloaded/1024/1024).toFixed(2)} MB / ${(size/1024/1024).toFixed(2)} MB`);
                                    lastUpdate = now;
                                }
                            } catch (e) {
                                chunkQueue.push(currentIndex);
                                await new Promise(r => setTimeout(r, 1000));
                            }
                        }
                    };


                    const startTime = Date.now();
                    const workers = [];
                    
                    for (let i = 1; i <= 4; i++) {
                        workers.push(workerTask(i));
                        await new Promise(r => setTimeout(r, 100)); 
                    }
                    
                    const uiUpdater = setInterval(async () => {
                        if (downloaded >= size) clearInterval(uiUpdater);
                        const percent = ((downloaded / size) * 100).toFixed(1);
                        await client.editMessage(msg.peerId, { 
                            message: msg.id, 
                            text: `⚡ **4-WORKER KILAT [${percent}%]**\n\n📊 \`${(downloaded/1024/1024).toFixed(2)} MB / ${(size/1024/1024).toFixed(2)} MB\`` 
                        }).catch(()=>{});
                    }, 4000); 

                    await Promise.all(workers);
                    clearInterval(uiUpdater);
                    fs.closeSync(fd);

                    const elapsed = ((Date.now() - startTime) / 1000).toFixed(1);
                    console.log(`\n✅ [TASK ${taskId}] SELESAI -> ${filePath} (${elapsed}s)`);

                    await client.editMessage(msg.peerId, { 
                        message: msg.id, 
                        text: `🔥 **DOWNLOAD SELESAI!** 🔥\n\n📁 \`${filePath}\`\n⏱ **Waktu:** ${elapsed} detik` 
                    }).catch(()=>{});

                } catch (err) {
                    console.log(`\n❌ [TASK ${taskId} ERROR]: ${err.message}`);
                    await client.editMessage(msg.peerId, { message: msg.id, text: `❌ **Error:** \`${err.message}\`` }).catch(()=>{});
                }
            })();
        }
    }
});

(async () => {
    client.setLogLevel('none');
    
    await client.start({
        phoneNumber: async () => await input.text("Nomor: "),
        password: async () => await input.text("2FA: "),
        phoneCode: async () => await input.text("OTP: "),
        onError: (err) => console.log(err),
    });

    console.log("\n[+] Mengaktifkan Fast-Connect Background Router (DC 1 - 5)...");
    
    // HANDSHAKE NON-BLOCKING: Background connection biar DC 1 langsung nancep tanpa bikin terminal freeze
    for (let i = 1; i <= 5; i++) {
        client.getSender(i).then(() => {
            console.log(`[+] Jalur DC ${i} Terhubung Sempurna!`);
        }).catch((e) => {
            // Silent background retry
        });
    }
    
    console.log("\n[+] MESIN SIAP. Silakan lanjut istirahat, nanti kita tuning lagi pas token fresh!");
})();

