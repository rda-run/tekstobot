package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"tekstobot/internal/config"
)

type WhisperClient struct {
	BaseURL string
	Model   string
}

func NewWhisperClient(cfg *config.Config) *WhisperClient {
	return &WhisperClient{
		BaseURL: cfg.WhisperURL,
		Model:   "medium",
	}
}

type whisperResponse struct {
	Text string `json:"text"`
}

func (c *WhisperClient) ProcessAudio(filePath string) (string, error) {
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

	url := fmt.Sprintf("%s/v1/audio/transcriptions", c.BaseURL)
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
