package ai

import (
	"fmt"

	"github.com/Zafer83/glimpse/internal/config"
)

// GenerateSlides routes the slide generation request to the appropriate
// LLM provider based on the model name in the config.
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

	return generateSlidesWithOpenAI(cfg, systemPrompt, userPrompt)
}
