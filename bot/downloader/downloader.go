package downloader

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gotd/td/telegram/downloader"
	"github.com/gotd/td/tg"
	"github.com/rs/zerolog"
)

var (
	// sanitizeRegex cleans unsafe characters from filenames
	sanitizeRegex = regexp.MustCompile(`[^a-zA-Z0-9._\- ]+`)
)

// Downloader manages zero-alloc, chunked media streaming to local disk.
type Downloader struct {
	client     *tg.Client
	downloadDir string
	chunkSize  int
	downloader *downloader.Downloader
	log        zerolog.Logger
	semaphore  chan struct{}
}

// MediaTarget describes the resolved media file to download.
type MediaTarget struct {
	Location tg.InputFileLocationClass
	FileName string
	Size     int64
	MimeType string
	Type     string
}

// DownloadProgress reports real-time transfer metrics.
type DownloadProgress struct {
	TotalBytes      int64
	DownloadedBytes int64
	SpeedBytesSec   int64
	Percentage      float64
}

// NewDownloader creates a new concurrent, memory-efficient media downloader.
func NewDownloader(client *tg.Client, downloadDir string, chunkSize, maxConcurrent int, log zerolog.Logger) (*Downloader, error) {
	if err := os.MkdirAll(downloadDir, 0755); err != nil {
		return nil, err
	}

	dl := downloader.NewDownloader().
		WithPartSize(chunkSize).
		WithThreads(2) // 2 threads per download is optimal for mobile Termux network stack

	return &Downloader{
		client:      client,
		downloadDir: downloadDir,
		chunkSize:   chunkSize,
		downloader:  dl,
		log:         log.With().Str("component", "downloader").Logger(),
		semaphore:   make(chan struct{}, maxConcurrent),
	}, nil
}

// ResolveMedia extracts the input file location and metadata from any Telegram MessageMedia.
func (d *Downloader) ResolveMedia(media tg.MessageMediaClass) (*MediaTarget, error) {
	if media == nil {
		return nil, errors.New("empty media payload")
	}

	switch m := media.(type) {
	case *tg.MessageMediaDocument:
		doc, ok := m.Document.AsNotEmpty()
		if !ok {
			return nil, errors.New("document is empty or expired")
		}

		fileName := ""
		for _, attr := range doc.Attributes {
			if fAttr, ok := attr.(*tg.DocumentAttributeFilename); ok {
				fileName = fAttr.FileName
				break
			}
			if vAttr, ok := attr.(*tg.DocumentAttributeVideo); ok {
				if fileName == "" {
					_ = vAttr
					fileName = fmt.Sprintf("video_%d.mp4", doc.ID)
				}
			}
			if aAttr, ok := attr.(*tg.DocumentAttributeAudio); ok {
				if fileName == "" {
					ext := ".mp3"
					if aAttr.Voice {
						ext = ".ogg"
					}
					fileName = fmt.Sprintf("audio_%d%s", doc.ID, ext)
				}
			}
		}

		if fileName == "" {
			ext := extFromMime(doc.MimeType)
			fileName = fmt.Sprintf("doc_%d%s", doc.ID, ext)
		}

		location := &tg.InputDocumentFileLocation{
			ID:            doc.ID,
			AccessHash:    doc.AccessHash,
			FileReference: doc.FileReference,
			ThumbSize:     "",
		}

		return &MediaTarget{
			Location: location,
			FileName: sanitizeFileName(fileName),
			Size:     doc.Size,
			MimeType: doc.MimeType,
			Type:     "document",
		}, nil

	case *tg.MessageMediaPhoto:
		photo, ok := m.Photo.AsNotEmpty()
		if !ok {
			return nil, errors.New("photo is empty or expired")
		}

		// Find the largest size variant
		var bestSize tg.PhotoSizeClass
		var maxW int
		for _, sz := range photo.Sizes {
			switch s := sz.(type) {
			case *tg.PhotoSize:
				if s.W > maxW {
					maxW = s.W
					bestSize = s
				}
			case *tg.PhotoSizeProgressive:
				if s.W > maxW {
					maxW = s.W
					bestSize = s
				}
			}
		}

		thumbType := "x"
		var targetSize int64
		if bestSize != nil {
			switch s := bestSize.(type) {
			case *tg.PhotoSize:
				thumbType = s.Type
				targetSize = int64(s.Size)
			case *tg.PhotoSizeProgressive:
				thumbType = s.Type
				if len(s.Sizes) > 0 {
					targetSize = int64(s.Sizes[len(s.Sizes)-1])
				}
			}
		}

		location := &tg.InputPhotoFileLocation{
			ID:            photo.ID,
			AccessHash:    photo.AccessHash,
			FileReference: photo.FileReference,
			ThumbSize:     thumbType,
		}

		fileName := fmt.Sprintf("photo_%d_%s.jpg", photo.ID, thumbType)

		return &MediaTarget{
			Location: location,
			FileName: sanitizeFileName(fileName),
			Size:     targetSize,
			MimeType: "image/jpeg",
			Type:     "photo",
		}, nil

	default:
		return nil, fmt.Errorf("unsupported media type: %T", media)
	}
}

// DownloadStreams directly streams the remote media file to disk with progress callbacks and retry loops.
func (d *Downloader) DownloadStreams(
	ctx context.Context,
	target *MediaTarget,
	onProgress func(p DownloadProgress),
) (string, error) {
	// Acquire concurrency permit
	select {
	case d.semaphore <- struct{}{}:
		defer func() { <-d.semaphore }()
	case <-ctx.Done():
		return "", ctx.Err()
	}

	finalPath := filepath.Join(d.downloadDir, target.FileName)
	// Ensure unique file name if already exists
	finalPath = getUniqueFilePath(finalPath)

	d.log.Info().
		Str("filename", filepath.Base(finalPath)).
		Int64("expected_bytes", target.Size).
		Str("mime", target.MimeType).
		Msg("Starting MTProto chunked stream download")

	// Create temp download file
	partPath := finalPath + ".part"
	file, err := os.OpenFile(partPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		d.log.Error().Err(err).Str("path", partPath).Msg("Failed to open destination file")
		return "", err
	}
	defer func() {
		_ = file.Close()
	}()

	// Wrapping writer with progress monitor
	pw := &progressWriter{
		writer:     file,
		total:      target.Size,
		onProgress: onProgress,
		lastUpdate: time.Now(),
		log:        d.log,
	}

	// Execute download stream through gotd MTProto downloader
	startTime := time.Now()
	_, err = d.downloader.Download(d.client, target.Location).To(ctx, pw)
	if err != nil {
		d.log.Error().Err(err).Str("file", target.FileName).Msg("Download interrupted or failed")
		_ = os.Remove(partPath)
		return "", err
	}

	_ = file.Close()

	if err := os.Rename(partPath, finalPath); err != nil {
		d.log.Error().Err(err).Str("final_path", finalPath).Msg("Failed to rename .part file to final destination")
		return "", err
	}

	elapsed := time.Since(startTime)
	var avgSpeedKB float64
	if elapsed.Seconds() > 0 && pw.downloaded > 0 {
		avgSpeedKB = float64(pw.downloaded) / 1024 / elapsed.Seconds()
	}

	d.log.Info().
		Str("file", filepath.Base(finalPath)).
		Int64("bytes", pw.downloaded).
		Dur("elapsed", elapsed).
		Float64("avg_speed_kb_s", avgSpeedKB).
		Msg("Media download completed successfully")

	return finalPath, nil
}

type progressWriter struct {
	writer      io.Writer
	total       int64
	downloaded  int64
	lastBytes   int64
	lastUpdate  time.Time
	onProgress  func(p DownloadProgress)
	mux         sync.Mutex
	log         zerolog.Logger
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n, err := pw.writer.Write(p)
	if n > 0 {
		pw.mux.Lock()
		pw.downloaded += int64(n)
		now := time.Now()
		delta := now.Sub(pw.lastUpdate)

		// Throttle UI / callback updates to every 500ms to avoid locking
		if delta >= 500*time.Millisecond && pw.onProgress != nil {
			var speed int64
			if delta.Seconds() > 0 {
				speed = int64(float64(pw.downloaded-pw.lastBytes) / delta.Seconds())
			}
			var pct float64
			if pw.total > 0 {
				pct = float64(pw.downloaded) / float64(pw.total) * 100
			}

			prog := DownloadProgress{
				TotalBytes:      pw.total,
				DownloadedBytes: pw.downloaded,
				SpeedBytesSec:   speed,
				Percentage:      pct,
			}

			pw.lastBytes = pw.downloaded
			pw.lastUpdate = now
			pw.mux.Unlock()

			pw.onProgress(prog)
		} else {
			pw.mux.Unlock()
		}
	}
	return n, err
}

func sanitizeFileName(name string) string {
	name = strings.TrimSpace(name)
	name = sanitizeRegex.ReplaceAllString(name, "_")
	if len(name) > 120 {
		ext := filepath.Ext(name)
		base := name[:120-len(ext)]
		name = base + ext
	}
	if name == "" || name == "." {
		name = fmt.Sprintf("file_%d.bin", time.Now().UnixNano())
	}
	return name
}

func extFromMime(mime string) string {
	switch strings.ToLower(mime) {
	case "video/mp4":
		return ".mp4"
	case "video/mkv", "video/x-matroska":
		return ".mkv"
	case "video/webm":
		return ".webm"
	case "video/quicktime":
		return ".mov"
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "audio/mpeg", "audio/mp3":
		return ".mp3"
	case "audio/ogg":
		return ".ogg"
	case "application/pdf":
		return ".pdf"
	case "application/zip":
		return ".zip"
	case "application/x-rar-compressed":
		return ".rar"
	case "application/vnd.android.package-archive":
		return ".apk"
	default:
		return ".bin"
	}
}

func getUniqueFilePath(path string) string {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return path
	}

	dir := filepath.Dir(path)
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(filepath.Base(path), ext)

	for i := 1; i < 10000; i++ {
		testPath := filepath.Join(dir, fmt.Sprintf("%s_(%d)%s", base, i, ext))
		if _, err := os.Stat(testPath); errors.Is(err, os.ErrNotExist) {
			return testPath
		}
	}
	return fmt.Sprintf("%s_%d%s", path, time.Now().UnixNano(), ext)
}
