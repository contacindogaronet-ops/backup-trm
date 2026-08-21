require("dotenv").config();
const { TelegramClient, Api } = require("telegram");
const { StringSession } = require("telegram/sessions");
const { NewMessage } = require("telegram/events");
const fs = require("fs");
const path = require("path");

const apiId = parseInt(process.env.API_ID);
const apiHash = process.env.API_HASH;
const sessionString = process.env.STRING_SESSION || process.env.SESSION_STRING || "";
const stringSession = new StringSession(sessionString);

// --- SMART MULTI-DC WORKER & PROGRESS MANAGER ---
class SmartDownloadManager {
    constructor(client, targetDir, tempDir) {
        this.client = client;
        this.targetDir = targetDir;
        this.tempDir = tempDir;
        this.MAX_WORKERS = 16;
    }

    // Kalkulasi instan (< 0.001ms) untuk alokasi worker & chunk size
    calculateLoad(fileSize) {
        const CHUNK_LIMIT = 512 * 1024; // 512KB per bagian standar Telegram upload.GetFile
        const totalParts = Math.ceil(fileSize / CHUNK_LIMIT);
        
        const workerCount = Math.min(this.MAX_WORKERS, Math.max(1, totalParts));
        const partsPerWorker = Math.ceil(totalParts / workerCount);

        return { workerCount, partsPerWorker, CHUNK_LIMIT, totalParts };
    }

    // Dedicated Worker dengan Auto-DC Migration & Live Progress Callback
    async executeWorker(workerId, fileLocation, startPart, endPart, chunkSize, partsBuffer, onChunkDownloaded) {
        let currentSender = null;

        for (let i = startPart; i < endPart; i++) {
            const offset = i * chunkSize;
            let success = false;
            let retries = 3;

            while (!success && retries > 0) {
                try {
                    let result;
                    if (currentSender) {
                        result = await this.client.invokeWithSender(
                            new Api.upload.GetFile({
                                location: fileLocation,
                                offset: offset,
                                limit: chunkSize,
                            }),
                            currentSender
                        );
                    } else {
                        result = await this.client.invoke(
                            new Api.upload.GetFile({
                                location: fileLocation,
                                offset: offset,
                                limit: chunkSize,
                            })
                        );
                    }
                    partsBuffer[i] = result.bytes;
                    success = true;

                    // Laporkan byte yang berhasil ditarik ke progress manager
                    onChunkDownloaded(result.bytes.length);

                } catch (err) {
                    if (err.newDc || err.constructor.name === 'FileMigrateError') {
                        const targetDc = err.newDc;
                        currentSender = await this.client.getSender(targetDc);
                    } else {
                        retries--;
                        if (retries === 0) throw err;
                    }
                }
            }
        }
    }

    // Main Dispatcher dengan Real-time Backend CLI Progress
    async handleDownload(msg, targetMsg) {
        try {
            await msg.edit({ text: "📥 *Manager: Kalkulasi beban & multi-DC routing...*" });

            const doc = targetMsg.document;
            if (!doc) throw new Error("Format media bukan dokumen/video valid.");

            const fileSize = Number(doc.size);
            const startTime = process.hrtime.bigint();

            const { workerCount, partsPerWorker, CHUNK_LIMIT, totalParts } = this.calculateLoad(fileSize);

            const endTime = process.hrtime.bigint();
            const calcTimeMs = Number(endTime - startTime) / 1_000_000;
            console.log(`\n⚡ Kalkulasi Manager selesai dalam ${calcTimeMs.toFixed(4)}ms | File: ${(fileSize / 1024 / 1024).toFixed(2)} MB | Worker aktif: ${workerCount}`);

            const fileLocation = new Api.InputDocumentFileLocation({
                id: doc.id,
                accessHash: doc.accessHash,
                fileReference: doc.fileReference,
                thumbSize: "",
            });

            const partsBuffer = new Array(totalParts);
            const workerPromises = [];

            // Variabel Tracker untuk Backend CLI Progress (MB/s & Persentase)
            let downloadedBytes = 0;
            let lastTime = Date.now();
            let lastLoaded = 0;

            const onChunkDownloaded = (chunkSize) => {
                downloadedBytes += chunkSize;
                const now = Date.now();
                const duration = (now - lastTime) / 1000;

                // Update CLI setiap 0.3 detik atau saat file selesai
                if (duration >= 0.3 || downloadedBytes >= fileSize) {
                    const speed = (downloadedBytes - lastLoaded) / duration; // bytes per second
                    const speedMBps = (speed / (1024 * 1024)).toFixed(2);
                    const percent = Math.min(100, Math.floor((downloadedBytes / fileSize) * 100));
                    const currentMB = (downloadedBytes / (1024 * 1024)).toFixed(2);
                    const totalMB = (fileSize / (1024 * 1024)).toFixed(2);

                    process.stdout.write(`\r⚙️ Progress: ${percent}% | ${currentMB}MB / ${totalMB}MB | 🚀 ${speedMBps} MB/s   `);

                    lastTime = now;
                    lastLoaded = downloadedBytes;
                }
            };

            // Spawn & Dispatch Workers secara paralel
            for (let w = 0; w < workerCount; w++) {
                const startPart = w * partsPerWorker;
                const endPart = Math.min(startPart + partsPerWorker, totalParts);
                if (startPart >= endPart) break;

                workerPromises.push(
                    this.executeWorker(w + 1, fileLocation, startPart, endPart, CHUNK_LIMIT, partsBuffer, onChunkDownloaded)
                );
            }

            await msg.edit({ text: `⚙️ *16 Worker paralel mendownload lintas DC...*` });

            // Tunggu seluruh worker selesai
            await Promise.all(workerPromises);

            console.log("\n✨ Semua worker selesai, merakit data buffer...");
            await msg.edit({ text: "🔄 *Merakit file & memindahkan ke penyimpanan...*" });

            const finalBuffer = Buffer.concat(partsBuffer);
            
            let ext = "mp4";
            const mime = doc.mimeType || "";
            if (mime.includes("audio")) ext = "mp3";
            else if (mime.includes("pdf")) ext = "pdf";
            else if (mime.includes("zip") || mime.includes("rar")) ext = "zip";

            const fileName = `download_${Date.now()}.${ext}`;
            const tempFilePath = path.join(this.tempDir, fileName);
            const finalPath = path.join(this.targetDir, fileName);

            fs.writeFileSync(tempFilePath, finalBuffer);
            fs.copyFileSync(tempFilePath, finalPath);
            fs.unlinkSync(tempFilePath);

            await msg.edit({ text: `✅ *Download Kilat Sukses!*\n📁 \`${fileName}\`\n📍 \`/sdcard/MEDIAT\`` });
            console.log(`[SUKSES] File tersimpan permanen di: ${finalPath}`);

        } catch (err) {
            console.error("\n❌ Gagal Manager:", err);
            await msg.edit({ text: `❌ Gagal: ${err.message}` });
        }
    }
}

(async () => {
    console.log("⚡ Menghubungkan userbot GramJS ke Telegram...");
    
    const client = new TelegramClient(stringSession, apiId, apiHash, {
        connectionRetries: 5,
    });

    await client.connect();

    if (!await client.isUserAuthorized()) {
        console.error("❌ Error: String session tidak valid atau sudah expired!");
        process.exit(1);
    }

    const tempDir = path.join(__dirname, "downloads");
    const targetDir = "/sdcard/MEDIAT";

    if (!fs.existsSync(tempDir)) fs.mkdirSync(tempDir, { recursive: true });
    if (!fs.existsSync(targetDir)) fs.mkdirSync(targetDir, { recursive: true });

    const manager = new SmartDownloadManager(client, targetDir, tempDir);

    console.log("🚀 High-Performance Multi-DC Core Aktif | Siap Menerima Perintah .dr");

    client.addEventHandler(async (event) => {
        const msg = event.message;

        if (msg.text && msg.text.trim() === ".dr" && msg.isReply) {
            const targetMsg = await msg.getReplyMessage();

            if (!targetMsg || !targetMsg.media) {
                await msg.edit({ text: "❌ Pesan yang di-reply tidak mengandung media!" });
                return;
            }

            manager.handleDownload(msg, targetMsg);
        }
    }, new NewMessage({ outgoing: true }));
})();
