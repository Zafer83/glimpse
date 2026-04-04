package ai

import (
	"strings"
	"testing"

	"github.com/Zafer83/glimpse/internal/config"
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
		})
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
