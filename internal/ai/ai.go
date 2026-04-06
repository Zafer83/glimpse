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

	// Extract a structured outline from the project docs. This drives the
	// dynamic slide plan: project-specific topics instead of generic placeholders.
	outline := extractDocOutline(content.Docs)
	var projectName string
	projectName = outline.ProjectName
	if projectName == "" {
		projectName = "Project"
	}
	projectDesc := outline.Description
	if projectDesc == "" {
		projectDesc = "A software project"
	}

	promptVars := map[string]string{
		"theme":     cfg.Theme,
		"truncNote": truncNote,
		"code":      codeForPrompt,
	}
	var userPrompt string
	if isLocalModel(cfg.Model) {
		slidePlan := buildSlidePlan(outline, cfg.Language, true)
		promptVars["slidePlan"] = slidePlan
		promptVars["projectName"] = projectName
		promptVars["projectDescription"] = projectDesc
		userPrompt = renderPrompt(localUserPromptTpl, promptVars)
	} else {
		slidePlan := buildSlidePlan(outline, cfg.Language, false)
		promptVars["slidePlan"] = slidePlan
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

	normalized := normalizeSlidevMarkdown(slides, cfg, projectName)

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

// IsLocalModel is the exported version of isLocalModel for use by cmd/.
func IsLocalModel(model string) bool {
	return isLocalModel(model)
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
		// Modern local models (Qwen 2.5, LLaMA 3.2, Mistral) typically have
		// 32K-128K context windows. 80KB of content + ~2KB of prompt ≈ 82KB
		// total (~20K tokens) — comfortably within the attention range of 7B+
		// models. Rich docs/ folders often contain 100KB+ of markdown; a larger
		// budget allows the greedy docs-fill to include several complete files.
		return 80000
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
// Documentation gets the lion's share because the presentation narrative
// should be driven by project docs, not by code snippets.
const (
	docBudgetPct      = 0.70 // 70% for documentation
	businessBudgetPct = 0.25 // 25% for business logic
	supportBudgetPct  = 0.05 // 5% for support code
)

// assembleContentForModel builds a budget-weighted string from structured content.
// Documentation gets the highest byte budget, business logic second, support third.
// Unused budget from one tier flows to the next.
func assembleContentForModel(content *crawler.CollectedContent, model string) (string, string) {
	maxBytes := maxPromptBytesForModel(model)
	local := isLocalModel(model)

	docBudget := int(float64(maxBytes) * docBudgetPct)
	bizBudget := int(float64(maxBytes) * businessBudgetPct)
	supBudget := int(float64(maxBytes) * supportBudgetPct)

	// Measure raw tier sizes to redistribute unused budget downward.
	docRaw := rawTierSize(content.Docs, "DOC", "#")
	bizRaw := rawTierSize(content.Business, "CODE", "//")

	// Redistribute unused budget downward: doc → business → support.
	docUsed := min(docRaw, docBudget)
	leftover := docBudget - docUsed
	bizBudget += leftover

	bizUsed := min(bizRaw, bizBudget)
	leftover = bizBudget - bizUsed
	supBudget += leftover

	// Docs: greedy fill with compaction for local models. Local models get
	// compacted docs (headings + bullets + tables + first sentences) so they
	// see dense, presentation-ready content. Cloud models get full docs since
	// they have the reasoning capacity to summarize on their own.
	docStr := assembleDocsGreedy(content.Docs, "DOC", "#", docBudget, local)
	// Code tiers: proportional distribution — every file contributes a snippet
	// so the AI sees the full breadth of the codebase.
	bizStr := assembleTierWithBudget(content.Business, "CODE", "//", bizBudget)
	supStr := assembleTierWithBudget(content.Support, "SUPPORT", "//", supBudget)

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

// rawTierSize returns the total byte size that assembleTier would produce
// for the given entries, without allocating the full string.
func rawTierSize(entries []crawler.FileEntry, label, commentPrefix string) int {
	if len(entries) == 0 {
		return 0
	}
	total := 0
	for _, e := range entries {
		hdr := fmt.Sprintf("\n%s --- %s FILE: %s ---\n", commentPrefix, label, e.Path)
		total += len(hdr) + len(e.Content) + 1 // +1 for trailing \n
	}
	return total
}

// compactDocContent extracts only structurally meaningful elements from a
// documentation file, discarding verbose prose paragraphs that small LLMs
// tend to copy verbatim. The result is a much denser representation that
// preserves headings, bullet points, tables, and first sentences of paragraphs
// while being 3-5× smaller than the raw content.
//
// What is kept:
//   - All headings (#, ##, ###, etc.)
//   - All bullet point / list items (-, *, numbered)
//   - All table rows (lines with |)
//   - Block quote lines (>)
//   - Code block opening lines (first 3 lines only, rest replaced with ...)
//   - First sentence of each prose paragraph (non-structural text)
//   - Blank lines between sections (for readability)
//
// What is discarded:
//   - 2nd+ sentences of prose paragraphs
//   - Large code blocks (beyond 3 lines)
//   - Table-of-contents link lists
func compactDocContent(content string) string {
	lines := strings.Split(content, "\n")
	var out []string
	inCodeBlock := false
	codeBlockLines := 0
	inProseParagraph := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Code block toggle.
		if strings.HasPrefix(trimmed, "```") {
			if !inCodeBlock {
				inCodeBlock = true
				codeBlockLines = 0
				out = append(out, line)
			} else {
				inCodeBlock = false
				out = append(out, line)
			}
			inProseParagraph = false
			continue
		}

		// Inside code block: keep first 3 lines, then add "..." and skip.
		if inCodeBlock {
			codeBlockLines++
			if codeBlockLines <= 3 {
				out = append(out, line)
			} else if codeBlockLines == 4 {
				out = append(out, "  ...")
			}
			continue
		}

		// Blank line: keep it as section separator, reset prose state.
		if trimmed == "" {
			inProseParagraph = false
			out = append(out, "")
			continue
		}

		// Headings: always keep.
		if strings.HasPrefix(trimmed, "#") {
			inProseParagraph = false
			out = append(out, line)
			continue
		}

		// Horizontal rules / separators: keep.
		if trimmed == "---" || trimmed == "***" || trimmed == "___" {
			inProseParagraph = false
			out = append(out, line)
			continue
		}

		// Bullet points / list items: always keep.
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") ||
			strings.HasPrefix(trimmed, "+ ") || (len(trimmed) > 2 && trimmed[0] >= '0' && trimmed[0] <= '9' && strings.Contains(trimmed[:3], ".")) {
			inProseParagraph = false
			out = append(out, line)
			continue
		}

		// Table rows: always keep.
		if strings.HasPrefix(trimmed, "|") || (strings.Contains(trimmed, "|") && strings.Contains(trimmed, "---")) {
			inProseParagraph = false
			out = append(out, line)
			continue
		}

		// Block quotes: always keep.
		if strings.HasPrefix(trimmed, ">") {
			inProseParagraph = false
			out = append(out, line)
			continue
		}

		// Skip TOC-style links: lines that are only [text](#anchor).
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, ")") && strings.Contains(trimmed, "](#") {
			continue
		}

		// Prose paragraph: keep only the first sentence (first line).
		if !inProseParagraph {
			inProseParagraph = true
			// Extract first sentence: up to first ". " or end of line.
			if idx := strings.Index(trimmed, ". "); idx > 0 && idx < 200 {
				out = append(out, trimmed[:idx+1])
			} else {
				out = append(out, trimmed)
			}
		}
		// Subsequent lines of prose paragraph: skip.
	}

	return strings.Join(out, "\n")
}

// assembleDocsGreedy fills the docs budget by including compacted high-priority
// documents first (entries are already sorted by docSortPriority). Each file's
// content is pre-processed by compactDocContent to extract only structural
// elements (headings, bullets, tables, first sentences). This dramatically
// reduces the byte footprint so MORE files fit in the budget, AND the model
// receives dense, presentation-ready content instead of verbose prose.
func assembleDocsGreedy(entries []crawler.FileEntry, label, commentPrefix string, budget int, compact bool) string {
	if len(entries) == 0 || budget <= 0 {
		return ""
	}

	// Minimum bytes needed to include a file's header plus a meaningful excerpt.
	const minUsefulBytes = 400

	var b strings.Builder
	remaining := budget

	for _, e := range entries {
		hdr := fmt.Sprintf("\n%s --- %s FILE: %s ---\n", commentPrefix, label, e.Path)
		overhead := len(hdr) + 1 // +1 for trailing newline

		if remaining < overhead+minUsefulBytes {
			break
		}

		content := e.Content
		if compact {
			content = compactDocContent(content)
		}

		b.WriteString(hdr)
		maxContent := remaining - overhead
		if len(content) <= maxContent {
			b.WriteString(content)
			remaining -= overhead + len(content)
		} else {
			b.WriteString(content[:maxContent])
			remaining -= overhead + maxContent
		}
		b.WriteString("\n")

		if remaining < minUsefulBytes {
			break
		}
	}

	return b.String()
}

// assembleTierWithBudget concatenates file entries and distributes the byte
// budget across all files. Each file gets a fair share of the budget so that
// no single large file crowds out the others and every file is represented
// (albeit possibly truncated) in the final context.
func assembleTierWithBudget(entries []crawler.FileEntry, label, commentPrefix string, budget int) string {
	if len(entries) == 0 || budget <= 0 {
		return ""
	}

	// Measure raw sizes (content + header overhead per file).
	type sized struct {
		idx     int
		raw     int // bytes including header
		header  string
		content string
	}
	items := make([]sized, len(entries))
	totalRaw := 0
	for i, e := range entries {
		hdr := fmt.Sprintf("\n%s --- %s FILE: %s ---\n", commentPrefix, label, e.Path)
		items[i] = sized{idx: i, raw: len(hdr) + len(e.Content) + 1, header: hdr, content: e.Content}
		totalRaw += items[i].raw
	}

	// If everything fits, no truncation needed.
	if totalRaw <= budget {
		var b strings.Builder
		for _, it := range items {
			b.WriteString(it.header)
			b.WriteString(it.content)
			b.WriteString("\n")
		}
		return b.String()
	}

	// Distribute budget proportionally across files. Each file gets at least
	// a minimum slice (header + 200 bytes) so small files aren't starved.
	minPerFile := 300
	remaining := budget
	alloc := make([]int, len(items))
	for i, it := range items {
		share := int(float64(budget) * float64(it.raw) / float64(totalRaw))
		if share < minPerFile && it.raw > minPerFile {
			share = minPerFile
		}
		if share > it.raw {
			share = it.raw
		}
		alloc[i] = share
		remaining -= share
	}

	// Give leftover bytes to the first files (highest priority / sorted first).
	for i := range alloc {
		if remaining <= 0 {
			break
		}
		room := items[i].raw - alloc[i]
		if room <= 0 {
			continue
		}
		give := min(room, remaining)
		alloc[i] += give
		remaining -= give
	}

	var b strings.Builder
	for i, it := range items {
		b.WriteString(it.header)
		maxContent := alloc[i] - len(it.header) - 1
		if maxContent <= 0 {
			maxContent = 0
		}
		if len(it.content) > maxContent && maxContent > 0 {
			b.WriteString(truncateMiddleByBytes(it.content, maxContent))
		} else {
			b.WriteString(it.content)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// extractProjectFacts builds a compact project summary from the top documentation
// entries. It is placed at the end of the local-model prompt (right before
// BEGIN OUTPUT NOW) to combat recency bias in small LLMs: the model receives
// the key project facts fresh in its context window when it starts generating.
//
// Returns the facts block and the detected project name (from the first # heading
// in the highest-priority doc — usually the README).
func extractProjectFacts(docs []crawler.FileEntry) (facts string, projectName string) {
	if len(docs) == 0 {
		return "", ""
	}

	const (
		maxBytesPerDoc = 2000 // enough for title + intro + first feature list
		maxDocs        = 3    // README + 2 more
	)

	var b strings.Builder
	b.WriteString("KEY PROJECT FACTS (extracted from documentation — use these for your slides):\n\n")

	count := 0
	for _, e := range docs {
		if count >= maxDocs {
			break
		}
		content := e.Content

		// Extract project name from the very first # heading in the first doc.
		if projectName == "" {
			for _, line := range strings.Split(content, "\n") {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "# ") {
					// Strip markdown formatting: *, **, emoji, badges.
					raw := strings.TrimPrefix(trimmed, "# ")
					raw = strings.TrimSpace(raw)
					// Remove leading emoji/symbols (non-ASCII until first letter/digit).
					for len(raw) > 0 {
						r := rune(raw[0])
						if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
							break
						}
						// Multi-byte rune: skip by finding next ASCII char.
						if raw[0] > 127 {
							i := 1
							for i < len(raw) && raw[i] > 127 {
								i++
							}
							raw = strings.TrimSpace(raw[i:])
						} else {
							raw = strings.TrimSpace(raw[1:])
						}
					}
					if raw != "" {
						projectName = raw
					}
					break
				}
			}
		}

		if len(content) > maxBytesPerDoc {
			// Keep only the leading portion — title, description, key bullet points.
			content = content[:maxBytesPerDoc]
			// Trim to last newline so we don't cut mid-word.
			if idx := strings.LastIndexByte(content, '\n'); idx > maxBytesPerDoc/2 {
				content = content[:idx]
			}
		}
		b.WriteString(fmt.Sprintf("[DOCUMENT: %s]\n%s\n\n", e.Path, strings.TrimSpace(content)))
		count++
	}

	return b.String(), projectName
}

var slidevFrontmatterRe = regexp.MustCompile(`(?s)\A---\s*\n(.*?)\n---\s*\n?`)

// yamlLineRe matches a YAML key-value line: "key: value" or "key:".
var yamlLineRe = regexp.MustCompile(`^\w[\w\-]*\s*:`)

// normalizeSlidevMarkdown enforces a valid global Slidev frontmatter block,
// injects the selected theme with layout: cover, and preserves visual fields
// (background, author) that the AI generated.
// hintTitle is an optional project name extracted from documentation; it is
// used when the model failed to provide a meaningful title in its output.
func normalizeSlidevMarkdown(raw string, cfg *config.Config, hintTitle string) string {
	md := strings.TrimSpace(raw)
	body := md
	// Use the hint title as default so small models that omit the frontmatter
	// still get the correct project name instead of the generic fallback.
	defaultTitle := "Code Architecture Presentation"
	if hintTitle != "" {
		defaultTitle = hintTitle
	}
	title := defaultTitle
	background := ""
	author := ""

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
			}
		}
	}

	// If the LLM output a generic placeholder title, override it with the
	// hint extracted from the documentation (the real project name).
	if hintTitle != "" && (title == "Code Architecture Presentation" || title == defaultTitle) {
		title = hintTitle
	}

	// Ensure cover slide always has a background image.
	if background == "" {
		background = defaultCoverBackground
	}

	// The first slide is always a cover — enforce layout: cover regardless of
	// what the model generated to guarantee the welcome page renders correctly.
	var fmParts []string
	fmParts = append(fmParts, "theme: "+yamlQuote(cfg.Theme))
	fmParts = append(fmParts, "layout: cover")
	fmParts = append(fmParts, "title: "+yamlQuote(title))
	if author != "" {
		fmParts = append(fmParts, "author: "+yamlQuote(author))
	}
	fmParts = append(fmParts, "background: "+background)
	frontmatter := "---\n" + strings.Join(fmParts, "\n") + "\n---\n\n"

	if body == "" {
		body = "# Presentation\n"
	}
	// Promote ## headings to # when they appear as the first heading after a
	// slide separator. Small models often output ## instead of # for slide titles,
	// which makes Slidev render them as body text instead of slide headings.
	body = promoteSlideHeadings(body)

	body = ensureTitleSlide(body, title)
	body = ensureSlideBreakPerTopLevelHeading(body)

	// Remove any mermaid code blocks — Slidev renders them with errors.
	body = removeMermaidBlocks(body)

	// Remove [DOCUMENT: ...] lines that can leak into the output when the model
	// echoes back the "project facts" block that was injected into the prompt.
	body = removeDocumentTagLines(body)

	// Fix malformed per-slide frontmatter: remove blank lines between --- and
	// YAML keys so Slidev parses them correctly.
	body = fixSlideFrontmatter(body)

	// Resolve keyword-style image: values to proper Unsplash CDN URLs.
	// Uses curated photo IDs with layout-aware crops (landscape for covers,
	// portrait for image-right/left).
	body = resolveAllImageKeywords(body)

	// Ensure the presentation ends with a Thank You outro slide.
	body = ensureOutroSlide(body, cfg.Language)

	return frontmatter + body
}

// outroHeadings are # headings that indicate a dedicated outro/thank-you slide.
// Only heading-level matches count — phrases like "Für Fragen..." in body text
// must not trigger a false positive.
var outroHeadings = []string{
	"# vielen dank", "# danke", "# danke schön",
	"# thank you", "# thanks",
	"# merci", "# gracias", "# grazie",
}

// ensureOutroSlide appends a Thank You cover slide if the body does not already
// end with a dedicated outro slide (detected by heading keywords).
func ensureOutroSlide(body, lang string) string {
	lower := strings.ToLower(body)

	// Check the LAST slide only (content after the last --- separator).
	lastSep := strings.LastIndex(lower, "\n---")
	lastSlide := lower
	if lastSep >= 0 {
		lastSlide = lower[lastSep:]
	}

	for _, kw := range outroHeadings {
		if strings.Contains(lastSlide, kw) {
			return body
		}
	}

	// No dedicated outro slide found — append one.
	outro := "\n\n---\n" +
		"layout: cover\n" +
		"background: https://images.unsplash.com/photo-1516116216624-53e697fedbea?auto=format&fit=crop&w=1600&h=900&q=80\n" +
		"---\n\n"
	switch strings.ToLower(lang) {
	case "de":
		outro += "# Vielen Dank!\n\nFragen? Gerne jetzt!\n"
	case "fr":
		outro += "# Merci!\n\nQuestions?\n"
	case "es":
		outro += "# ¡Gracias!\n\n¿Preguntas?\n"
	case "it":
		outro += "# Grazie!\n\nDomande?\n"
	default:
		outro += "# Thank You!\n\nQuestions?\n"
	}
	return strings.TrimRight(body, "\n") + outro
}

// defaultCoverBackground is the fallback Unsplash image for the cover slide.
// Uses landscape crop with dark code aesthetic for good text readability.
const defaultCoverBackground = "https://images.unsplash.com/photo-1555066931-4365d14bab8c?auto=format&fit=crop&w=1600&h=900&q=80"

// emptySlideRe matches two --- markers separated only by blank lines (empty slides).
var emptySlideRe = regexp.MustCompile(`(?m)^---\s*\n(\s*\n)*---\s*$`)

// fixSlideFrontmatter walks the body line-by-line to clean up per-slide
// frontmatter blocks. A --- followed (after optional blanks) by a YAML key
// starts a frontmatter block; a --- followed by non-YAML content is just a
// slide separator.
//
// Fixes applied:
//   - Removes blank lines inside frontmatter blocks.
//   - Moves non-YAML content (bullets, prose, code) that leaked into
//     frontmatter to the slide body after the closing ---.
//   - Replaces unsupported layout: end with layout: cover.
//   - Quotes YAML values starting with YAML-special characters (*, &, [, {).
//   - Collapses consecutive --- markers (empty slides) into a single ---.
func fixSlideFrontmatter(body string) string {
	lines := strings.Split(body, "\n")
	var out []string
	i := 0

	for i < len(lines) {
		trimmed := strings.TrimSpace(lines[i])

		// Not a --- marker — pass through.
		if trimmed != "---" {
			out = append(out, lines[i])
			i++
			continue
		}

		// We see ---. Is this a frontmatter start or just a slide separator?
		// Skip blank lines after --- and check the first non-blank line.
		j := i + 1
		for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
			j++
		}

		// If the next non-blank line is a YAML key this is a frontmatter block.
		if j < len(lines) && yamlLineRe.MatchString(strings.TrimSpace(lines[j])) {
			var yamlLines []string
			var spillLines []string
			inSpill := false

			k := i + 1 // scan from line after opening ---
			for k < len(lines) {
				t := strings.TrimSpace(lines[k])
				if t == "---" {
					break // closing --- (or next separator when inSpill)
				}
				if inSpill {
					spillLines = append(spillLines, lines[k])
				} else if yamlLineRe.MatchString(t) {
					yamlLines = append(yamlLines, fixYAMLLine(lines[k]))
				} else if t != "" {
					// Non-YAML, non-blank: slide content leaked into frontmatter.
					inSpill = true
					spillLines = append(spillLines, lines[k])
				}
				// blank lines: silently dropped from frontmatter
				k++
			}

			// Write cleaned frontmatter.
			out = append(out, "---")
			out = append(out, yamlLines...)
			out = append(out, "---")

			// Write any spilled content after the frontmatter.
			for len(spillLines) > 0 && strings.TrimSpace(spillLines[len(spillLines)-1]) == "" {
				spillLines = spillLines[:len(spillLines)-1]
			}
			if len(spillLines) > 0 {
				out = append(out, "")
				out = append(out, spillLines...)
			}

			// Advance past the closing ---.
			if k < len(lines) && strings.TrimSpace(lines[k]) == "---" {
				i = k + 1
			} else {
				i = k
			}
		} else {
			// No YAML after ---: just a slide separator.
			out = append(out, lines[i])
			i++
		}
	}

	// Collapse consecutive --- markers (empty slides) into one.
	result := strings.Join(out, "\n")
	for {
		cleaned := emptySlideRe.ReplaceAllString(result, "---")
		if cleaned == result {
			break
		}
		result = cleaned
	}
	return result
}

// fixYAMLLine normalises a single YAML key-value line:
//   - Replaces layout: end with layout: cover (end is not a valid Slidev layout).
//   - Quotes values that start with YAML-special characters to prevent alias/
//     anchor parse errors (e.g. a title like "*italic*" or "**bold**").
func fixYAMLLine(line string) string {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return line
	}
	key := strings.TrimSpace(strings.ToLower(parts[0]))
	rawVal := parts[1] // preserve original spacing for URL values

	val := strings.TrimSpace(rawVal)
	unquoted := strings.Trim(val, `"'`)

	// Replace unsupported layout: end.
	if key == "layout" && strings.ToLower(unquoted) == "end" {
		return parts[0] + ": cover"
	}

	// Quote values starting with YAML-special characters that could be
	// misinterpreted as anchors, aliases, or block scalars.
	special := len(val) > 0 && (val[0] == '*' || val[0] == '&' || val[0] == '!' ||
		val[0] == '[' || val[0] == '{' || val[0] == '|' || val[0] == '>')
	alreadyQuoted := len(val) >= 2 &&
		((val[0] == '\'' && val[len(val)-1] == '\'') ||
			(val[0] == '"' && val[len(val)-1] == '"'))

	if special && !alreadyQuoted {
		escaped := strings.ReplaceAll(unquoted, "'", "''")
		return parts[0] + ": '" + escaped + "'"
	}
	return line
}

// resolveImageKeywords replaces keyword-style image: values (not starting with
// "http") with a real URL built from the configured template.
// The template uses {keywords} as placeholder.
// Only image: keys inside per-slide frontmatter blocks (between --- markers)
// are processed; the global frontmatter background: is left untouched.
//
// Examples:
//
//	image: law,contract   → https://source.unsplash.com/1920x1080/?law,contract
//	image: code           → https://source.unsplash.com/1920x1080/?code
//	image: https://...    → unchanged (already a URL)
func resolveImageKeywords(body, urlTemplate string) string {
	if urlTemplate == "" {
		return body
	}
	lines := strings.Split(body, "\n")
	inFrontmatter := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			inFrontmatter = !inFrontmatter
			continue
		}
		if !inFrontmatter {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(strings.ToLower(parts[0]))
		if key != "image" {
			continue
		}
		val := strings.TrimSpace(parts[1])
		// Strip surrounding quotes if present.
		val = strings.Trim(val, `"'`)
		if strings.HasPrefix(val, "http") {
			// Already a URL — leave it alone.
			continue
		}
		if val == "" {
			continue
		}
		// Build URL by replacing {keywords} with the trimmed keyword string.
		resolved := strings.ReplaceAll(urlTemplate, "{keywords}", strings.TrimSpace(val))
		lines[i] = parts[0] + ": " + resolved
	}
	return strings.Join(lines, "\n")
}

// documentTagRe matches [DOCUMENT: path] lines injected into the prompt facts
// block that the model may echo back verbatim into the slide output.
var documentTagRe = regexp.MustCompile(`(?m)^\[DOCUMENT:[^\]]*\]\s*\n?`)

// removeDocumentTagLines strips [DOCUMENT: ...] lines from the slide body.
func removeDocumentTagLines(body string) string {
	return documentTagRe.ReplaceAllString(body, "")
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

// promoteSlideHeadings converts ## headings to # when they are the first
// heading after a slide separator (--- or start of body). Small local models
// often output ## for slide titles, but Slidev needs # for the main heading
// per slide to render correctly.
func promoteSlideHeadings(body string) string {
	lines := strings.Split(body, "\n")
	var out []string
	inFence := false
	expectHeading := true // true at start and after ---

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			out = append(out, line)
			continue
		}

		if inFence {
			out = append(out, line)
			continue
		}

		if trimmed == "---" {
			expectHeading = true
			out = append(out, line)
			continue
		}

		// Skip blank lines — don't reset expectHeading.
		if trimmed == "" {
			out = append(out, line)
			continue
		}

		// YAML-like line in frontmatter — skip, don't reset.
		if expectHeading && yamlLineRe.MatchString(trimmed) {
			out = append(out, line)
			continue
		}

		// First content line after --- (or start): if it's ##+ promote to #.
		if expectHeading && strings.HasPrefix(trimmed, "## ") {
			// Promote: remove one # level.
			out = append(out, "#"+strings.TrimPrefix(trimmed, "##"))
			expectHeading = false
			continue
		}

		expectHeading = false
		out = append(out, line)
	}

	return strings.Join(out, "\n")
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
