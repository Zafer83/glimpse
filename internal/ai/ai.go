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
	"fmt"
	"strings"

	"github.com/Zafer83/glimpse/internal/config"
)

// GenerateSlides routes the slide generation request to the appropriate
// LLM provider based on the model name in the config.
func GenerateSlides(cfg *config.Config, code string) (string, error) {
	codeForPrompt, truncNote := prepareCodeForModel(code, cfg.Model)

	systemPrompt := fmt.Sprintf(
		"You are Glimpse, an expert AI architect. Generate a professional Slidev "+
			"presentation from source code. Write all slide content in %s.", cfg.Language)

	userPrompt := fmt.Sprintf(
		`Create a Slidev Markdown presentation from the source code below.

CRITICAL FORMAT RULES — the output MUST follow this structure exactly:

1. The file MUST start with a YAML frontmatter block as the very first thing.
   This block sets the global theme and title:

---
theme: %s
title: <a short descriptive title>
---

2. Each subsequent slide is separated by a line containing only "---".
   Slides MAY have their own per-slide frontmatter for layout, background, or class:

---
layout: center
class: text-white
---

3. Valid layout values include: default, center, cover, two-cols, image-right, image-left, fact, statement, quote, section.

CONTENT GUIDELINES:
- Start with a cover slide (layout: cover) summarizing the project.
- Highlight core business logic and architectural patterns.
- Use fenced code blocks with Slidev focus markers, e.g. {1-5|7-10}.
- Include at least one Mermaid diagram (use a fenced mermaid block).
- End with a summary or key-takeaways slide.
- Aim for 8-15 slides total.

Return ONLY the raw Markdown content. No wrapping in a code fence. No explanation before or after.
%s
SOURCE CODE:
%s`,
		cfg.Theme, truncNote, codeForPrompt)

	// Provider routing is model-name based to keep the CLI model input simple.
	if isGeminiModel(cfg.Model) {
		return generateSlidesWithGemini(cfg, systemPrompt, userPrompt)
	}
	if isAnthropicModel(cfg.Model) {
		return generateSlidesWithAnthropic(cfg, systemPrompt, userPrompt)
	}
	if isLocalModel(cfg.Model) {
		return generateSlidesWithLocalLLM(cfg, systemPrompt, userPrompt)
	}

	return generateSlidesWithOpenAI(cfg, systemPrompt, userPrompt)
}

// RequiresAPIKey returns true if the model needs a remote API key.
func RequiresAPIKey(model string) bool {
	return !isLocalModel(model)
}

// --- Model detection helpers ---

func isGeminiModel(model string) bool {
	normalized := strings.ToLower(normalizeGeminiModel(model))
	return strings.HasPrefix(normalized, "gemini")
}

func isAnthropicModel(model string) bool {
	normalized := strings.ToLower(normalizeAnthropicModel(model))
	return strings.HasPrefix(normalized, "claude")
}

func isLocalModel(model string) bool {
	normalized := strings.ToLower(strings.TrimSpace(model))
	return normalized == "local" || normalized == "ollama" ||
		strings.HasPrefix(normalized, "local/") || strings.HasPrefix(normalized, "ollama/")
}

// --- Code truncation helpers ---

// prepareCodeForModel truncates the source code if it exceeds the model's
// prompt size limit and returns the (possibly truncated) code plus a note.
func prepareCodeForModel(code, model string) (string, string) {
	maxBytes := maxPromptBytesForModel(model)
	if len(code) <= maxBytes {
		return code, ""
	}

	truncated := truncateMiddleByBytes(code, maxBytes)
	note := fmt.Sprintf("- Note: Source code was truncated to %d bytes for request size limits.", maxBytes)
	return truncated, note
}

func maxPromptBytesForModel(model string) int {
	lower := strings.ToLower(strings.TrimSpace(model))
	switch {
	case strings.HasPrefix(lower, "claude"), strings.HasPrefix(lower, "anthropic/claude"):
		return 85000
	case strings.HasPrefix(lower, "gemini"), strings.HasPrefix(lower, "models/gemini"):
		return 140000
	default:
		return 180000
	}
}

func truncateMiddleByBytes(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	if maxBytes < 200 {
		return s[:maxBytes]
	}

	marker := "\n\n/* ... SOURCE TRUNCATED FOR REQUEST SIZE ... */\n\n"
	markerLen := len(marker)
	if markerLen >= maxBytes {
		return s[:maxBytes]
	}

	available := maxBytes - markerLen
	headLen := int(float64(available) * 0.65)
	tailLen := available - headLen
	if headLen < 0 {
		headLen = 0
	}
	if tailLen < 0 {
		tailLen = 0
	}

	head := s[:headLen]
	tail := s[len(s)-tailLen:]
	return head + marker + tail
}
