require("dotenv").config();
const { TelegramClient } = require("telegram");
const { StringSession } = require("telegram/sessions");
const { NewMessage } = require("telegram/events");
const fs = require("fs");
const path = require("path");

const apiId = parseInt(process.env.API_ID);
const apiHash = process.env.API_HASH;
// Support STRING_SESSION atau SESSION_STRING dari .env
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

    const downloadDir = path.join(__dirname, "downloads");
    if (!fs.existsSync(downloadDir)) {
        fs.mkdirSync(downloadDir, { recursive: true });
    }

    client.addEventHandler(async (event) => {
        const msg = event.message;

        if (msg.text && msg.text.trim() === ".dr" && msg.isReply) {
            try {
                const targetMsg = await msg.getReplyMessage();

                if (!targetMsg || !targetMsg.media) {
                    await msg.edit({ text: "❌ Pesan yang di-reply tidak mengandung media/video!" });
                    return;
                }

                await msg.edit({ text: "⏳ Sedang memproses dan mendownload media..." });

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
                const filePath = path.join(downloadDir, fileName);

                const downloaded = await client.downloadMedia(targetMsg, {
                    outputFile: filePath,
                });

                await msg.edit({ text: `✅ Berhasil didownload!\n📁 File: ${path.basename(downloaded)}` });
                console.log(`[SUKSES] Media tersimpan di: ${downloaded}`);
            } catch (err) {
                console.error("❌ Gagal download:", err);
                await msg.edit({ text: `❌ Gagal download: ${err.message}` });
            }
        }
    }, new NewMessage({ outgoing: true }));
})();
