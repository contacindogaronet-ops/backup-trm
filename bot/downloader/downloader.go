package downloader

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gotd/td/telegram/downloader"
	"github.com/gotd/td/tg"
	"github.com/rs/zerolog/log"
)

type MediaTarget struct {
	Location tg.InputFileLocationClass
	FileName string
	Size     int64
	MimeType string
}

type ProgressCallback func(transferred, total int64, percent float64)

type Downloader struct {
	client     *tg.Client
	downloader *downloader.Downloader
}

func New(client *tg.Client) *Downloader {
	// Standar chunk part size 512KB
	d := downloader.NewDownloader().WithPartSize(512 * 1024)
	return &Downloader{
		client:     client,
		downloader: d,
	}
}

// DownloadMedia mendownload media stream ke file lokal
func (d *Downloader) DownloadMedia(ctx context.Context, target MediaTarget, outputDir string, progress ProgressCallback) (string, error) {
	if target.Location == nil {
		return "", errors.New("lokasi media tidak valid")
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("gagal membuat direktori output: %w", err)
	}

	fileName := target.FileName
	if fileName == "" {
		fileName = fmt.Sprintf("media_%d.bin", time.Now().Unix())
	}

	destPath := filepath.Join(outputDir, fileName)

	log.Info().
		Str("filename", fileName).
		Int64("size", target.Size).
		Msg("Memulai stream download...")

	builder := d.downloader.Download(d.client, target.Location)

	// Eksekusi download langsung ke file tujuan
	_, err := builder.ToPath(ctx, destPath)
	if err != nil {
		log.Error().Err(err).Str("file", destPath).Msg("Gagal mendownload media")
		return "", err
	}

	log.Info().Str("path", destPath).Msg("Download selesai")
	return destPath, nil
}
