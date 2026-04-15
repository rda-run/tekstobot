package ai

import (
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestCloudflareClientProcessAudioSuccess(t *testing.T) {
	audioFile, err := os.CreateTemp(t.TempDir(), "audio-*.ogg")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	if _, err := audioFile.WriteString("fake-audio"); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	if err := audioFile.Close(); err != nil {
		t.Fatalf("close temp file: %v", err)
	}

	client := NewCloudflareClient("acc-123", "token-abc", "en")
	client.BaseURL = "https://api.cloudflare.com/client/v4"
	client.HTTPClient = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.Method; got != http.MethodPost {
			t.Fatalf("method mismatch: got %s", got)
		}
		if !strings.HasSuffix(req.URL.String(), "/accounts/acc-123/ai/run/@cf/openai/whisper-large-v3-turbo") {
			t.Fatalf("unexpected endpoint: %s", req.URL.String())
		}
		if got := req.Header.Get("Authorization"); got != "Bearer token-abc" {
			t.Fatalf("auth header mismatch: %q", got)
		}
		if got := req.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("content type mismatch: %q", got)
		}

		body, _ := io.ReadAll(req.Body)
		if !strings.Contains(string(body), `"audio"`) {
			t.Fatalf("request body is missing audio field: %s", string(body))
		}
		if !strings.Contains(string(body), `"language":"en"`) {
			t.Fatalf("request body is missing language field: %s", string(body))
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(
				`{"success":true,"result":{"text":"hello from cloudflare"}}`,
			)),
			Header: make(http.Header),
		}, nil
	})

	got, err := client.ProcessAudio(audioFile.Name())
	if err != nil {
		t.Fatalf("ProcessAudio returned error: %v", err)
	}
	if got != "hello from cloudflare" {
		t.Fatalf("transcription mismatch: got %q", got)
	}
}

func TestCloudflareClientProcessAudioAPIError(t *testing.T) {
	audioFile, err := os.CreateTemp(t.TempDir(), "audio-*.ogg")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	if _, err := audioFile.WriteString("fake-audio"); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	if err := audioFile.Close(); err != nil {
		t.Fatalf("close temp file: %v", err)
	}

	client := NewCloudflareClient("acc-123", "token-abc", "")
	client.HTTPClient = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Body: io.NopCloser(strings.NewReader(
				`{"success":false,"errors":[{"code":10000,"message":"authentication error"}]}`,
			)),
			Header: make(http.Header),
		}, nil
	})

	_, err = client.ProcessAudio(audioFile.Name())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	msg := err.Error()
	if !strings.Contains(msg, "cloudflare transcription failed") {
		t.Fatalf("expected cloudflare failure message, got %q", msg)
	}
	if !strings.Contains(msg, "status=401") {
		t.Fatalf("expected status code in message, got %q", msg)
	}
	if !strings.Contains(msg, "code=10000") {
		t.Fatalf("expected error code in message, got %q", msg)
	}
}
