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
	_ "embed"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/Zafer83/glimpse/internal/config"
	"github.com/Zafer83/glimpse/internal/crawler"
)

//go:embed prompts/system.txt
var systemPromptTpl string

//go:embed prompts/local_user.txt
var localUserPromptTpl string

//go:embed prompts/cloud_user.txt
var cloudUserPromptTpl string

// renderPrompt replaces {{key}} placeholders in a template string.
func renderPrompt(tpl string, vars map[string]string) string {
	result := tpl
	for k, v := range vars {
		result = strings.ReplaceAll(result, "{{"+k+"}}", v)
	}
	return strings.TrimRight(result, "\n")
}

// GenerateSlides routes the slide generation request to the appropriate
// LLM provider based on the model name in the config.
// Accepts either *crawler.CollectedContent (structured) or a flat code string.
func GenerateSlides(cfg *config.Config, content *crawler.CollectedContent) (string, error) {
	codeForPrompt, truncNote := assembleContentForModel(content, cfg.Model)

	systemPrompt := renderPrompt(systemPromptTpl, map[string]string{
		"language": cfg.Language,
	})

	promptVars := map[string]string{
		"theme":     cfg.Theme,
		"truncNote": truncNote,
		"code":      codeForPrompt,
	}
	var userPrompt string
	if isLocalModel(cfg.Model) {
		userPrompt = renderPrompt(localUserPromptTpl, promptVars)
	} else {
		userPrompt = renderPrompt(cloudUserPromptTpl, promptVars)
	}

	// callProvider dispatches to the correct AI backend.
	callProvider := func() (string, error) {
		if isGeminiModel(cfg.Model) {
			return generateSlidesWithGemini(cfg, systemPrompt, userPrompt)
		} else if isAnthropicModel(cfg.Model) {
			return generateSlidesWithAnthropic(cfg, systemPrompt, userPrompt)
		} else if isLocalModel(cfg.Model) {
			return generateSlidesWithLocalLLM(cfg, systemPrompt, userPrompt)
		}
		return generateSlidesWithOpenAI(cfg, systemPrompt, userPrompt)
	}

	// Retry transient errors (timeouts, 429, 5xx) with exponential backoff.
	var slides string
	var lastErr error
	for attempt := 0; attempt < DefaultRetryConfig.MaxAttempts; attempt++ {
		if attempt > 0 {
			delay := backoffDelay(DefaultRetryConfig, attempt)
			fmt.Printf("  Retry %d/%d after error, waiting %v...\n", attempt, DefaultRetryConfig.MaxAttempts-1, delay.Round(time.Millisecond))
			time.Sleep(delay)
		}
		slides, lastErr = callProvider()
		if lastErr == nil {
			break
		}
		if !isRetryable(lastErr) {
			return "", lastErr
		}
	}
	if lastErr != nil {
		return "", fmt.Errorf("failed after %d attempts: %w", DefaultRetryConfig.MaxAttempts, lastErr)
	}

	normalized := normalizeSlidevMarkdown(slides, cfg)

	// Validate that the output looks like a real multi-slide presentation.
	// If the model returned prose instead of Slidev Markdown (common with small
	// local models), warn the user rather than silently writing a broken file.
	if isLocalModel(cfg.Model) {
		if err := validateSlidevOutput(normalized); err != nil {
			return "", err
		}
	}

	return normalized, nil
}

// validateSlidevOutput returns an error when the output does not contain enough
// slide separators to be a valid multi-slide presentation.
func validateSlidevOutput(md string) error {
	count := strings.Count(md, "\n---\n")
	if count < 3 {
		return fmt.Errorf(
			"the local model returned plain text instead of Slidev slides (found %d slide separators, need at least 3).\n"+
				"  Tip: use a larger model that follows instructions better, e.g.:\n"+
				"    AI Model: local/qwen2.5-coder:7b\n"+
				"    AI Model: local/qwen2.5-coder:14b\n"+
				"    AI Model: local/mistral\n"+
				"  Then run: ollama pull qwen2.5-coder:7b",
			count,
		)
	}
	return nil
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
	case isLocalModel(lower):
		// Local models typically have 8K-32K context windows.
		// Instructions MUST stay visible inside the attention window.
		// 12KB of code + ~2KB of prompt ≈ 14KB total (~3500 tokens).
		return 12000
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

// Budget allocation weights for content tiers.
const (
	docBudgetPct      = 0.60 // 60% for documentation
	businessBudgetPct = 0.35 // 35% for business logic
	supportBudgetPct  = 0.05 // 5% for support code
)

// assembleContentForModel builds a budget-weighted string from structured content.
// Documentation gets the highest byte budget, business logic second, support third.
// Unused budget from one tier flows to the next.
func assembleContentForModel(content *crawler.CollectedContent, model string) (string, string) {
	maxBytes := maxPromptBytesForModel(model)

	docBudget := int(float64(maxBytes) * docBudgetPct)
	bizBudget := int(float64(maxBytes) * businessBudgetPct)
	supBudget := int(float64(maxBytes) * supportBudgetPct)

	// Assemble each tier.
	docStr := assembleTier(content.Docs, "DOC", "#")
	bizStr := assembleTier(content.Business, "CODE", "//")
	supStr := assembleTier(content.Support, "SUPPORT", "//")

	// Redistribute unused budget downward: doc → business → support.
	docUsed := min(len(docStr), docBudget)
	leftover := docBudget - docUsed
	bizBudget += leftover

	bizUsed := min(len(bizStr), bizBudget)
	leftover = bizBudget - bizUsed
	supBudget += leftover

	// Truncate each tier to its budget.
	if len(docStr) > docBudget {
		docStr = truncateMiddleByBytes(docStr, docBudget)
	}
	if len(bizStr) > bizBudget {
		bizStr = truncateMiddleByBytes(bizStr, bizBudget)
	}
	if len(supStr) > supBudget {
		supStr = truncateMiddleByBytes(supStr, supBudget)
	}

	var b strings.Builder
	b.WriteString("# PROJECT CONTEXT ORDER: DOCUMENTATION FIRST, THEN CORE CODE\n")

	if docStr != "" {
		b.WriteString("\n# === SECTION: PROJECT DOCUMENTATION ===\n")
		b.WriteString("# Use this section to understand the project's purpose, architecture, and goals.\n\n")
		b.WriteString(docStr)
	}
	if bizStr != "" {
		b.WriteString("\n\n# === SECTION: CORE BUSINESS LOGIC ===\n")
		b.WriteString("# The most architecturally relevant source code.\n\n")
		b.WriteString(bizStr)
	}
	if supStr != "" {
		b.WriteString("\n\n# === SECTION: SUPPORTING CODE ===\n")
		b.WriteString("# Tests, configs, and utilities — for additional context only.\n\n")
		b.WriteString(supStr)
	}

	result := b.String()
	totalBytes := len(result)
	var note string
	if totalBytes > maxBytes {
		result = truncateMiddleByBytes(result, maxBytes)
		note = fmt.Sprintf("- Note: Project content was truncated to %d bytes for model limits.", maxBytes)
	}

	docs, biz, sup := content.Stats()
	if docs+biz+sup > 0 {
		truncated := totalBytes > maxBytes
		if truncated {
			note = fmt.Sprintf("- Note: Project content was truncated to %d bytes (%d docs, %d code, %d support files).",
				maxBytes, docs, biz, sup)
		}
	}

	return result, note
}

// assembleTier concatenates file entries into a single string with headers.
func assembleTier(entries []crawler.FileEntry, label, commentPrefix string) string {
	if len(entries) == 0 {
		return ""
	}
	var b strings.Builder
	for _, e := range entries {
		b.WriteString(fmt.Sprintf("\n%s --- %s FILE: %s ---\n%s\n", commentPrefix, label, e.Path, e.Content))
	}
	return b.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

var slidevFrontmatterRe = regexp.MustCompile(`(?s)\A---\s*\n(.*?)\n---\s*\n?`)

// normalizeSlidevMarkdown enforces a valid global Slidev frontmatter block,
// injects the selected theme with layout: cover, and preserves visual fields
// (background, author) that the AI generated.
func normalizeSlidevMarkdown(raw string, cfg *config.Config) string {
	md := strings.TrimSpace(raw)
	body := md
	title := "Code Architecture Presentation"
	background := ""
	author := ""
	layout := "cover"

	if m := slidevFrontmatterRe.FindStringSubmatch(md); len(m) == 2 {
		fm := m[1]
		body = strings.TrimSpace(strings.TrimPrefix(md, m[0]))
		for _, line := range strings.Split(fm, "\n") {
			// SplitN(..., 2) keeps the full value even when it contains colons (e.g. URLs).
			parts := strings.SplitN(line, ":", 2)
			if len(parts) != 2 {
				continue
			}
			key := strings.TrimSpace(strings.ToLower(parts[0]))
			val := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
			switch key {
			case "title":
				if val != "" {
					title = val
				}
			case "background":
				if val != "" {
					background = strings.TrimSpace(parts[1])
				}
			case "author":
				if val != "" {
					author = val
				}
			case "layout":
				if val != "" {
					layout = val
				}
			}
		}
	}

	var fmParts []string
	fmParts = append(fmParts, "theme: "+yamlQuote(cfg.Theme))
	fmParts = append(fmParts, "layout: "+layout)
	fmParts = append(fmParts, "title: "+yamlQuote(title))
	if author != "" {
		fmParts = append(fmParts, "author: "+yamlQuote(author))
	}
	if background != "" {
		fmParts = append(fmParts, "background: "+background)
	}
	frontmatter := "---\n" + strings.Join(fmParts, "\n") + "\n---\n\n"

	if body == "" {
		body = "# Presentation\n"
	}
	body = ensureTitleSlide(body, title)
	body = ensureSlideBreakPerTopLevelHeading(body)

	// Remove any mermaid code blocks — Slidev renders them with errors.
	body = removeMermaidBlocks(body)

	return frontmatter + body
}

// mermaidBlockRe matches ```mermaid ... ``` blocks.
var mermaidBlockRe = regexp.MustCompile("(?s)```mermaid\\s*\n.*?```")

// removeMermaidBlocks strips mermaid code blocks from the output since Slidev
// cannot reliably render them without additional plugins.
func removeMermaidBlocks(body string) string {
	return mermaidBlockRe.ReplaceAllString(body, "")
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
