require("dotenv").config();
const { TelegramClient } = require("telegram");
const { StringSession } = require("telegram/sessions");
const fs = require("fs");
const path = require("path");

const SmartDownloadManager = require("./manager");
const { registerHandler } = require("./handler");

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

    const targetDir = "/sdcard/MEDIAT";
    if (!fs.existsSync(targetDir)) {
        fs.mkdirSync(targetDir, { recursive: true });
    }

    // Inisialisasi Manager & Handler
    const manager = new SmartDownloadManager(client, targetDir);
    registerHandler(client, manager);

    console.log("🚀 High-Performance Zero-Copy Core Aktif | Siap Menerima Perintah .dr");
})();
