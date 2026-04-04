/*
Copyright 2026 Zafer Kılıçaslan

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package ai

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/Zafer83/glimpse/internal/config"
)

const ollamaBaseURL = "http://127.0.0.1:11434"

// ollamaTagsResponse represents the response from /api/tags listing available models.
type ollamaTagsResponse struct {
	Models []struct {
		Name string `json:"name"`
	} `json:"models"`
}

// Ollama native API types.

type ollamaChatRequest struct {
	Model    string `json:"model"`
	Stream   bool   `json:"stream"`
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
	Options struct {
		NumPredict  int     `json:"num_predict"`
		Temperature float64 `json:"temperature"`
	} `json:"options"`
}

type ollamaChatResponse struct {
	Message *struct {
		Content string `json:"content"`
	} `json:"message,omitempty"`
	Error string `json:"error,omitempty"`
}

// OpenAI-compatible local server types (e.g. llama.cpp, LM Studio).

type localOpenAIChatRequest struct {
	Model    string `json:"model"`
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
	Temperature float64 `json:"temperature,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
}

type localOpenAIChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// generateSlidesWithLocalLLM tries an OpenAI-compatible endpoint first,
// then falls back to Ollama's native /api/chat.
func generateSlidesWithLocalLLM(cfg *config.Config, systemPrompt, userPrompt string) (string, error) {
	model := normalizeLocalModel(cfg.Model)
	if model == "" {
		return "", fmt.Errorf("local llm error: model is empty")
	}

	// If the user entered just "local" (no specific model), auto-detect from Ollama.
	if isGenericLocalInput(cfg.Model) {
		detected, err := detectOllamaModel(cfg.LocalBaseURL, model)
		if err == nil && detected != "" {
			model = detected
		}
	}

	fmt.Printf("  Using model: %s\n", model)

	// Try OpenAI-compatible local servers first (e.g. llama.cpp /v1/chat/completions).
	if text, err := generateSlidesWithLocalOpenAICompat(cfg, model, systemPrompt, userPrompt); err == nil {
		return text, nil
	}

	// Fallback to Ollama /api/chat.
	return generateSlidesWithOllama(cfg, model, systemPrompt, userPrompt)
}

// isGenericLocalInput returns true when the user entered just "local" or "ollama"
// without specifying a particular model name.
func isGenericLocalInput(raw string) bool {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	return normalized == "local" || normalized == "ollama"
}

// detectOllamaModel queries Ollama's /api/tags and returns the best available model.
// If the preferred default is installed, it is returned. Otherwise the first
// available model is used. Returns empty string if no models are found.
func detectOllamaModel(baseURL, preferred string) (string, error) {
	endpoint := strings.TrimRight(baseURL, "/") + "/api/tags"
	if !strings.HasPrefix(endpoint, "http") {
		endpoint = "http://" + endpoint
	}

	var resp ollamaTagsResponse
	if _, err := doJSONRequest(http.MethodGet, endpoint, nil, nil, &resp); err != nil {
		return "", err
	}

	if len(resp.Models) == 0 {
		return "", fmt.Errorf("no models installed in Ollama")
	}

	// Check if the preferred model is available.
	for _, m := range resp.Models {
		name := strings.Split(m.Name, ":")[0] // strip tag for matching
		if m.Name == preferred || name == strings.Split(preferred, ":")[0] {
			return m.Name, nil
		}
	}

	// Preferred not found — return the first available model.
	return resp.Models[0].Name, nil
}

func generateSlidesWithOllama(cfg *config.Config, model, systemPrompt, userPrompt string) (string, error) {
	var reqBody ollamaChatRequest
	reqBody.Model = model
	reqBody.Stream = false
	reqBody.Options.NumPredict = 8192
	reqBody.Options.Temperature = 0.05
	reqBody.Messages = []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("ollama marshal: %w", err)
	}

	endpoint := localOllamaChatEndpoint(cfg.LocalBaseURL)

	var parsed ollamaChatResponse
	if _, err := doJSONRequest(http.MethodPost, endpoint, jsonHeader, payload, &parsed); err != nil {
		return "", fmt.Errorf("ollama request (is server running at %s?): %w", endpoint, err)
	}
	if parsed.Error != "" {
		return "", fmt.Errorf("ollama error: %s", parsed.Error)
	}
	if parsed.Message == nil || strings.TrimSpace(parsed.Message.Content) == "" {
		return "", fmt.Errorf("ollama returned empty response")
	}

	return strings.TrimSpace(parsed.Message.Content), nil
}

func generateSlidesWithLocalOpenAICompat(cfg *config.Config, model, systemPrompt, userPrompt string) (string, error) {
	var reqBody localOpenAIChatRequest
	reqBody.Model = model
	reqBody.Temperature = 0.05
	reqBody.MaxTokens = 8192
	reqBody.Messages = []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("local openai-compat marshal: %w", err)
	}

	endpoint := localOpenAIChatEndpoint(cfg.LocalBaseURL)
	headers := map[string]string{"Content-Type": "application/json"}

	key := strings.TrimSpace(cfg.APIKey)
	if key != "" && !strings.EqualFold(key, "none") {
		headers["Authorization"] = "Bearer " + key
	}

	var parsed localOpenAIChatResponse
	if _, err := doJSONRequest(http.MethodPost, endpoint, headers, payload, &parsed); err != nil {
		return "", fmt.Errorf("local openai-compat request: %w", err)
	}
	if parsed.Error != nil {
		return "", fmt.Errorf("local openai-compat error: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("local openai-compat returned empty response")
	}

	content := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if content == "" {
		return "", fmt.Errorf("local openai-compat returned empty content")
	}
	return content, nil
}

// --- Model and endpoint normalization ---

func normalizeLocalModel(raw string) string {
	model := strings.TrimSpace(raw)
	model = strings.TrimPrefix(model, "local/")
	model = strings.TrimPrefix(model, "ollama/")
	model = strings.TrimPrefix(model, "/")
	if strings.EqualFold(model, "local") || strings.EqualFold(model, "ollama") || model == "" {
		return "qwen2.5-coder:7b"
	}
	return model
}

func localOllamaChatEndpoint(raw string) string {
	base := strings.TrimSpace(raw)
	if base == "" {
		base = ollamaBaseURL
	}
	if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		base = "http://" + base
	}
	base = strings.TrimRight(base, "/")
	if strings.HasSuffix(base, "/api/chat") {
		return base
	}
	return base + "/api/chat"
}

func localOpenAIChatEndpoint(raw string) string {
	base := strings.TrimSpace(raw)
	if base == "" {
		base = ollamaBaseURL
	}
	if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		base = "http://" + base
	}
	base = strings.TrimRight(base, "/")

	if strings.HasSuffix(base, "/v1/chat/completions") {
		return base
	}
	if strings.HasSuffix(base, "/v1/models") {
		return strings.TrimSuffix(base, "/models") + "/chat/completions"
	}
	if strings.HasSuffix(base, "/v1") {
		return base + "/chat/completions"
	}
	return base + "/v1/chat/completions"
}
