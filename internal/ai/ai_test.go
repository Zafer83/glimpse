package ai

import (
	"strings"
	"testing"

	"github.com/Zafer83/glimpse/internal/config"
	"github.com/Zafer83/glimpse/internal/crawler"
)

func TestRenderPrompt(t *testing.T) {
	tests := []struct {
		name string
		tpl  string
		vars map[string]string
		want string
	}{
		{
			"single var",
			"Hello {{name}}!",
			map[string]string{"name": "World"},
			"Hello World!",
		},
		{
			"multiple vars",
			"{{a}} and {{b}}",
			map[string]string{"a": "X", "b": "Y"},
			"X and Y",
		},
		{
			"no vars",
			"static text",
			nil,
			"static text",
		},
		{
			"unused placeholder",
			"{{missing}} stays",
			map[string]string{},
			"{{missing}} stays",
		},
		{
			"trailing newlines stripped",
			"content\n\n\n",
			nil,
			"content",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderPrompt(tt.tpl, tt.vars)
			if got != tt.want {
				t.Errorf("renderPrompt() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeSlidevMarkdown(t *testing.T) {
	tests := []struct {
		name           string
		raw            string
		theme          string
		wantTheme      bool
		wantTitle      string
		wantBackground bool
	}{
		{
			name:      "adds frontmatter when missing",
			raw:       "# My Slides\n\nContent here",
			theme:     "seriph",
			wantTheme: true,
			wantTitle: "Code Architecture Presentation",
		},
		{
			name:      "preserves title from frontmatter",
			raw:       "---\ntheme: default\ntitle: My Project\n---\n\n# First Slide",
			theme:     "seriph",
			wantTheme: true,
			wantTitle: "My Project",
		},
		{
			name:           "preserves background URL",
			raw:            "---\ntitle: Test\nbackground: https://images.unsplash.com/photo-123\n---\n\n# Slide",
			theme:          "seriph",
			wantTheme:      true,
			wantTitle:      "Test",
			wantBackground: true,
		},
		{
			name:      "empty body gets fallback heading",
			raw:       "---\ntheme: x\ntitle: Empty\n---\n",
			theme:     "seriph",
			wantTheme: true,
			wantTitle: "Empty",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{Theme: tt.theme}
			got := normalizeSlidevMarkdown(tt.raw, cfg)

			if tt.wantTheme && !strings.Contains(got, "theme: '"+tt.theme+"'") {
				t.Errorf("expected theme %q in output:\n%s", tt.theme, got)
			}
			if tt.wantTitle != "" && !strings.Contains(got, tt.wantTitle) {
				t.Errorf("expected title %q in output:\n%s", tt.wantTitle, got)
			}
			if tt.wantBackground && !strings.Contains(got, "background:") {
				t.Errorf("expected background in output:\n%s", got)
			}
			// Must always start with ---
			if !strings.HasPrefix(got, "---\n") {
				t.Errorf("output must start with frontmatter:\n%s", got)
			}
			// Must always contain layout
			if !strings.Contains(got, "layout:") {
				t.Errorf("output must contain layout:\n%s", got)
			}
		})
	}
}

func TestNormalizeSlidevMarkdown_CoverLayout(t *testing.T) {
	raw := "---\ntheme: seriph\nlayout: cover\nauthor: Zafer\ntitle: My Project\n---\n\n# My Project\n\nDescription"
	cfg := &config.Config{Theme: "seriph"}
	got := normalizeSlidevMarkdown(raw, cfg)

	if !strings.Contains(got, "layout: cover") {
		t.Error("expected layout: cover in output")
	}
	if !strings.Contains(got, "author: 'Zafer'") {
		t.Errorf("expected author in output:\n%s", got)
	}
}

func TestRemoveMermaidBlocks(t *testing.T) {
	input := "# Slide\n\nSome text\n\n```mermaid\ngraph TD\n    A --> B\n```\n\nMore text"
	got := removeMermaidBlocks(input)

	if strings.Contains(got, "mermaid") {
		t.Errorf("mermaid block should be removed:\n%s", got)
	}
	if !strings.Contains(got, "Some text") {
		t.Error("non-mermaid content should be preserved")
	}
	if !strings.Contains(got, "More text") {
		t.Error("text after mermaid block should be preserved")
	}
}

func TestRemoveMermaidBlocks_NoMermaid(t *testing.T) {
	input := "# Slide\n\n```go\nfmt.Println(\"hello\")\n```\n"
	got := removeMermaidBlocks(input)
	if got != input {
		t.Error("non-mermaid code blocks should be untouched")
	}
}

func TestValidateSlidevOutput(t *testing.T) {
	tests := []struct {
		name    string
		md      string
		wantErr bool
	}{
		{"valid 5 slides", "---\n\n# A\n\n---\n\n# B\n\n---\n\n# C\n\n---\n\n# D\n", false},
		{"only 2 separators", "---\n\n# A\n\n---\n\n# B\n", true},
		{"no separators", "# Just text\n\nSome content.", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSlidevOutput(tt.md)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSlidevOutput() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEnsureTitleSlide(t *testing.T) {
	tests := []struct {
		name  string
		body  string
		title string
		want  string
	}{
		{"already has H1", "# Existing\n\nContent", "Fallback", "# Existing"},
		{"no H1 adds title", "Some content", "My Title", "# My Title"},
		{"empty body", "", "Title", "# Title"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ensureTitleSlide(tt.body, tt.title)
			if !strings.HasPrefix(strings.TrimSpace(got), tt.want) {
				t.Errorf("ensureTitleSlide() = %q, want prefix %q", got, tt.want)
			}
		})
	}
}

func TestEnsureSlideBreakPerTopLevelHeading(t *testing.T) {
	input := "# First\n\nContent\n\n# Second\n\nMore content\n\n# Third\n"
	got := ensureSlideBreakPerTopLevelHeading(input)

	count := strings.Count(got, "\n---\n")
	if count < 2 {
		t.Errorf("expected at least 2 slide breaks, got %d in:\n%s", count, got)
	}
}

func TestPrepareCodeForModel(t *testing.T) {
	short := "small code"
	code, note := prepareCodeForModel(short, "gpt-4o")
	if code != short {
		t.Error("short code should not be truncated")
	}
	if note != "" {
		t.Error("no truncation note expected for short code")
	}

	long := strings.Repeat("x", 200000)
	code, note = prepareCodeForModel(long, "gpt-4o")
	if len(code) >= len(long) {
		t.Error("long code should be truncated")
	}
	if note == "" {
		t.Error("expected truncation note for long code")
	}
	if !strings.Contains(code, "TRUNCATED") {
		t.Error("truncated code should contain marker")
	}
}

func TestAssembleContentForModel_BudgetWeighting(t *testing.T) {
	content := &crawler.CollectedContent{
		Docs: []crawler.FileEntry{
			{Path: "README.md", Content: "# Project\nThis is the project overview.", Category: "doc"},
		},
		Business: []crawler.FileEntry{
			{Path: "main.go", Content: "package main\nfunc main() {}", Category: "business"},
		},
		Support: []crawler.FileEntry{
			{Path: "main_test.go", Content: "package main\nfunc TestMain(t *testing.T) {}", Category: "support"},
		},
	}

	result, note := assembleContentForModel(content, "gpt-4o")

	// All three sections should be present.
	if !strings.Contains(result, "PROJECT DOCUMENTATION") {
		t.Error("expected DOCUMENTATION section")
	}
	if !strings.Contains(result, "CORE BUSINESS LOGIC") {
		t.Error("expected BUSINESS LOGIC section")
	}
	if !strings.Contains(result, "SUPPORTING CODE") {
		t.Error("expected SUPPORTING CODE section")
	}
	if !strings.Contains(result, "README.md") {
		t.Error("expected README.md in output")
	}
	if !strings.Contains(result, "main.go") {
		t.Error("expected main.go in output")
	}
	if !strings.Contains(result, "main_test.go") {
		t.Error("expected main_test.go in output")
	}
	// Small content should not be truncated.
	if note != "" {
		t.Errorf("unexpected truncation note for small content: %q", note)
	}
}

func TestAssembleContentForModel_EmptyContent(t *testing.T) {
	content := &crawler.CollectedContent{}
	result, _ := assembleContentForModel(content, "gpt-4o")

	if !strings.Contains(result, "DOCUMENTATION FIRST") {
		t.Error("expected header even for empty content")
	}
	// Should not contain section headers for empty tiers.
	if strings.Contains(result, "CORE BUSINESS LOGIC") {
		t.Error("should not include business section when empty")
	}
}

func TestAssembleContentForModel_DocsFirst(t *testing.T) {
	content := &crawler.CollectedContent{
		Docs: []crawler.FileEntry{
			{Path: "README.md", Content: "docs content", Category: "doc"},
		},
		Business: []crawler.FileEntry{
			{Path: "app.go", Content: "code content", Category: "business"},
		},
	}

	result, _ := assembleContentForModel(content, "gpt-4o")

	docsIdx := strings.Index(result, "docs content")
	codeIdx := strings.Index(result, "code content")
	if docsIdx < 0 || codeIdx < 0 {
		t.Fatal("both docs and code should be in output")
	}
	if docsIdx > codeIdx {
		t.Error("docs should appear before code in output")
	}
}

func TestMaxPromptBytesForModel(t *testing.T) {
	tests := []struct {
		model string
		want  int
	}{
		{"local", 12000},
		{"local/qwen2.5-coder:7b", 12000},
		{"ollama/mistral", 12000},
		{"claude-3-5-sonnet-latest", 85000},
		{"gemini-2.0-flash", 140000},
		{"gpt-4o", 180000},
	}
	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			got := maxPromptBytesForModel(tt.model)
			if got != tt.want {
				t.Errorf("maxPromptBytesForModel(%q) = %d, want %d", tt.model, got, tt.want)
			}
		})
	}
}
