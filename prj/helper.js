function formatSize(bytes) {
    if (!bytes) return '0 B';
    const k = 1024, sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

function getProgressBar(current, total) {
    const percent = (current / total) * 100;
    const filled = Math.floor(percent / 10);
    return '■'.repeat(filled) + '□'.repeat(10 - filled) + ` ${percent.toFixed(1)}%`;
}

module.exports = { formatSize, getProgressBar };
