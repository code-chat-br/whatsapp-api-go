package chat

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"go.mau.fi/whatsmeow"

	dbtypes "whatsapp-go-api/internal/database/types"
)

func TestBuildDownloadableMessageSupportedTypes(t *testing.T) {
	tests := []struct {
		messageType string
		content     string
		mimeType    string
		fileName    string
	}{
		{
			messageType: MediaTypeImage,
			mimeType:    "image/jpeg",
			content:     baseMediaJSON(`"mimetype":"image/jpeg","fileLength":"221457","height":1536,"width":1024`),
		},
		{
			messageType: MediaTypeVideo,
			mimeType:    "video/mp4",
			content:     baseMediaJSON(`"mimetype":"video/mp4","fileLength":"1048576","height":1080,"width":1920,"seconds":12`),
		},
		{
			messageType: MediaTypeAudio,
			mimeType:    "audio/ogg",
			content:     baseMediaJSON(`"mimetype":"audio/ogg; codecs=opus","fileLength":"50000","seconds":8,"ptt":true`),
		},
		{
			messageType: MediaTypeDocument,
			mimeType:    "application/pdf",
			fileName:    "relatorio.pdf",
			content:     baseMediaJSON(`"mimetype":"application/pdf","fileLength":"204800","fileName":"relatorio.pdf","pageCount":3`),
		},
		{
			messageType: MediaTypeSticker,
			mimeType:    "image/webp",
			content:     baseMediaJSON(`"mimetype":"image/webp","fileLength":"20480","height":512,"width":512,"isAnimated":false`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.messageType, func(t *testing.T) {
			message, metadata, err := BuildDownloadableMessage(tt.messageType, json.RawMessage(tt.content))
			if err != nil {
				t.Fatalf("BuildDownloadableMessage() error = %v", err)
			}
			var downloadable whatsmeow.DownloadableMessage = message
			if downloadable.GetDirectPath() != "/media/path" {
				t.Fatalf("directPath = %q", downloadable.GetDirectPath())
			}
			if metadata.MediaType != tt.messageType {
				t.Fatalf("MediaType = %q, want %q", metadata.MediaType, tt.messageType)
			}
			if metadata.MIMEType != tt.mimeType {
				t.Fatalf("MIMEType = %q, want %q", metadata.MIMEType, tt.mimeType)
			}
			if tt.fileName != "" && metadata.FileName != tt.fileName {
				t.Fatalf("FileName = %q, want %q", metadata.FileName, tt.fileName)
			}
			if metadata.Size["fileLength"] == "" {
				t.Fatalf("expected fileLength metadata")
			}
		})
	}
}

func TestBuildDownloadableMessageValidation(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr error
	}{
		{name: "invalid base64", content: mediaJSONWith(`"mediaKey":"%%%"`), wantErr: ErrInvalidMediaContent},
		{name: "missing directPath", content: mediaJSONWithout("directPath"), wantErr: ErrInvalidMediaContent},
		{name: "directPath without slash", content: mediaJSONWith(`"directPath":"media/path"`), wantErr: ErrInvalidMediaContent},
		{name: "missing mediaKey", content: mediaJSONWithout("mediaKey"), wantErr: ErrInvalidMediaContent},
		{name: "missing fileSha256", content: mediaJSONWithout("fileSha256"), wantErr: ErrInvalidMediaContent},
		{name: "missing fileEncSha256", content: mediaJSONWithout("fileEncSha256"), wantErr: ErrInvalidMediaContent},
		{name: "unknown fields", content: baseMediaJSON(`"mimetype":"image/jpeg","unknownFutureField":true`), wantErr: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := BuildDownloadableMessage(MediaTypeImage, json.RawMessage(tt.content))
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("BuildDownloadableMessage() error = %v", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("BuildDownloadableMessage() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestBuildDownloadableMessageFallbackMIMEAndFileName(t *testing.T) {
	_, metadata, err := BuildDownloadableMessage(MediaTypeSticker, json.RawMessage(baseMediaJSON(`"mimetype":"bad\r\nmime"`)))
	if err != nil {
		t.Fatalf("BuildDownloadableMessage() error = %v", err)
	}
	if metadata.MIMEType != "image/webp" {
		t.Fatalf("MIMEType = %q, want image/webp", metadata.MIMEType)
	}

	fileName := CompleteMediaFileName(metadata, "../ABC\r\n")
	if fileName != "ABC.webp" {
		t.Fatalf("CompleteMediaFileName() = %q, want ABC.webp", fileName)
	}
}

func messageForMediaTest(messageType string, content string) dbtypes.Message {
	return dbtypes.Message{
		ID:          1,
		KeyID:       "ABC",
		MessageType: messageType,
		Content:     json.RawMessage(content),
		InstanceID:  10,
	}
}

func baseMediaJSON(extra string) string {
	base := `"url":"https://mmg.whatsapp.net/ignored","directPath":"/media/path","mediaKey":"AQIDBA==","fileSha256":"BQYHCA==","fileEncSha256":"CQoLDA=="`
	if strings.TrimSpace(extra) != "" {
		base += "," + extra
	}
	return "{" + base + "}"
}

func mediaJSONWith(replacement string) string {
	return "{" + `"url":"https://mmg.whatsapp.net/ignored","directPath":"/media/path","mediaKey":"AQIDBA==","fileSha256":"BQYHCA==","fileEncSha256":"CQoLDA==","mimetype":"image/jpeg",` + replacement + "}"
}

func mediaJSONWithout(field string) string {
	var object map[string]any
	if err := json.Unmarshal([]byte(baseMediaJSON(`"mimetype":"image/jpeg"`)), &object); err != nil {
		panic(err)
	}
	delete(object, field)
	encoded, err := json.Marshal(object)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}
