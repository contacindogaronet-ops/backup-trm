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

// --- WORKER & MANAGER ENGINE ---
class SmartDownloadManager {
    constructor(client, targetDir, tempDir) {
        this.client = client;
        this.targetDir = targetDir;
        this.tempDir = tempDir;
        this.MAX_WORKERS = 16;
    }

    // Kalkulasi instan (< 0.001ms) untuk menentukan alokasi worker & chunk size
    calculateLoad(fileSize) {
        const CHUNK_LIMIT = 512 * 1024; // 512KB per bagian standar Telegram upload.getFile
        const totalParts = Math.ceil(fileSize / CHUNK_LIMIT);
        
        // Tentukan jumlah worker optimal (maksimal 16, atau disesuaikan dengan total bagian file)
        const workerCount = Math.min(this.MAX_WORKERS, Math.max(1, totalParts));
        const partsPerWorker = Math.ceil(totalParts / workerCount);

        return { workerCount, partsPerWorker, CHUNK_LIMIT, totalParts };
    }

    // Dedicated Worker Routine (Siap dipanggil instan oleh manager)
    async executeWorker(workerId, fileLocation, startPart, endPart, chunkSize, partsBuffer) {
        for (let i = startPart; i < endPart; i++) {
            const offset = i * chunkSize;
            try {
                const result = await this.client.invoke(
                    new Api.upload.GetFile({
                        location: fileLocation,
                        offset: offset,
                        limit: chunkSize,
                    })
                );
                partsBuffer[i] = result.bytes;
            } catch (err) {
                console.error(`❌ Worker ${workerId} gagal pada part ${i}:`, err.message);
                throw err;
            }
        }
    }

    // Main Dispatcher
    async handleDownload(msg, targetMsg) {
        try {
            await msg.edit({ text: "📥 *Manager: Kalkulasi beban & alokasi 16 Worker...*" });

            const doc = targetMsg.document;
            if (!doc) throw new Error("Format media bukan dokumen/video valid.");

            const fileSize = Number(doc.size);
            const startTime = process.hrtime.bigint();

            // 1. Kalkulasi kilat alokasi worker
            const { workerCount, partsPerWorker, CHUNK_LIMIT, totalParts } = this.calculateLoad(fileSize);

            const endTime = process.hrtime.bigint();
            const calcTimeMs = Number(endTime - startTime) / 1_000_000;
            console.log(`⚡ Kalkulasi Manager selesai dalam ${calcTimeMs.toFixed(4)}ms | File: ${(fileSize / 1024 / 1024).toFixed(2)} MB | Worker aktif: ${workerCount}`);

            // Ekstraksi location objek untuk MTProto low-level
            const fileLocation = new Api.InputDocumentFileLocation({
                id: doc.id,
                accessHash: doc.accessHash,
                fileReference: doc.fileReference,
                thumbSize: "",
            });

            const partsBuffer = new Array(totalParts);
            const workerPromises = [];

            // 2. Spawn & Dispatch Workers secara paralel instan
            for (let w = 0; w < workerCount; w++) {
                const startPart = w * partsPerWorker;
                const endPart = Math.min(startPart + partsPerWorker, totalParts);
                if (startPart >= endPart) break;

                workerPromises.push(
                    this.executeWorker(w + 1, fileLocation, startPart, endPart, CHUNK_LIMIT, partsBuffer)
                );
            }

            await msg.edit({ text: `⚙️ *16 Worker bekerja secara paralel mendownload...*` });

            // Tunggu seluruh worker menyelesaikan tugas chunking-nya
            await Promise.all(workerPromises);

            console.log("✨ Semua worker selesai, menggabungkan data buffer...");
            await msg.edit({ text: "🔄 *Merakit file & memindahkan ke penyimpanan...*" });

            // Gabungkan semua buffer bagian file
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

    console.log("🚀 High-Performance Core Aktif | Siap Menerima Perintah .dr");

    client.addEventHandler(async (event) => {
        const msg = event.message;

        if (msg.text && msg.text.trim() === ".dr" && msg.isReply) {
            const targetMsg = await msg.getReplyMessage();

            if (!targetMsg || !targetMsg.media) {
                await msg.edit({ text: "❌ Pesan yang di-reply tidak mengandung media!" });
                return;
            }

            // Eksekusi instan via Smart Manager
            manager.handleDownload(msg, targetMsg);
        }
    }, new NewMessage({ outgoing: true }));
})();
