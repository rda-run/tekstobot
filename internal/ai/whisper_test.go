package ai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"tekstobot/internal/config"
)

func TestWhisperClientProcessAudioSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
			return
		case "/v1/audio/transcriptions":
			if got := r.Method; got != http.MethodPost {
				t.Fatalf("method mismatch: got %s", got)
			}
			if !strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
				t.Fatalf("content type mismatch: %q", r.Header.Get("Content-Type"))
			}

			if err := r.ParseMultipartForm(1024 * 1024); err != nil {
				t.Fatalf("failed to parse multipart form: %v", err)
			}
			if got := r.FormValue("model"); got != "medium" {
				t.Fatalf("model mismatch: got %q", got)
			}
			if got := r.FormValue("response_format"); got != "json" {
				t.Fatalf("response format mismatch: got %q", got)
			}

			f, _, err := r.FormFile("file")
			if err != nil {
				t.Fatalf("missing file form field: %v", err)
			}
			defer f.Close()
			data, _ := io.ReadAll(f)
			if string(data) != "audio-bytes" {
				t.Fatalf("audio payload mismatch: got %q", string(data))
			}

			_, _ = w.Write([]byte(`{"text":"transcribed text"}`))
			return
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer ts.Close()

	audioFile, err := os.CreateTemp(t.TempDir(), "audio-*.ogg")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	if _, err := audioFile.WriteString("audio-bytes"); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	if err := audioFile.Close(); err != nil {
		t.Fatalf("close temp file: %v", err)
	}

	cfg := &config.Config{
		WhisperURL:            ts.URL,
		WhisperHealthInterval: 3600,
	}
	client := NewWhisperClient(cfg)

	got, err := client.ProcessAudio(audioFile.Name())
	if err != nil {
		t.Fatalf("ProcessAudio returned error: %v", err)
	}
	if got != "transcribed text" {
		t.Fatalf("transcription mismatch: got %q", got)
	}
}

func TestWhisperClientProcessAudioNoServers(t *testing.T) {
	cfg := &config.Config{
		WhisperURL:            "",
		WhisperHealthInterval: 30,
	}
	client := NewWhisperClient(cfg)

	audioFile, err := os.CreateTemp(t.TempDir(), "audio-*.ogg")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	if _, err := audioFile.WriteString("audio-bytes"); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	if err := audioFile.Close(); err != nil {
		t.Fatalf("close temp file: %v", err)
	}

	_, err = client.ProcessAudio(audioFile.Name())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no whisper servers available") {
		t.Fatalf("unexpected error: %v", err)
	}
}
