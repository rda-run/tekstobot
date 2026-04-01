package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"tekstobot/internal/config"
)

type WhisperClient struct {
	URLs         []string
	availability []bool
	mutex        sync.RWMutex
	Model        string
}

func NewWhisperClient(cfg *config.Config) *WhisperClient {
	rawURLs := strings.Split(cfg.WhisperURL, ",")
	var urls []string
	for _, u := range rawURLs {
		trimmed := strings.TrimSpace(u)
		if trimmed != "" {
			trimmed = strings.TrimRight(trimmed, "/")
			urls = append(urls, trimmed)
		}
	}

	client := &WhisperClient{
		URLs:         urls,
		availability: make([]bool, len(urls)),
		Model:        "medium",
	}

	// Assume all are available until checked
	for i := range client.availability {
		client.availability[i] = true
	}

	// Start health check routine
	interval := time.Duration(cfg.WhisperHealthInterval) * time.Second
	if interval <= 0 {
		interval = 30 * time.Second
	}
	go client.startHealthCheck(interval)

	return client
}

func (c *WhisperClient) startHealthCheck(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Initial check
	c.checkAll()
	for range ticker.C {
		c.checkAll()
	}
}

func (c *WhisperClient) checkAll() {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	for i, u := range c.URLs {
		reqURL := fmt.Sprintf("%s/health", u)
		resp, err := client.Get(reqURL)
		
		isAvailable := err == nil && resp.StatusCode >= 200 && resp.StatusCode < 400
		if err == nil {
			resp.Body.Close()
		}

		c.mutex.Lock()
		if c.availability[i] != isAvailable {
			if isAvailable {
				log.Printf("Whisper server '%s' is now UP", u)
			} else {
				log.Printf("Whisper server '%s' is now DOWN", u)
			}
			c.availability[i] = isAvailable
		}
		c.mutex.Unlock()
	}
}

type whisperResponse struct {
	Text string `json:"text"`
}

func (c *WhisperClient) ProcessAudio(filePath string) (string, error) {
	c.mutex.RLock()
	var targetURL string
	for i, isAvailable := range c.availability {
		if isAvailable {
			targetURL = c.URLs[i]
			break
		}
	}
	c.mutex.RUnlock()

	if targetURL == "" {
		return "", fmt.Errorf("no whisper servers available")
	}

	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open audio file: %w", err)
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add model parameter expected by OpenAI compatible APIs
	if err := writer.WriteField("model", c.Model); err != nil {
		return "", fmt.Errorf("failed to write model field: %w", err)
	}

	// Request JSON response
	if err := writer.WriteField("response_format", "json"); err != nil {
		return "", fmt.Errorf("failed to write response_format field: %w", err)
	}

	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return "", fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := io.Copy(part, file); err != nil {
		return "", fmt.Errorf("failed to copy file content: %w", err)
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("failed to close multipart writer: %w", err)
	}

	url := fmt.Sprintf("%s/v1/audio/transcriptions", targetURL)
	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return "", fmt.Errorf("failed to create whisper request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("whisper http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("whisper returned non-200 status %d: %s", resp.StatusCode, respBody)
	}

	var resData whisperResponse
	if err := json.NewDecoder(resp.Body).Decode(&resData); err != nil {
		return "", fmt.Errorf("failed to decode whisper response: %w", err)
	}

	return resData.Text, nil
}
