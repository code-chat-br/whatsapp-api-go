package message

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"go.mau.fi/whatsmeow"
	"google.golang.org/protobuf/proto"
)

func TestThumbnailFromJPEG(t *testing.T) {
	service := testThumbnailService(ThumbnailConfig{MaxWidth: 100, MaxHeight: 100})
	source := testJPEG(t, 400, 200)

	thumbnail, err := service.FromImage(context.Background(), source)
	if err != nil {
		t.Fatalf("FromImage() error = %v", err)
	}
	if len(thumbnail.Bytes) == 0 {
		t.Fatal("expected thumbnail bytes")
	}
	decoded, err := jpeg.Decode(bytes.NewReader(thumbnail.Bytes))
	if err != nil {
		t.Fatalf("thumbnail is not a JPEG: %v", err)
	}
	if decoded.Bounds().Dx() != 100 || decoded.Bounds().Dy() != 50 {
		t.Fatalf("unexpected dimensions: %dx%d", decoded.Bounds().Dx(), decoded.Bounds().Dy())
	}
}

func TestThumbnailFromTransparentPNGUsesWhiteBackground(t *testing.T) {
	service := testThumbnailService(ThumbnailConfig{MaxWidth: 100, MaxHeight: 100})
	source := testTransparentPNG(t, 80, 80)

	thumbnail, err := service.FromImage(context.Background(), source)
	if err != nil {
		t.Fatalf("FromImage() error = %v", err)
	}
	decoded, err := jpeg.Decode(bytes.NewReader(thumbnail.Bytes))
	if err != nil {
		t.Fatalf("thumbnail is not a JPEG: %v", err)
	}
	r, g, b, _ := decoded.At(0, 0).RGBA()
	if r>>8 < 240 || g>>8 < 240 || b>>8 < 240 {
		t.Fatalf("transparent background was not flattened to white: rgb=(%d,%d,%d)", r>>8, g>>8, b>>8)
	}
}

func TestThumbnailFromInvalidImage(t *testing.T) {
	service := testThumbnailService(ThumbnailConfig{})

	thumbnail, err := service.FromImage(context.Background(), []byte("not an image"))
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrThumbnailFailed) {
		t.Fatalf("expected ErrThumbnailFailed, got %v", err)
	}
	if len(thumbnail.Bytes) != 0 {
		t.Fatal("expected no partial thumbnail")
	}
}

func TestThumbnailInputSizeLimit(t *testing.T) {
	service := testThumbnailService(ThumbnailConfig{MaxInputBytes: 3})

	_, err := service.FromImage(context.Background(), []byte("too large"))
	if !errors.Is(err, ErrThumbnailFailed) {
		t.Fatalf("expected ErrThumbnailFailed, got %v", err)
	}
}

func TestThumbnailFromVideo(t *testing.T) {
	video := testVideo(t)
	service := testThumbnailService(ThumbnailConfig{MaxWidth: 100, MaxHeight: 100})

	thumbnail, err := service.FromVideo(context.Background(), video)
	if err != nil {
		t.Fatalf("FromVideo() error = %v", err)
	}
	assertJPEGWithin(t, thumbnail.Bytes, 100, 100)
}

func TestThumbnailFromInvalidVideoCleansTemp(t *testing.T) {
	requireFFmpeg(t)
	tempDir := t.TempDir()
	service := testThumbnailService(ThumbnailConfig{TempDir: tempDir})

	thumbnail, err := service.FromVideo(context.Background(), []byte("not a video"))
	if err == nil {
		t.Fatal("expected error")
	}
	if len(thumbnail.Bytes) != 0 {
		t.Fatal("expected no thumbnail")
	}
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("read temp dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("temporary files were not cleaned up: %v", entries)
	}
}

func TestThumbnailFromVideoTimeout(t *testing.T) {
	requireFFmpeg(t)
	service := testThumbnailService(ThumbnailConfig{Timeout: time.Nanosecond})
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()

	_, err := service.FromVideo(ctx, []byte("not a video"))
	if err == nil {
		t.Fatal("expected timeout or cancellation error")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context error, got %v", err)
	}
}

func TestBuildMediaProtoIncludesThumbnails(t *testing.T) {
	thumbnail := testJPEG(t, 20, 20)
	upload := whatsmeow.UploadResponse{
		URL:           "https://example.invalid/media",
		DirectPath:    "/media",
		MediaKey:      []byte("media-key"),
		FileSHA256:    []byte("sha"),
		FileEncSHA256: []byte("enc-sha"),
		FileLength:    123,
	}
	media := &MediaMessage{Caption: proto.String("caption")}

	imageMsg, _, _, err := buildMediaProto(KindImage, media, "image/jpeg", upload, nil, thumbnail)
	if err != nil {
		t.Fatalf("build image: %v", err)
	}
	if len(imageMsg.GetImageMessage().GetJPEGThumbnail()) == 0 {
		t.Fatal("expected image thumbnail")
	}

	videoMsg, _, _, err := buildMediaProto(KindVideo, media, "video/mp4", upload, nil, thumbnail)
	if err != nil {
		t.Fatalf("build video: %v", err)
	}
	if len(videoMsg.GetVideoMessage().GetJPEGThumbnail()) == 0 {
		t.Fatal("expected video thumbnail")
	}

	ptvMsg, _, _, err := buildMediaProto(KindPTV, media, "video/mp4", upload, nil, thumbnail)
	if err != nil {
		t.Fatalf("build ptv: %v", err)
	}
	if ptvMsg.GetPtvMessage() == nil {
		t.Fatal("expected ptv message envelope")
	}
	if len(ptvMsg.GetPtvMessage().GetJPEGThumbnail()) == 0 {
		t.Fatal("expected ptv thumbnail")
	}
}

func TestBuildMediaProtoAllowsMissingThumbnail(t *testing.T) {
	upload := whatsmeow.UploadResponse{
		URL:           "https://example.invalid/media",
		DirectPath:    "/media",
		MediaKey:      []byte("media-key"),
		FileSHA256:    []byte("sha"),
		FileEncSHA256: []byte("enc-sha"),
		FileLength:    123,
	}

	msg, _, _, err := buildMediaProto(KindImage, &MediaMessage{}, "image/jpeg", upload, nil, nil)
	if err != nil {
		t.Fatalf("build image: %v", err)
	}
	if msg.GetImageMessage() == nil {
		t.Fatal("expected image message")
	}
	if len(msg.GetImageMessage().GetJPEGThumbnail()) != 0 {
		t.Fatal("expected nil thumbnail on generation failure")
	}
}

func testThumbnailService(config ThumbnailConfig) ThumbnailService {
	return NewThumbnailService(config, zerolog.Nop())
}

func testJPEG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{R: 200, G: 20, B: 20, A: 255}}, image.Point{}, draw.Src)
	var output bytes.Buffer
	if err := jpeg.Encode(&output, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	return output.Bytes()
}

func testTransparentPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	var output bytes.Buffer
	if err := png.Encode(&output, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return output.Bytes()
}

func testVideo(t *testing.T) []byte {
	t.Helper()
	ffmpeg := requireFFmpeg(t)
	path := filepath.Join(t.TempDir(), "sample.mp4")
	cmd := exec.Command(
		ffmpeg,
		"-hide_banner",
		"-loglevel", "error",
		"-y",
		"-f", "lavfi",
		"-i", "testsrc=size=160x120:rate=1",
		"-t", "1",
		"-pix_fmt", "yuv420p",
		path,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate test video: %v: %s", err, string(output))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read test video: %v", err)
	}
	return data
}

func requireFFmpeg(t *testing.T) string {
	t.Helper()
	ffmpeg, err := exec.LookPath(defaultFFmpegPath)
	if err != nil {
		t.Skip("ffmpeg is not available")
	}
	return ffmpeg
}

func assertJPEGWithin(t *testing.T, data []byte, maxWidth, maxHeight int) {
	t.Helper()
	if len(data) == 0 {
		t.Fatal("expected jpeg bytes")
	}
	decoded, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("thumbnail is not a JPEG: %v", err)
	}
	if decoded.Bounds().Dx() > maxWidth || decoded.Bounds().Dy() > maxHeight {
		t.Fatalf("thumbnail exceeds bounds: %dx%d", decoded.Bounds().Dx(), decoded.Bounds().Dy())
	}
}
