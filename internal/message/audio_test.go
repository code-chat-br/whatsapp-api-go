package message

import (
	"context"
	"errors"
	"math"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"go.mau.fi/whatsmeow"
)

func TestNormalizeDuration(t *testing.T) {
	tests := []struct {
		name    string
		value   float64
		want    uint32
		wantErr bool
	}{
		{name: "fraction rounds up", value: 1.2, want: 2},
		{name: "short audio becomes one", value: 0.2, want: 1},
		{name: "integer", value: 3, want: 3},
		{name: "zero", value: 0, wantErr: true},
		{name: "nan", value: math.NaN(), wantErr: true},
		{name: "inf", value: math.Inf(1), wantErr: true},
		{name: "above limit", value: 11, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeDuration(tt.value, 10)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeDuration() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("duration mismatch: want %d got %d", tt.want, got)
			}
		})
	}
}

func TestSafeFilename(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: `/home/user/audio.mp3`, want: "audio.mp3"},
		{input: `C:\Users\clebe\Music\audio.mp3`, want: "audio.mp3"},
		{input: "../audio.wav", want: "audio.wav"},
		{input: "..", want: ""},
		{input: "bad\x00name.ogg", want: "badname.ogg"},
	}
	for _, tt := range tests {
		if got := safeFilename(tt.input); got != tt.want {
			t.Fatalf("safeFilename(%q): want %q got %q", tt.input, tt.want, got)
		}
	}
}

func TestParseMultipartAudioOptions(t *testing.T) {
	options, err := ParseMultipartAudioOptions("1200", "recording", "1883", `{"keyId":"x"}`, "true")
	if err != nil {
		t.Fatalf("ParseMultipartAudioOptions() error = %v", err)
	}
	if options == nil || options.Delay == nil || *options.Delay != 1200 {
		t.Fatalf("delay not parsed: %#v", options)
	}
	if options.Presence == nil || *options.Presence != "recording" {
		t.Fatalf("presence not parsed: %#v", options)
	}
	if options.QuotedMessageID == nil || *options.QuotedMessageID != 1883 {
		t.Fatalf("quotedMessageId not parsed: %#v", options)
	}
	if options.QuotedMessage["keyId"] != "x" {
		t.Fatalf("quotedMessage not parsed: %#v", options.QuotedMessage)
	}
	if options.MentionAll == nil || !*options.MentionAll {
		t.Fatalf("mentionAll not parsed: %#v", options)
	}
}

func TestParseMultipartAudioOptionsRejectsBadTypes(t *testing.T) {
	if _, err := ParseMultipartAudioOptions("interger", "", "", "", ""); err == nil {
		t.Fatal("expected delay parse error")
	}
	if _, err := ParseMultipartAudioOptions("", "", "interger", "", ""); err == nil {
		t.Fatal("expected quotedMessageId parse error")
	}
	if _, err := ParseMultipartAudioOptions("", "", "", "[]", ""); err == nil {
		t.Fatal("expected quotedMessage object error")
	}
	if _, err := ParseMultipartAudioOptions("", "", "", "", "yes"); err == nil {
		t.Fatal("expected mentionAll boolean error")
	}
}

func TestValidateAudioOptions(t *testing.T) {
	delay := int64(300000)
	tooHigh := int64(300001)
	options, delayDuration, err := validateAudioOptions(&MessageOptions{Delay: &delay})
	if err != nil {
		t.Fatalf("validateAudioOptions() error = %v", err)
	}
	if options == nil || *options != "recording" {
		t.Fatalf("default presence mismatch: %#v", options)
	}
	if delayDuration.Milliseconds() != 300000 {
		t.Fatalf("delay mismatch: %v", delayDuration)
	}
	if _, _, err := validateAudioOptions(&MessageOptions{Delay: &tooHigh}); err == nil {
		t.Fatal("expected delay too high")
	}
	if _, _, err := validateAudioOptions(&MessageOptions{Presence: ptr("composing")}); err == nil {
		t.Fatal("expected composing rejection")
	}
	if _, _, err := validateAudioOptions(&MessageOptions{Presence: ptr("paused")}); err != nil {
		t.Fatalf("paused should be accepted: %v", err)
	}
}

func TestBuildPTTAudioMessage(t *testing.T) {
	upload := whatsmeow.UploadResponse{
		URL:           "https://mmg.whatsapp.net/audio",
		DirectPath:    "/v/t62/audio",
		MediaKey:      []byte("media-key"),
		FileEncSHA256: []byte("enc"),
		FileSHA256:    []byte("sha"),
		FileLength:    10,
	}
	prepared := PreparedAudio{
		Data:            []byte("audio-bytes"),
		MIMEType:        audioOutputMIME,
		Filename:        audioOutputFilename,
		DurationSeconds: 7,
		Codec:           "opus",
		Container:       "ogg",
	}
	msg, content := buildPTTAudioMessage(upload, prepared, nil, audioSource{Source: "upload", Filename: "input.mp3", MIMEType: "audio/mpeg"})
	audio := msg.GetAudioMessage()
	if audio == nil {
		t.Fatal("expected audio message")
	}
	if !audio.GetPTT() {
		t.Fatal("expected PTT=true")
	}
	if audio.GetSeconds() != 7 {
		t.Fatalf("seconds mismatch: %d", audio.GetSeconds())
	}
	if content["ptt"] != true || content["seconds"] != uint32(7) {
		t.Fatalf("content mismatch: %#v", content)
	}
}

func TestIsPublicIP(t *testing.T) {
	if isPublicIP(net.ParseIP("127.0.0.1")) {
		t.Fatal("loopback should be blocked")
	}
	if isPublicIP(net.ParseIP("10.0.0.1")) {
		t.Fatal("private IP should be blocked")
	}
	if !isPublicIP(net.ParseIP("8.8.8.8")) {
		t.Fatal("public IP should be accepted")
	}
}

func TestLooksLikeSupportedAudio(t *testing.T) {
	if !looksLikeSupportedAudio("audio/mpeg", "file.bin") {
		t.Fatal("audio/mpeg should be supported")
	}
	if !looksLikeSupportedAudio("application/octet-stream", "file.ogg") {
		t.Fatal("ogg extension should be supported")
	}
	if looksLikeSupportedAudio("text/plain", "file.txt") {
		t.Fatal("text should be rejected")
	}
}

func TestFFmpegAudioProcessorPreparePTT(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not installed")
	}
	tempDir := t.TempDir()
	wavPath := filepath.Join(tempDir, "input.wav")
	cmd := exec.Command(
		"ffmpeg",
		"-hide_banner",
		"-loglevel", "error",
		"-f", "lavfi",
		"-i", "sine=frequency=1000:duration=0.2",
		"-ac", "1",
		"-ar", "8000",
		"-y",
		wavPath,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate wav: %v %s", err, string(output))
	}
	input, err := os.ReadFile(wavPath)
	if err != nil {
		t.Fatalf("read wav: %v", err)
	}
	processor := NewFFmpegAudioProcessor(DefaultAudioConfig(), zerolog.Nop())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	prepared, err := processor.PreparePTT(ctx, input, "input.wav", "audio/wav")
	if err != nil {
		t.Fatalf("PreparePTT() error = %v", err)
	}
	if prepared.MIMEType != audioOutputMIME || prepared.Codec != "opus" || prepared.Container != "ogg" {
		t.Fatalf("unexpected prepared audio: %#v", prepared)
	}
	if prepared.DurationSeconds != 1 {
		t.Fatalf("short audio should round to one second, got %d", prepared.DurationSeconds)
	}
	if len(prepared.Data) == 0 {
		t.Fatal("expected converted bytes")
	}
}

func TestAudioErrorsAreDistinct(t *testing.T) {
	if errors.Is(ErrUnsupportedMediaType, ErrInvalidRequest) {
		t.Fatal("unsupported media must map independently")
	}
}
