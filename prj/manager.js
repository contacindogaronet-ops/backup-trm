const { Api } = require("telegram");
const fs = require("fs");
const path = require("path");

class SmartDownloadManager {
    constructor(client, targetDir) {
        this.client = client;
        this.targetDir = targetDir;
        this.MAX_WORKERS = 16;
        this.CHUNK_LIMIT = 512 * 1024; // 512KB MTProto Standard
    }

    // Kalkulasi Bitwise murni (Ultra-low latency < 0.005ms)
    calculateLoad(fileSize) {
        const totalParts = ((fileSize + this.CHUNK_LIMIT - 1) / this.CHUNK_LIMIT) | 0;
        const workerCount = totalParts > this.MAX_WORKERS ? this.MAX_WORKERS : (totalParts > 0 ? totalParts : 1);
        const partsPerWorker = ((totalParts + workerCount - 1) / workerCount) | 0;

        return { workerCount, partsPerWorker, totalParts };
    }

    async executeWorker(workerId, fileLocation, startPart, endPart, masterBuffer, onChunkDownloaded) {
        let currentSender = null;

        for (let i = startPart; i < endPart; i++) {
            const offset = i * this.CHUNK_LIMIT;
            let success = false;
            let retries = 3;

            while (!success && retries > 0) {
                try {
                    const payload = new Api.upload.GetFile({
                        location: fileLocation,
                        offset: offset,
                        limit: this.CHUNK_LIMIT,
                    });

                    const result = currentSender 
                        ? await this.client.invokeWithSender(payload, currentSender)
                        : await this.client.invoke(payload);

                    // Zero-Copy: Langsung suntik payload byte ke memori utama
                    result.bytes.copy(masterBuffer, offset);
                    
                    success = true;
                    onChunkDownloaded(result.bytes.length);
                } catch (err) {
                    if (err.newDc || err.constructor.name === 'FileMigrateError') {
                        currentSender = await this.client.getSender(err.newDc);
                    } else {
                        retries--;
                        if (retries === 0) throw err;
                    }
                }
            }
        }
    }

    async handleDownload(msg, targetMsg) {
        try {
            await msg.edit({ text: "📥 *Manager: Zero-Copy Engine & Bitwise Calc Running...*" });

            const doc = targetMsg.document;
            if (!doc) throw new Error("Format media tidak valid.");

            const fileSize = Number(doc.size);
            const startTime = process.hrtime.bigint();

            const { workerCount, partsPerWorker, totalParts } = this.calculateLoad(fileSize);

            const endTime = process.hrtime.bigint();
            const calcTimeMs = Number(endTime - startTime) / 1_000_000;
            console.log(`\n⚡ Kalkulasi Bitwise: ${calcTimeMs.toFixed(5)}ms | Size: ${(fileSize / (1024 * 1024)).toFixed(2)} MB | Workers: ${workerCount}`);

            const fileLocation = new Api.InputDocumentFileLocation({
                id: doc.id,
                accessHash: doc.accessHash,
                fileReference: doc.fileReference,
                thumbSize: "",
            });

            // Alokasi RAM Instan tanpa pembersihan default V8 (Anti Garbage Collector Overhead)
            const masterBuffer = Buffer.allocUnsafe(fileSize);
            const workerPromises = [];
            
            let downloadedBytes = 0;
            let lastTime = Date.now();
            let lastLoaded = 0;

            const onChunkDownloaded = (chunkSize) => {
                downloadedBytes += chunkSize;
                const now = Date.now();
                const duration = (now - lastTime) / 1000;

                if (duration >= 0.3 || downloadedBytes >= fileSize) {
                    const speed = (downloadedBytes - lastLoaded) / duration;
                    const speedMBps = (speed / (1024 * 1024)).toFixed(2);
                    const percent = Math.min(100, Math.floor((downloadedBytes / fileSize) * 100));
                    const currentMB = (downloadedBytes / (1024 * 1024)).toFixed(2);
                    const totalMB = (fileSize / (1024 * 1024)).toFixed(2);

                    process.stdout.write(`\r⚙️ Progress: ${percent}% | ${currentMB}MB / ${totalMB}MB | 🚀 ${speedMBps} MB/s   `);

                    lastTime = now;
                    lastLoaded = downloadedBytes;
                }
            };

            for (let w = 0; w < workerCount; w++) {
                const startPart = w * partsPerWorker;
                const endPart = startPart + partsPerWorker > totalParts ? totalParts : startPart + partsPerWorker;
                if (startPart >= endPart) break;

                workerPromises.push(
                    this.executeWorker(w + 1, fileLocation, startPart, endPart, masterBuffer, onChunkDownloaded)
                );
            }

            await msg.edit({ text: `⚙️ *Memproses Zero-Copy Download dengan ${workerCount} Worker...*` });
            await Promise.all(workerPromises);

            console.log("\n✨ Data utuh di memori, menulis langsung ke storage...");
            await msg.edit({ text: "🔄 *Direct I/O Write ke /sdcard/MEDIAT...*" });

            let ext = "mp4";
            const mime = doc.mimeType || "";
            if (mime.includes("audio")) ext = "mp3";
            else if (mime.includes("pdf")) ext = "pdf";
            else if (mime.includes("zip") || mime.includes("rar")) ext = "zip";

            const fileName = `download_${Date.now()}.${ext}`;
            const finalPath = path.join(this.targetDir, fileName);

            // Tulis buffer utuh langsung ke direktori final
            fs.writeFileSync(finalPath, masterBuffer);

            await msg.edit({ text: `✅ *Download Zero-Copy Sukses!*\n📁 \`${fileName}\`\n📍 \`/sdcard/MEDIAT\`` });
            console.log(`[SUKSES] File tersimpan permanen di: ${finalPath}`);

        } catch (err) {
            console.error("\n❌ Gagal Manager:", err);
            await msg.edit({ text: `❌ Gagal: ${err.message}` });
        }
    }
}

module.exports = SmartDownloadManager;
