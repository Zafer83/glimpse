package ai

import "testing"

func TestNormalizeLocalModel(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"local/qwen:7b", "qwen:7b"},
		{"ollama/mistral", "mistral"},
		{"local", "qwen2.5-coder:7b"},
		{"ollama", "qwen2.5-coder:7b"},
		{"Local", "qwen2.5-coder:7b"},
		{"", "qwen2.5-coder:7b"},
		{"local/custom-model:latest", "custom-model:latest"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got := normalizeLocalModel(tt.in)
			if got != tt.want {
				t.Errorf("normalizeLocalModel(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestLocalOllamaChatEndpoint(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"http://localhost:11434", "http://localhost:11434/api/chat"},
		{"http://localhost:11434/", "http://localhost:11434/api/chat"},
		{"http://localhost:11434/api/chat", "http://localhost:11434/api/chat"},
		{"localhost:11434", "http://localhost:11434/api/chat"},
		{"", "http://127.0.0.1:11434/api/chat"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got := localOllamaChatEndpoint(tt.in)
			if got != tt.want {
				t.Errorf("localOllamaChatEndpoint(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestLocalOpenAIChatEndpoint(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"http://localhost:8080", "http://localhost:8080/v1/chat/completions"},
		{"http://localhost:11434/", "http://localhost:11434/v1/chat/completions"},
		{"http://localhost:8080/v1/chat/completions", "http://localhost:8080/v1/chat/completions"},
		{"http://localhost:8080/v1", "http://localhost:8080/v1/chat/completions"},
		{"", "http://127.0.0.1:11434/v1/chat/completions"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got := localOpenAIChatEndpoint(tt.in)
			if got != tt.want {
				t.Errorf("localOpenAIChatEndpoint(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestIsGenericLocalInput(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"local", true},
		{"ollama", true},
		{"Local", true},
		{"OLLAMA", true},
		{"local/foo", false},
		{"gpt-4o", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got := isGenericLocalInput(tt.in)
			if got != tt.want {
				t.Errorf("isGenericLocalInput(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestIsGeminiModel(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"gemini-2.0-flash", true},
		{"gpt-4o", false},
		{"local", false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := isGeminiModel(tt.in); got != tt.want {
				t.Errorf("isGeminiModel(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestIsAnthropicModel(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"claude-3-5-sonnet-latest", true},
		{"gpt-4o", false},
		{"local", false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := isAnthropicModel(tt.in); got != tt.want {
				t.Errorf("isAnthropicModel(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestIsLocalModel(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"local", true},
		{"local/qwen:7b", true},
		{"ollama", true},
		{"ollama/mistral", true},
		{"gpt-4o", false},
		{"gemini-2.0-flash", false},
		{"claude-3-5-sonnet-latest", false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := isLocalModel(tt.in); got != tt.want {
				t.Errorf("isLocalModel(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestRequiresAPIKey(t *testing.T) {
	if RequiresAPIKey("local") {
		t.Error("local should not require API key")
	}
	if !RequiresAPIKey("gpt-4o") {
		t.Error("gpt-4o should require API key")
	}
}
