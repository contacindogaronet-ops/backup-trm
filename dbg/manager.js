const { Api } = require("telegram");
const fs = require("fs");
const path = require("path");

class SmartDownloadManager {
    constructor(client, targetDir) {
        this.client = client;
        this.targetDir = targetDir;
        this.MAX_WORKERS = 16;
        this.CHUNK_LIMIT = 512 * 1024; // 512KB

        // PID Controller Constants
        this.Kp = 0.03;
        this.Ki = 0.005;
        this.Kd = 0.01;
        this.targetRtt = 150; // Target ideal 150ms
        this.integral = 0;
        this.lastError = 0;

        this.currentRtt = 150;
        this.activeWorkers = 4; // Start safe
    }

    // PID Dynamic Concurrency Controller
    updateConcurrency() {
        const error = this.targetRtt - this.currentRtt;
        this.integral += error;
        
        // Anti-windup clamping
        this.integral = Math.max(-50, Math.min(50, this.integral));
        
        const derivative = error - this.lastError;
        const output = (this.Kp * error) + (this.Ki * this.integral) + (this.Kd * derivative);
        this.lastError = error;

        // Apply shift
        const nextWorkerCount = Math.round(this.activeWorkers + output);
        this.activeWorkers = Math.max(1, Math.min(this.MAX_WORKERS, nextWorkerCount));
        
        return this.activeWorkers;
    }

    calculateLoad(fileSize, workerCount) {
        const totalParts = ((fileSize + this.CHUNK_LIMIT - 1) / this.CHUNK_LIMIT) | 0;
        const partsPerWorker = ((totalParts + workerCount - 1) / workerCount) | 0;
        return { totalParts, partsPerWorker };
    }

    async executeWorker(workerId, fileLocation, startPart, endPart, masterBuffer, onChunkDownloaded) {
        let currentSender = null;
        let attempt = 0;

        for (let i = startPart; i < endPart; i++) {
            const offset = i * this.CHUNK_LIMIT;
            let success = false;

            while (!success && attempt < 5) {
                try {
                    const startTime = Date.now();
                    const payload = new Api.upload.GetFile({
                        location: fileLocation,
                        offset: offset,
                        limit: this.CHUNK_LIMIT,
                    });

                    const result = currentSender 
                        ? await this.client.invokeWithSender(payload, currentSender)
                        : await this.client.invoke(payload);
                    
                    // Latency update
                    const rtt = Date.now() - startTime;
                    this.currentRtt = (this.currentRtt * 0.9) + (rtt * 0.1); 

                    result.bytes.copy(masterBuffer, offset);
                    
                    success = true;
                    attempt = 0;
                    onChunkDownloaded(result.bytes.length);
                } catch (err) {
                    if (err.newDc || err.constructor.name === 'FileMigrateError') {
                        currentSender = await this.client.getSender(err.newDc);
                    } else {
                        attempt++;
                        await new Promise(r => setTimeout(r, Math.pow(2, attempt) * 100));
                        if (attempt === 5) throw err;
                    }
                }
            }
        }
    }

    async handleDownload(msg, targetMsg) {
        try {
            const doc = targetMsg.document;
            if (!doc) throw new Error("Format media tidak valid.");

            await msg.edit({ text: "📥 *Manager: PID Controller Active...*" });

            // Apply PID Controller to determine optimal concurrency
            const workerCount = this.updateConcurrency();
            const fileSize = Number(doc.size);
            const { totalParts, partsPerWorker } = this.calculateLoad(fileSize, workerCount);

            console.log(`\n⚡ PID Control | Workers: ${workerCount} | RTT: ${this.currentRtt.toFixed(2)}ms | Size: ${(fileSize / (1024 * 1024)).toFixed(2)} MB`);

            const fileLocation = new Api.InputDocumentFileLocation({
                id: doc.id,
                accessHash: doc.accessHash,
                fileReference: doc.fileReference,
                thumbSize: "",
            });

            const masterBuffer = Buffer.allocUnsafe(fileSize);
            const workerPromises = [];
            
            let downloadedBytes = 0;
            let lastTime = Date.now();
            let lastLoaded = 0;

            const onChunkDownloaded = (chunkSize) => {
                downloadedBytes += chunkSize;
                const now = Date.now();
                const duration = (now - lastTime) / 1000;

                if (duration >= 0.5 || downloadedBytes >= fileSize) {
                    const speed = (downloadedBytes - lastLoaded) / duration;
                    const speedMBps = (speed / (1024 * 1024)).toFixed(2);
                    const percent = Math.min(100, Math.floor((downloadedBytes / fileSize) * 100));
                    
                    process.stdout.write(`\r⚙️ Active: ${workerCount} | ${percent}% | 🚀 ${speedMBps} MB/s   `);

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

            await msg.edit({ text: `⚙️ *Processing with ${workerCount} Adaptive Workers...*` });
            await Promise.all(workerPromises);

            let ext = "mp4";
            const mime = doc.mimeType || "";
            if (mime.includes("audio")) ext = "mp3";
            else if (mime.includes("pdf")) ext = "pdf";
            else if (mime.includes("zip") || mime.includes("rar")) ext = "zip";

            const fileName = `download_${Date.now()}.${ext}`;
            const finalPath = path.join(this.targetDir, fileName);

            fs.writeFileSync(finalPath, masterBuffer);

            await msg.edit({ text: `✅ *Download Sukses!*\n📁 \`${fileName}\`\n📍 \`/sdcard/MEDIAT\`` });
            console.log(`\n[SUKSES] File tersimpan: ${finalPath}`);

        } catch (err) {
            console.error("\n❌ Manager Critical Failure:", err);
            await msg.edit({ text: `❌ Fatal Error: ${err.message}` });
        }
    }
}

module.exports = SmartDownloadManager;
