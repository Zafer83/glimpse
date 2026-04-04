package ai

import (
	"context"
	"fmt"

	"github.com/Zafer83/glimpse/internal/config"
	"github.com/sashabaranov/go-openai"
)

// generateSlidesWithOpenAI sends the prompts to the OpenAI chat completion API
// and returns the generated slide content.
func generateSlidesWithOpenAI(cfg *config.Config, systemPrompt, userPrompt string) (string, error) {
	client := openai.NewClient(cfg.APIKey)
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
		return "", fmt.Errorf("openai completion: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("openai returned no choices")
	}
	return resp.Choices[0].Message.Content, nil
}
