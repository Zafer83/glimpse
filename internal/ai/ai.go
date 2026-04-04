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
	"regexp"
	"strings"

	"github.com/Zafer83/glimpse/internal/config"
)

// GenerateSlides routes the slide generation request to the appropriate
// LLM provider based on the model name in the config.
func GenerateSlides(cfg *config.Config, code string) (string, error) {
	codeForPrompt, truncNote := prepareCodeForModel(code, cfg.Model)

	systemPrompt := fmt.Sprintf(`You generate Slidev presentations. Write all content in %s.
Output ONLY raw Slidev Markdown — no code fences around it, no explanation.`, cfg.Language)

	userPrompt := fmt.Sprintf(
		`Analyze the source code below and create a Slidev Markdown presentation.

EXACT FORMAT — follow this structure precisely:

---
theme: %s
title: <descriptive project title>
---

# Project Title

Short project summary.

---

# Architecture Overview

- Key architectural decisions
- Main components and their roles

---

# Core Logic

Explain the most important business logic with a code snippet:

`+"```"+`go {1-3|5-8}
func example() {
    // highlighted code
}
`+"```"+`

---

# System Flow

`+"```"+`mermaid
graph LR
  A[Input] --> B[Process] --> C[Output]
`+"```"+`

---
layout: center
---

# Key Takeaways

- Summary point 1
- Summary point 2

RULES:
1. Start with "---" then theme/title frontmatter, then "---".
2. Every slide begins with "# Title" as the first content line.
3. Separate slides with a blank line, then "---", then a blank line.
4. Per-slide frontmatter (layout, class) goes between "---" and the heading.
5. Use fenced code blocks with language tags and Slidev line highlights {1-3|5-7}.
6. Include at least one mermaid diagram.
7. Create 8-15 slides covering: overview, architecture, key modules, important logic, data flow, takeaways.
8. Each slide should focus on ONE topic with a clear heading.
%s
SOURCE CODE:
%s`,
		cfg.Theme, truncNote, codeForPrompt)

	// Provider routing is model-name based to keep the CLI model input simple.
	var slides string
	var err error
	if isGeminiModel(cfg.Model) {
		slides, err = generateSlidesWithGemini(cfg, systemPrompt, userPrompt)
	} else if isAnthropicModel(cfg.Model) {
		slides, err = generateSlidesWithAnthropic(cfg, systemPrompt, userPrompt)
	} else if isLocalModel(cfg.Model) {
		slides, err = generateSlidesWithLocalLLM(cfg, systemPrompt, userPrompt)
	} else {
		slides, err = generateSlidesWithOpenAI(cfg, systemPrompt, userPrompt)
	}
	if err != nil {
		return "", err
	}
	return normalizeSlidevMarkdown(slides, cfg), nil
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

var slidevFrontmatterRe = regexp.MustCompile(`(?s)\A---\s*\n(.*?)\n---\s*\n?`)

// normalizeSlidevMarkdown enforces a valid global Slidev frontmatter block
// and injects the selected theme reliably.
func normalizeSlidevMarkdown(raw string, cfg *config.Config) string {
	md := strings.TrimSpace(raw)
	body := md
	title := "Code Architecture Presentation"

	if m := slidevFrontmatterRe.FindStringSubmatch(md); len(m) == 2 {
		fm := m[1]
		body = strings.TrimSpace(strings.TrimPrefix(md, m[0]))
		for _, line := range strings.Split(fm, "\n") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) != 2 {
				continue
			}
			key := strings.TrimSpace(strings.ToLower(parts[0]))
			val := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
			if key == "title" && val != "" {
				title = val
			}
		}
	}

	frontmatter := fmt.Sprintf("---\ntheme: %s\ntitle: %s\n---\n\n", yamlQuote(cfg.Theme), yamlQuote(title))
	if body == "" {
		body = "# Presentation\n"
	}
	body = ensureTitleSlide(body, title)
	body = ensureSlideBreakPerTopLevelHeading(body)
	return frontmatter + body
}

func yamlQuote(v string) string {
	escaped := strings.ReplaceAll(strings.TrimSpace(v), "'", "''")
	return "'" + escaped + "'"
}

func ensureTitleSlide(body, title string) string {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return fmt.Sprintf("# %s\n", title)
	}
	for _, line := range strings.Split(trimmed, "\n") {
		l := strings.TrimSpace(line)
		if l == "" {
			continue
		}
		if strings.HasPrefix(l, "# ") {
			return trimmed
		}
		break
	}
	return fmt.Sprintf("# %s\n\n%s", title, trimmed)
}

func ensureSlideBreakPerTopLevelHeading(body string) string {
	lines := strings.Split(body, "\n")
	var out []string
	inFence := false
	firstTopLevelSeen := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			out = append(out, line)
			continue
		}

		if !inFence && strings.HasPrefix(trimmed, "# ") {
			if !firstTopLevelSeen {
				firstTopLevelSeen = true
			} else {
				// Ensure a clean slide separator before each subsequent top-level heading.
				lastIdx := len(out) - 1
				for lastIdx >= 0 && strings.TrimSpace(out[lastIdx]) == "" {
					lastIdx--
				}
				if lastIdx < 0 || strings.TrimSpace(out[lastIdx]) != "---" {
					out = append(out, "", "---", "")
				}
			}
		}

		// Avoid adding an extra trailing blank line for the final input line.
		if i == len(lines)-1 && line == "" {
			continue
		}
		out = append(out, line)
	}

	return strings.TrimSpace(strings.Join(out, "\n")) + "\n"
}
