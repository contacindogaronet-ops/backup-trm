require("dotenv").config();
const { TelegramClient } = require("telegram");
const { StringSession } = require("telegram/sessions");
const { NewMessage } = require("telegram/events");
const fs = require("fs");
const path = require("path");

const apiId = parseInt(process.env.API_ID);
const apiHash = process.env.API_HASH;
const sessionString = process.env.STRING_SESSION || process.env.SESSION_STRING || "";
const stringSession = new StringSession(sessionString);

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

    console.log("🚀 Userbot aktif! Silakan balas pesan media target menggunakan .dr");

    const tempDir = path.join(__dirname, "downloads");
    const targetDir = "/sdcard/MEDIAT";

    if (!fs.existsSync(tempDir)) fs.mkdirSync(tempDir, { recursive: true });
    if (!fs.existsSync(targetDir)) fs.mkdirSync(targetDir, { recursive: true });

    client.addEventHandler(async (event) => {
        const msg = event.message;

        if (msg.text && msg.text.trim() === ".dr" && msg.isReply) {
            try {
                const targetMsg = await msg.getReplyMessage();

                if (!targetMsg || !targetMsg.media) {
                    await msg.edit({ text: "❌ Pesan yang di-reply tidak mengandung media/video!" });
                    return;
                }

                // Status Telegram rapi (tanpa spam persentase)
                await msg.edit({ text: "📥 *Sedang mendownload media...*" });

                let ext = "mp4";
                if (targetMsg.document) {
                    const mime = targetMsg.document.mimeType || "";
                    if (mime.includes("video")) ext = "mp4";
                    else if (mime.includes("audio")) ext = "mp3";
                    else if (mime.includes("pdf")) ext = "pdf";
                    else if (mime.includes("zip") || mime.includes("rar")) ext = "zip";
                } else if (targetMsg.photo) {
                    ext = "jpg";
                }

                const fileName = `download_${Date.now()}.${ext}`;
                const tempFilePath = path.join(tempDir, fileName);

                let lastTime = Date.now();
                let lastLoaded = 0;

                const downloaded = await client.downloadMedia(targetMsg, {
                    outputFile: tempFilePath,
                    progressCallback: async (downloadedBytes, totalBytes) => {
                        const now = Date.now();
                        const duration = (now - lastTime) / 1000;

                        // Kalkulasi kecepatan setiap interval waktu pendek
                        if (duration >= 0.4 || downloadedBytes === totalBytes) {
                            const speed = (downloadedBytes - lastLoaded) / duration; // bytes/s
                            const speedMBps = (speed / (1024 * 1024)).toFixed(2);
                            const percent = Math.floor((downloadedBytes / totalBytes) * 100);
                            const currentMB = (downloadedBytes / (1024 * 1024)).toFixed(2);
                            const totalMB = (totalBytes / (1024 * 1024)).toFixed(2);

                            process.stdout.write(`\r📥 Progress: ${percent}% | ${currentMB}MB / ${totalMB}MB | 🚀 ${speedMBps} MB/s   `);

                            lastTime = now;
                            lastLoaded = downloadedBytes;
                        }
                    }
                });

                console.log("\n✨ Download di backend selesai.");

                await msg.edit({ text: "🔄 *Memindahkan file ke penyimpanan...*" });

                const finalPath = path.join(targetDir, path.basename(downloaded));
                fs.copyFileSync(downloaded, finalPath);
                fs.unlinkSync(downloaded); // Hapus cache temp

                // Status sukses final di Telegram
                await msg.edit({ text: `✅ *Download & Pindah Sukses!*\n📁 \`${path.basename(finalPath)}\`\n📍 \`/sdcard/MEDIAT\`` });
                console.log(`[SUKSES] Tersimpan permanen di: ${finalPath}`);
            } catch (err) {
                console.error("\n❌ Gagal:", err);
                await msg.edit({ text: `❌ Gagal: ${err.message}` });
            }
        }
    }, new NewMessage({ outgoing: true }));
})();
