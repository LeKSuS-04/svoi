package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/LeKSuS-04/svoi-bot/internal/logging"
)

type Config struct {
	BaseURL             string        `yaml:"base_url"`
	APIKey              string        `env:"AI_API_KEY"`
	Model               string        `yaml:"model"`
	FallbackModels      []string      `yaml:"fallback_models"`
	ResponseResetPeriod time.Duration `yaml:"reset_period"`
	SystemPrompt        string        `yaml:"system_prompt"`
}

type AI struct {
	model        string
	cfg          *Config
	log          *slog.Logger
	systemPrompt *template.Template
}

type UserContext struct {
	Username  string
	FirstName string
	LastName  string
}

func NewAI(config *Config) (*AI, error) {
	systemPrompt, err := template.New("system_prompt").Parse(config.SystemPrompt)
	if err != nil {
		return nil, fmt.Errorf("parse system prompt: %w", err)
	}

	return &AI{
		model:        config.Model,
		cfg:          config,
		log:          logging.New("ai"),
		systemPrompt: systemPrompt,
	}, nil
}

func (a *AI) GeneratePatrioticResponse(ctx context.Context, prompt string, userContext UserContext) (response string, err error) {
	start := time.Now()
	defer func() {
		if err != nil {
			failedGenerations.Inc()
		} else {
			successfulGenerations.Inc()
		}
		duration := time.Since(start).Seconds()
		generationDurationSeconds.Observe(duration)
	}()

	systemPromptBuf := bytes.NewBuffer(nil)
	err = a.systemPrompt.Execute(systemPromptBuf, userContext)
	if err != nil {
		return "", fmt.Errorf("render system prompt: %w", err)
	}
	a.log.DebugContext(ctx, "rendered system prompt", "prompt", systemPromptBuf.String(), "userContext", userContext)

	reqModel := OpenrouterRequest{
		Model:  a.model,
		Models: a.cfg.FallbackModels,
		Messages: []Message{
			{Role: "system", Content: systemPromptBuf.String()},
			{Role: "user", Content: prompt},
		},
	}

	jsonReq, err := json.Marshal(reqModel)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	a.log.DebugContext(ctx, "sending request to ai provider", "url", a.cfg.BaseURL, "primaryModel", a.model, "fallbackModels", a.cfg.FallbackModels)
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
	defer func() { _ = rsp.Body.Close() }()

	if rsp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(rsp.Body, 1024))
		return "", fmt.Errorf("unexpected status code: %d; body: %s", rsp.StatusCode, string(body))
	}

	var rspModel OpenrouterResponse
	if err := json.NewDecoder(rsp.Body).Decode(&rspModel); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	a.log.DebugContext(ctx, "received response from ai provider", "usedModel", rspModel.Model, "usage", rspModel.Usage)

	promptTokens.WithLabelValues(rspModel.Model).Add(float64(rspModel.Usage.PromptTokens))
	completionTokens.WithLabelValues(rspModel.Model).Add(float64(rspModel.Usage.CompletionTokens))
	totalTokens.WithLabelValues(rspModel.Model).Add(float64(rspModel.Usage.TotalTokens))

	if len(rspModel.Choices) != 1 {
		return "", fmt.Errorf("unexpected number of choices: %d", len(rspModel.Choices))
	}

	choice := rspModel.Choices[0]
	if choice.Message.Content == "" {
		return "", fmt.Errorf("empty message content")
	}

	message := strings.TrimSpace(choice.Message.Content)
	a.log.DebugContext(ctx, "generated ai response", "response", message)
	return message, nil
}

type OpenrouterRequest struct {
	Model    string    `json:"model"`
	Models   []string  `json:"models,omitempty"`
	Messages []Message `json:"messages"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenrouterResponse struct {
	Choices []Choice `json:"choices"`
	Model   string   `json:"model"`
	Usage   Usage    `json:"usage"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type Choice struct {
	Message Message `json:"message"`
}
