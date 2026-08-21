const { NewMessage } = require("telegram/events");

function registerHandler(client, manager) {
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
}

module.exports = { registerHandler };
