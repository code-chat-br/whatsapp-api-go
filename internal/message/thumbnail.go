package message

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	"image/jpeg"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/disintegration/imaging"
	"github.com/rs/zerolog"
)

const (
	defaultThumbnailMaxWidth    = 320
	defaultThumbnailMaxHeight   = 320
	defaultThumbnailJPEGQuality = 75
	defaultThumbnailMaxInput    = 50 * 1024 * 1024
	defaultThumbnailTimeout     = 15 * time.Second
	defaultFFmpegPath           = "ffmpeg"
)

var ErrThumbnailFailed = errors.New("thumbnail generation failed")

type Thumbnail struct {
	Bytes  []byte
	Width  int
	Height int
}

type ThumbnailConfig struct {
	MaxWidth      int
	MaxHeight     int
	JPEGQuality   int
	MaxInputBytes int64
	Timeout       time.Duration
	FFmpegPath    string
	TempDir       string
}

type ThumbnailService interface {
	FromImage(ctx context.Context, media []byte) (Thumbnail, error)
	FromVideo(ctx context.Context, media []byte) (Thumbnail, error)
}

type thumbnailService struct {
	config ThumbnailConfig
	logger zerolog.Logger
}

func DefaultThumbnailConfig() ThumbnailConfig {
	return ThumbnailConfig{
		MaxWidth:      defaultThumbnailMaxWidth,
		MaxHeight:     defaultThumbnailMaxHeight,
		JPEGQuality:   defaultThumbnailJPEGQuality,
		MaxInputBytes: defaultThumbnailMaxInput,
		Timeout:       defaultThumbnailTimeout,
		FFmpegPath:    ffmpegPath(),
	}
}

func NewThumbnailService(config ThumbnailConfig, logger zerolog.Logger) ThumbnailService {
	config = normalizeThumbnailConfig(config)
	return &thumbnailService{
		config: config,
		logger: logger.With().Str("component", "thumbnail_service").Logger(),
	}
}

func normalizeThumbnailConfig(config ThumbnailConfig) ThumbnailConfig {
	if config.MaxWidth <= 0 {
		config.MaxWidth = defaultThumbnailMaxWidth
	}
	if config.MaxHeight <= 0 {
		config.MaxHeight = defaultThumbnailMaxHeight
	}
	if config.JPEGQuality <= 0 || config.JPEGQuality > 100 {
		config.JPEGQuality = defaultThumbnailJPEGQuality
	}
	if config.MaxInputBytes <= 0 {
		config.MaxInputBytes = defaultThumbnailMaxInput
	}
	if config.Timeout <= 0 {
		config.Timeout = defaultThumbnailTimeout
	}
	if config.FFmpegPath == "" {
		config.FFmpegPath = defaultFFmpegPath
	}
	return config
}

func (s *thumbnailService) FromImage(ctx context.Context, media []byte) (Thumbnail, error) {
	if err := s.validateInput(media); err != nil {
		return Thumbnail{}, err
	}
	select {
	case <-ctx.Done():
		return Thumbnail{}, ctx.Err()
	default:
	}
	source, _, err := image.Decode(bytes.NewReader(media))
	if err != nil {
		return Thumbnail{}, fmt.Errorf("%w: decode image: %w", ErrThumbnailFailed, err)
	}
	bounds := source.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return Thumbnail{}, fmt.Errorf("%w: invalid image dimensions", ErrThumbnailFailed)
	}
	flattened := flattenImageOnWhite(source)
	resized := resizeImage(flattened, s.config.MaxWidth, s.config.MaxHeight)
	out, err := encodeJPEG(resized, s.config.JPEGQuality)
	if err != nil {
		return Thumbnail{}, err
	}
	return Thumbnail{
		Bytes:  out,
		Width:  resized.Bounds().Dx(),
		Height: resized.Bounds().Dy(),
	}, nil
}

func (s *thumbnailService) FromVideo(ctx context.Context, media []byte) (Thumbnail, error) {
	if err := s.validateInput(media); err != nil {
		return Thumbnail{}, err
	}
	if err := validateExecutable(s.config.FFmpegPath); err != nil {
		return Thumbnail{}, fmt.Errorf("%w: ffmpeg unavailable: %w", ErrThumbnailFailed, err)
	}
	dir, err := os.MkdirTemp(s.config.TempDir, "whatsapp-go-api-thumbnail-*")
	if err != nil {
		return Thumbnail{}, fmt.Errorf("%w: temp dir: %w", ErrThumbnailFailed, err)
	}
	defer func() {
		if removeErr := os.RemoveAll(dir); removeErr != nil {
			s.logger.Debug().Err(removeErr).Str("dir", dir).Msg("failed to remove thumbnail temp dir")
		}
	}()

	inputPath := filepath.Join(dir, "input.video")
	if err := os.WriteFile(inputPath, media, 0600); err != nil {
		return Thumbnail{}, fmt.Errorf("%w: write video: %w", ErrThumbnailFailed, err)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()
	var lastErr error
	for index, seek := range []string{"0", "0.1", "0.5"} {
		select {
		case <-timeoutCtx.Done():
			return Thumbnail{}, timeoutCtx.Err()
		default:
		}
		outputPath := filepath.Join(dir, "frame-"+strconv.Itoa(index)+".jpg")
		if err := s.extractVideoFrame(timeoutCtx, inputPath, outputPath, seek); err != nil {
			lastErr = err
			continue
		}
		frame, err := os.ReadFile(outputPath)
		if err != nil {
			lastErr = fmt.Errorf("%w: read frame: %w", ErrThumbnailFailed, err)
			continue
		}
		thumbnail, err := s.FromImage(ctx, frame)
		if err != nil {
			lastErr = err
			continue
		}
		return thumbnail, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("%w: no video frame extracted", ErrThumbnailFailed)
	}
	return Thumbnail{}, lastErr
}

func (s *thumbnailService) validateInput(media []byte) error {
	if len(media) == 0 {
		return fmt.Errorf("%w: empty input", ErrThumbnailFailed)
	}
	if int64(len(media)) > s.config.MaxInputBytes {
		return fmt.Errorf("%w: input too large", ErrThumbnailFailed)
	}
	return nil
}

func (s *thumbnailService) extractVideoFrame(ctx context.Context, inputPath, outputPath, seek string) error {
	args := []string{
		"-hide_banner",
		"-loglevel", "error",
		"-y",
		"-ss", seek,
		"-i", inputPath,
		"-frames:v", "1",
		"-q:v", "4",
		outputPath,
	}
	cmd := exec.CommandContext(ctx, s.config.FFmpegPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return fmt.Errorf("%w: ffmpeg frame at %s: %w: %s", ErrThumbnailFailed, seek, err, stderr.String())
	}
	info, err := os.Stat(outputPath)
	if err != nil {
		return fmt.Errorf("%w: frame missing: %w", ErrThumbnailFailed, err)
	}
	if info.Size() == 0 {
		return fmt.Errorf("%w: empty frame", ErrThumbnailFailed)
	}
	return nil
}

func flattenImageOnWhite(source image.Image) image.Image {
	bounds := source.Bounds()
	target := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(target, target.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	draw.Draw(target, target.Bounds(), source, bounds.Min, draw.Over)
	return target
}

func resizeImage(source image.Image, maxWidth, maxHeight int) image.Image {
	width := source.Bounds().Dx()
	height := source.Bounds().Dy()
	if width <= maxWidth && height <= maxHeight {
		return source
	}
	return imaging.Fit(source, maxWidth, maxHeight, imaging.Lanczos)
}

func encodeJPEG(source image.Image, quality int) ([]byte, error) {
	var output bytes.Buffer
	if err := jpeg.Encode(&output, source, &jpeg.Options{Quality: quality}); err != nil {
		return nil, fmt.Errorf("%w: encode jpeg: %w", ErrThumbnailFailed, err)
	}
	if output.Len() == 0 {
		return nil, fmt.Errorf("%w: empty jpeg output", ErrThumbnailFailed)
	}
	return output.Bytes(), nil
}
