package ai

import (
	"context"
	"fmt"

	"github.com/Zafer83/glimpse/internal/config"
	"github.com/sashabaranov/go-openai"
)

func GenerateSlides(cfg *config.Config, code string) (string, error) {
	client := openai.NewClient(cfg.APIKey)

	systemPrompt := fmt.Sprintf("You are Glimpse. Create a Slidev presentation in %s language.", cfg.Language)

	userPrompt := fmt.Sprintf(
		"Create a professional tech deep-dive in Slidev Markdown.\n"+
			"- Theme: '%s'\n"+
			"- Use focus markers for code.\n"+
			"- Language: %s\n"+
			"- Include Mermaid.js diagrams.\n"+
			"- Return ONLY raw Markdown.\n\nSOURCE:\n%s",
		cfg.Theme, cfg.Language, code)

	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: cfg.Model, // Dynamisches Modell aus .env
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
