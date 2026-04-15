package ai

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

const cloudflareWhisperModel = "@cf/openai/whisper-large-v3-turbo"

type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type CloudflareClient struct {
	AccountID  string
	APIToken   string
	Model      string
	Language   string
	HTTPClient httpDoer
	BaseURL    string
}

type cloudflareRunRequest struct {
	Audio    string `json:"audio"`
	Language string `json:"language,omitempty"`
}

type cloudflareRunOutput struct {
	Text string `json:"text"`
}

type cloudflareAPIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type cloudflareRunResponse struct {
	Success bool                 `json:"success"`
	Result  cloudflareRunOutput  `json:"result"`
	Errors  []cloudflareAPIError `json:"errors"`
}

func NewCloudflareClient(accountID, apiToken, language string) *CloudflareClient {
	return &CloudflareClient{
		AccountID:  strings.TrimSpace(accountID),
		APIToken:   strings.TrimSpace(apiToken),
		Model:      cloudflareWhisperModel,
		Language:   strings.TrimSpace(language),
		HTTPClient: http.DefaultClient,
		BaseURL:    "https://api.cloudflare.com/client/v4",
	}
}

func (c *CloudflareClient) ProcessAudio(filePath string) (string, error) {
	audioBytes, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read audio file: %w", err)
	}

	reqData := cloudflareRunRequest{
		Audio: base64.StdEncoding.EncodeToString(audioBytes),
	}
	if c.Language != "" {
		reqData.Language = c.Language
	}

	body, err := json.Marshal(reqData)
	if err != nil {
		return "", fmt.Errorf("failed to encode cloudflare request body: %w", err)
	}

	endpoint := fmt.Sprintf(
		"%s/accounts/%s/ai/run/%s",
		strings.TrimRight(c.BaseURL, "/"),
		c.AccountID,
		c.Model,
	)
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create cloudflare request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIToken)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("cloudflare http request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read cloudflare response: %w", err)
	}

	// Some Cloudflare examples show output directly while v4 endpoints wrap it in "result".
	var wrapped cloudflareRunResponse
	if err := json.Unmarshal(respBody, &wrapped); err == nil && (wrapped.Success || wrapped.Result.Text != "" || len(wrapped.Errors) > 0) {
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices || !wrapped.Success {
			return "", fmt.Errorf(
				"cloudflare transcription failed: status=%d code=%d message=%s",
				resp.StatusCode,
				firstCloudflareErrorCode(wrapped.Errors),
				firstCloudflareErrorMessage(wrapped.Errors),
			)
		}
		return wrapped.Result.Text, nil
	}

	var direct cloudflareRunOutput
	if err := json.Unmarshal(respBody, &direct); err == nil && direct.Text != "" {
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			return "", fmt.Errorf("cloudflare transcription failed: status=%d", resp.StatusCode)
		}
		return direct.Text, nil
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("cloudflare transcription failed: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	return "", fmt.Errorf("cloudflare response did not include transcription text")
}

func firstCloudflareErrorCode(errors []cloudflareAPIError) int {
	if len(errors) == 0 {
		return 0
	}
	return errors[0].Code
}

func firstCloudflareErrorMessage(errors []cloudflareAPIError) string {
	if len(errors) == 0 {
		return "unknown error"
	}
	msg := strings.TrimSpace(errors[0].Message)
	if msg == "" {
		return "unknown error"
	}
	return msg
}
