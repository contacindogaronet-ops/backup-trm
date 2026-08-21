require('dotenv').config();

module.exports = {
    apiId: parseInt(process.env.API_ID),
    apiHash: process.env.API_HASH,
    sessionStr: process.env.SESSION_STRING || '',
    downloadDir: process.env.DOWNLOAD_DIR || '/sdcard/MEDIAT',
    prefix: process.env.PREFIX || '.'
};

