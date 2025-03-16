package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Config struct {
	BaseURL             string        `yaml:"base_url"`
	APIKey              string        `env:"AI_API_KEY"`
	Model               string        `yaml:"model"`
	ResponseResetPeriod time.Duration `yaml:"reset_period"`
}

type AI struct {
	model string
	cfg   *Config
}

func NewAI(config *Config) *AI {
	return &AI{model: config.Model, cfg: config}
}

func (a *AI) GeneratePatrioticResponse(ctx context.Context, prompt string) (string, error) {
	const systemPrompt = `
	Ты — российский патриот, который очень любит свою родину, а ещё глупые шутки, анекдоты и мемы.
	У тебя нет цели разжигать ненависть, ты просто очень любишь свою страну.
	В диалоге с тобой сказали фразу. Ответь на неё кратко, но смешно, а главное патриотчино.
	Используй много эмодзи и выгляди излишне глупым.`

	reqModel := OpenrouterRequest{
		Model: a.model,
		Messages: []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: prompt},
		},
	}

	jsonReq, err := json.Marshal(reqModel)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.cfg.BaseURL, bytes.NewBuffer(jsonReq))
	if err != nil {
		return "", fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.cfg.APIKey))

	rsp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("new completion: %w", err)
	}

	var rspModel OpenrouterResponse
	if err := json.NewDecoder(rsp.Body).Decode(&rspModel); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	if len(rspModel.Choices) != 1 {
		return "", fmt.Errorf("unexpected number of choices: %d", len(rspModel.Choices))
	}

	choice := rspModel.Choices[0]
	if choice.Message.Content == "" {
		return "", fmt.Errorf("empty message content")
	}

	return strings.TrimSpace(choice.Message.Content), nil
}

type OpenrouterRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenrouterResponse struct {
	Choices []Choice `json:"choices"`
}

type Choice struct {
	Message Message `json:"message"`
}
