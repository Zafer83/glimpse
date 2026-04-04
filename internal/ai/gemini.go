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
	"net/url"
	"strings"

	"github.com/Zafer83/glimpse/internal/config"
)

const (
	geminiBaseV1Beta = "https://generativelanguage.googleapis.com/v1beta"
	geminiBaseV1     = "https://generativelanguage.googleapis.com/v1"
)

// Gemini API request and response types.

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

// generateSlidesWithGemini builds the request and calls the Gemini API with
// automatic fallback across API versions and models.
func generateSlidesWithGemini(cfg *config.Config, systemPrompt, userPrompt string) (string, error) {
	model := normalizeGeminiModel(cfg.Model)
	if model == "" {
		return "", fmt.Errorf("gemini error: model is empty")
	}
	baseURL := geminiBaseV1Beta

	if resolved, resolvedBase, err := resolveGeminiModel(cfg.APIKey, model); err == nil && resolved != "" {
		model = resolved
		baseURL = resolvedBase
	}

	payload, err := buildGeminiPayload(systemPrompt, userPrompt)
	if err != nil {
		return "", fmt.Errorf("gemini payload marshal: %w", err)
	}

	// First attempt with the resolved base URL.
	text, statusCode, err := callGeminiGenerate(baseURL, model, cfg.APIKey, payload)
	if err == nil {
		return text, nil
	}
	firstErr := err

	// Retry with the other API version to handle model/version mismatches.
	altBase := geminiBaseV1
	if baseURL == geminiBaseV1 {
		altBase = geminiBaseV1Beta
	}
	if text, _, err := callGeminiGenerate(altBase, model, cfg.APIKey, payload); err == nil {
		return text, nil
	}

	// Try one additional best-available fallback model (preferring free-tier-friendly options).
	if fallback, fallbackBase, resolveErr := resolveGeminiModel(cfg.APIKey, ""); resolveErr == nil && fallback != "" && fallback != model {
		if text, _, err := callGeminiGenerate(fallbackBase, fallback, cfg.APIKey, payload); err == nil {
			return text, nil
		}
	}

	if statusCode == http.StatusTooManyRequests {
		return "", fmt.Errorf("gemini quota exceeded (429) on model %q: %w", model, firstErr)
	}
	return "", fmt.Errorf("gemini error on model %q: %w", model, firstErr)
}

// buildGeminiPayload marshals the system and user prompts into a Gemini request body.
func buildGeminiPayload(systemPrompt, userPrompt string) ([]byte, error) {
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
	return json.Marshal(reqBody)
}

// callGeminiGenerate performs a single generateContent request against the Gemini API.
func callGeminiGenerate(baseURL, model, apiKey string, payload []byte) (string, int, error) {
	reqURL := fmt.Sprintf("%s/models/%s:generateContent?key=%s", baseURL, model, url.QueryEscape(apiKey))

	var parsed geminiResponse
	statusCode, err := doJSONRequest(http.MethodPost, reqURL, jsonHeader, payload, &parsed)
	if err != nil {
		return "", statusCode, fmt.Errorf("gemini generate: %w", err)
	}

	if parsed.Error != nil {
		return "", statusCode, fmt.Errorf("gemini API error: %s", parsed.Error.Message)
	}
	if len(parsed.Candidates) == 0 || len(parsed.Candidates[0].Content.Parts) == 0 {
		return "", statusCode, fmt.Errorf("gemini returned empty response")
	}

	var out strings.Builder
	for _, part := range parsed.Candidates[0].Content.Parts {
		out.WriteString(part.Text)
	}
	result := strings.TrimSpace(out.String())
	if result == "" {
		return "", statusCode, fmt.Errorf("gemini returned empty text")
	}
	return result, statusCode, nil
}

// resolveGeminiModel looks up available models from the Gemini API and returns
// the best match for the requested model name, or a preferred fallback.
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
		return requested, geminiBaseV1Beta, fmt.Errorf("could not resolve model %q via ListModels", requested)
	}
	return "", "", fmt.Errorf("no Gemini model with generateContent support found")
}

// listGeminiModels fetches all available models from the given Gemini API base URL.
func listGeminiModels(baseURL, apiKey string) ([]geminiModelEntry, error) {
	var all []geminiModelEntry
	pageToken := ""

	for {
		reqURL := fmt.Sprintf("%s/models?key=%s&pageSize=1000", baseURL, url.QueryEscape(apiKey))
		if pageToken != "" {
			reqURL += "&pageToken=" + url.QueryEscape(pageToken)
		}

		var parsed geminiModelsResponse
		if _, err := doJSONRequest(http.MethodGet, reqURL, nil, nil, &parsed); err != nil {
			return nil, fmt.Errorf("gemini list models: %w", err)
		}
		if parsed.Error != nil {
			return nil, fmt.Errorf("gemini list models: %s", parsed.Error.Message)
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

	// Fall back to any available Gemini model.
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

func normalizeGeminiModel(raw string) string {
	model := strings.TrimSpace(raw)
	model = strings.TrimPrefix(model, "models/")
	model = strings.TrimPrefix(model, "/")
	return model
}
