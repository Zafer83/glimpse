package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/Zafer83/glimpse/internal/config"
	"github.com/sashabaranov/go-openai"
)

func GenerateSlides(cfg *config.Config, code string) (string, error) {
	systemPrompt := fmt.Sprintf(
		"You are Glimpse, an expert AI architect. Your task is to generate a professional "+
			"Slidev presentation from the provided source code in %s language.", cfg.Language)

	userPrompt := fmt.Sprintf(
		"Create a high-quality Slidev Markdown presentation.\n"+
			"- Theme: '%s'\n"+
			"- Highlight core business logic and architectural patterns.\n"+
			"- Use code blocks with focus markers (e.g., {1-5|7-10}).\n"+
			"- Include at least one Mermaid.js diagram to visualize the flow.\n"+
			"- Return ONLY the raw Markdown content.\n\nSOURCE CODE:\n%s",
		cfg.Theme, code)

	if isGeminiModel(cfg.Model) {
		return generateSlidesWithGemini(cfg, systemPrompt, userPrompt)
	}

	client := openai.NewClient(cfg.APIKey)
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: cfg.Model,
			Messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
				{Role: openai.ChatMessageRoleUser, Content: userPrompt},
			},
		},
	)

	if err != nil {
		return "", err
	}

	return resp.Choices[0].Message.Content, nil
}

const (
	geminiBaseV1Beta = "https://generativelanguage.googleapis.com/v1beta"
	geminiBaseV1     = "https://generativelanguage.googleapis.com/v1"
)

type geminiRequest struct {
	SystemInstruction struct {
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	} `json:"system_instruction"`
	Contents []struct {
		Role  string `json:"role"`
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	} `json:"contents"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func generateSlidesWithGemini(cfg *config.Config, systemPrompt, userPrompt string) (string, error) {
	requestedModel := normalizeGeminiModel(cfg.Model)
	model := requestedModel
	if model == "" {
		return "", fmt.Errorf("gemini error: model is empty")
	}
	baseURL := geminiBaseV1Beta

	resolvedModel, resolvedBaseURL, _ := resolveGeminiModel(cfg.APIKey, requestedModel)
	if resolvedModel != "" {
		model = resolvedModel
		baseURL = resolvedBaseURL
	}

	var reqBody geminiRequest
	reqBody.SystemInstruction.Parts = []struct {
		Text string `json:"text"`
	}{
		{Text: systemPrompt},
	}
	reqBody.Contents = []struct {
		Role  string `json:"role"`
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	}{
		{
			Role: "user",
			Parts: []struct {
				Text string `json:"text"`
			}{
				{Text: userPrompt},
			},
		},
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	text, statusCode, rawErr, err := callGeminiGenerate(baseURL, model, cfg.APIKey, payload)
	if err == nil {
		return text, nil
	}

	// Retry once with the other API version to handle model/version mismatches.
	altBase := geminiBaseV1
	if baseURL == geminiBaseV1 {
		altBase = geminiBaseV1Beta
	}
	text, _, _, altErr := callGeminiGenerate(altBase, model, cfg.APIKey, payload)
	if altErr == nil {
		return text, nil
	}

	// Try one additional best-available fallback model (prefering free-tier-friendly options).
	fallbackModel, fallbackBase, resolveErr := resolveGeminiModel(cfg.APIKey, "")
	if resolveErr == nil && fallbackModel != "" && fallbackModel != model {
		text, _, _, fbErr := callGeminiGenerate(fallbackBase, fallbackModel, cfg.APIKey, payload)
		if fbErr == nil {
			return text, nil
		}
	}

	if statusCode == http.StatusTooManyRequests {
		return "", fmt.Errorf("gemini quota exceeded (status 429) on model %q. raw error: %s", model, rawErr)
	}
	return "", fmt.Errorf("gemini error: status %d: %s", statusCode, rawErr)
}

func callGeminiGenerate(baseURL, model, apiKey string, payload []byte) (string, int, string, error) {
	reqURL := fmt.Sprintf("%s/models/%s:generateContent?key=%s", baseURL, model, url.QueryEscape(apiKey))
	req, err := http.NewRequest(http.MethodPost, reqURL, bytes.NewReader(payload))
	if err != nil {
		return "", 0, "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", 0, "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", resp.StatusCode, strings.TrimSpace(string(body)), fmt.Errorf("gemini request failed")
	}

	var parsed geminiResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", resp.StatusCode, "", err
	}
	if parsed.Error != nil {
		return "", resp.StatusCode, parsed.Error.Message, fmt.Errorf("gemini response error")
	}
	if len(parsed.Candidates) == 0 || len(parsed.Candidates[0].Content.Parts) == 0 {
		return "", resp.StatusCode, "empty response", fmt.Errorf("gemini response error")
	}

	var out strings.Builder
	for _, part := range parsed.Candidates[0].Content.Parts {
		out.WriteString(part.Text)
	}
	result := strings.TrimSpace(out.String())
	if result == "" {
		return "", resp.StatusCode, "empty text content", fmt.Errorf("gemini response error")
	}
	return result, resp.StatusCode, "", nil
}

type geminiModelEntry struct {
	Name                       string   `json:"name"`
	SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
}

type geminiModelsResponse struct {
	Models        []geminiModelEntry `json:"models"`
	NextPageToken string             `json:"nextPageToken"`
	Error         *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func resolveGeminiModel(apiKey, requested string) (string, string, error) {
	preferredBases := []string{geminiBaseV1Beta, geminiBaseV1}
	requested = normalizeGeminiModel(requested)

	for _, base := range preferredBases {
		models, err := listGeminiModels(base, apiKey)
		if err != nil {
			continue
		}

		if requested != "" {
			if model := findExactGeminiModel(models, requested); model != "" {
				return model, base, nil
			}
		}

		if fallback := findPreferredGeminiModel(models); fallback != "" {
			return fallback, base, nil
		}
	}

	if requested != "" {
		return requested, geminiBaseV1Beta, fmt.Errorf("could not resolve requested model %q from ListModels", requested)
	}

	return "", "", fmt.Errorf("no Gemini model with generateContent support found")
}

func listGeminiModels(baseURL, apiKey string) ([]geminiModelEntry, error) {
	var all []geminiModelEntry
	pageToken := ""

	for {
		reqURL := fmt.Sprintf("%s/models?key=%s&pageSize=1000", baseURL, url.QueryEscape(apiKey))
		if pageToken != "" {
			reqURL += "&pageToken=" + url.QueryEscape(pageToken)
		}

		resp, err := http.Get(reqURL)
		if err != nil {
			return nil, err
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode >= 300 {
			return nil, fmt.Errorf("list models failed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}

		var parsed geminiModelsResponse
		if err := json.Unmarshal(body, &parsed); err != nil {
			return nil, err
		}
		if parsed.Error != nil {
			return nil, fmt.Errorf("list models error: %s", parsed.Error.Message)
		}

		all = append(all, parsed.Models...)
		if parsed.NextPageToken == "" {
			break
		}
		pageToken = parsed.NextPageToken
	}

	return all, nil
}

func findExactGeminiModel(models []geminiModelEntry, requested string) string {
	target := strings.ToLower(normalizeGeminiModel(requested))
	for _, m := range models {
		if !supportsGenerateContent(m.SupportedGenerationMethods) {
			continue
		}
		clean := strings.ToLower(normalizeGeminiModel(m.Name))
		if clean == target {
			return normalizeGeminiModel(m.Name)
		}
	}
	return ""
}

func findPreferredGeminiModel(models []geminiModelEntry) string {
	preferredPrefixes := []string{
		"gemini-2.0-flash-lite",
		"gemini-2.0-flash",
		"gemini-1.5-flash",
		"gemini-1.5-pro",
		"gemini-2.5-flash",
		"gemini-2.5-pro",
	}

	for _, prefix := range preferredPrefixes {
		for _, m := range models {
			if !supportsGenerateContent(m.SupportedGenerationMethods) {
				continue
			}
			clean := strings.ToLower(normalizeGeminiModel(m.Name))
			if strings.HasPrefix(clean, prefix) {
				return normalizeGeminiModel(m.Name)
			}
		}
	}

	for _, m := range models {
		if !supportsGenerateContent(m.SupportedGenerationMethods) {
			continue
		}
		clean := normalizeGeminiModel(m.Name)
		if strings.HasPrefix(strings.ToLower(clean), "gemini-") {
			return clean
		}
	}
	return ""
}

func supportsGenerateContent(methods []string) bool {
	for _, method := range methods {
		if method == "generateContent" {
			return true
		}
	}
	return false
}

func isGeminiModel(model string) bool {
	normalized := strings.ToLower(normalizeGeminiModel(model))
	return strings.HasPrefix(normalized, "gemini")
}

func normalizeGeminiModel(raw string) string {
	model := strings.TrimSpace(raw)
	model = strings.TrimPrefix(model, "models/")
	model = strings.TrimPrefix(model, "/")
	return model
}
