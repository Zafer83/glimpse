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

	systemPrompt := fmt.Sprintf(`You are a Slidev presentation generator. You output ONLY valid Slidev Markdown.
Write all slide text in %s. Output nothing except the Slidev Markdown itself.`, cfg.Language)

	userPrompt := fmt.Sprintf(
		`Read the project documentation and source code below. Then generate a Slidev Markdown presentation.

YOU MUST produce between 8 and 15 slides. Each slide MUST be separated by --- on its own line.
Every slide MUST start with a # heading. Do NOT put all content on one slide.

The FIRST lines of your output must be exactly this (the global headmatter):

---
theme: %s
title: <your chosen title>
---

After the headmatter, write slides like this. THIS IS THE EXACT PATTERN TO REPEAT:

# Slide Title Here

- Bullet point or content
- Another point

---

# Next Slide Title

More content here.

---

SLIDE PLAN — you must create separate slides for each of these topics:
1. Cover/Title slide — project name, one-line description
2. Project Overview — what the project does, key features (from README/docs)
3. Tech Stack — languages, frameworks, dependencies used
4. Architecture — high-level component overview
5. Core Module 1 — explain the most important module with a code snippet
6. Core Module 2 — explain another key module with a code snippet
7. Data/Control Flow — include a mermaid diagram showing how components interact
8. API/Interface Design — how users or other systems interact with the project
9. Error Handling / Edge Cases — how the project handles failures
10. Key Takeaways — summary of main insights

For code snippets use fenced blocks with the language tag:

`+"```"+`go {2-4|6-8}
// actual code from the project, not placeholders
`+"```"+`

For diagrams use mermaid blocks:

`+"```"+`mermaid
graph TD
    A[Component] --> B[Component]
`+"```"+`

CRITICAL RULES:
- Output MUST contain at least 8 occurrences of --- (slide separators)
- Every slide MUST begin with # (a top-level heading)
- Do NOT write a single long text. SPLIT content across slides.
- Use REAL code from the provided source, not placeholder examples.
- Do NOT wrap output in a markdown code fence.
%s
PROJECT SOURCE:
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
