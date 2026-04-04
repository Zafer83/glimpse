package ai

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/Zafer83/glimpse/internal/config"
)

const anthropicBaseURL = "https://api.anthropic.com/v1/messages"

type anthropicRequest struct {
	Model     string `json:"model"`
	System    string `json:"system,omitempty"`
	MaxTokens int    `json:"max_tokens"`
	Messages  []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

// generateSlidesWithAnthropic sends the prompts to the Anthropic Messages API.
func generateSlidesWithAnthropic(cfg *config.Config, systemPrompt, userPrompt string) (string, error) {
	model := normalizeAnthropicModel(cfg.Model)
	if model == "" {
		return "", fmt.Errorf("anthropic error: model is empty")
	}

	var reqBody anthropicRequest
	reqBody.Model = model
	reqBody.System = systemPrompt
	reqBody.MaxTokens = 4096
	reqBody.Messages = []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}{
		{Role: "user", Content: userPrompt},
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("anthropic marshal: %w", err)
	}

	headers := map[string]string{
		"Content-Type":      "application/json",
		"x-api-key":         cfg.APIKey,
		"anthropic-version": "2023-06-01",
	}

	var parsed anthropicResponse
	if _, err := doJSONRequest(http.MethodPost, anthropicBaseURL, headers, payload, &parsed); err != nil {
		return "", fmt.Errorf("anthropic request: %w", err)
	}
	if parsed.Error != nil {
		return "", fmt.Errorf("anthropic API error: %s", parsed.Error.Message)
	}

	var out strings.Builder
	for _, item := range parsed.Content {
		if item.Type == "text" {
			out.WriteString(item.Text)
		}
	}

	result := strings.TrimSpace(out.String())
	if result == "" {
		return "", fmt.Errorf("anthropic returned empty response")
	}
	return result, nil
}

func normalizeAnthropicModel(raw string) string {
	model := strings.TrimSpace(raw)
	model = strings.TrimPrefix(model, "anthropic/")
	model = strings.TrimPrefix(model, "/")
	return model
}
