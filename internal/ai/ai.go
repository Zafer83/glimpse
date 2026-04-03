package ai

import (
	"context"
	"fmt"

	"github.com/Zafer83/glimpse/internal/config"
)

func GenerateSlides(cfg *config.Config, code string) (string, error) {
	client := openai.NewClient(cfg.APIKey)

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

	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: cfg.Model,
			Messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
				{Role: openai.ChatMessageRoleUser, Content: userPrompt},
			},
		},
	)

	if err != nil {
		return "", err
	}

	return resp.Choices[0].Message.Content, nil
}
