package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
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
	model string
	cfg   *Config
}

func NewAI(config *Config) *AI {
	return &AI{model: config.Model, cfg: config}
}

func (a *AI) GeneratePatrioticResponse(ctx context.Context, log *logrus.Entry, prompt string) (response string, err error) {
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

	reqModel := OpenrouterRequest{
		Model:  a.model,
		Models: a.cfg.FallbackModels,
		Messages: []Message{
			{Role: "system", Content: a.cfg.SystemPrompt},
			{Role: "user", Content: prompt},
		},
	}

	jsonReq, err := json.Marshal(reqModel)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	log.WithFields(logrus.Fields{
		"url":             a.cfg.BaseURL,
		"model":           a.model,
		"fallback_models": a.cfg.FallbackModels,
	}).Debug("Sending request to AI provider")
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
		return "", fmt.Errorf("unexpected status code: %d", rsp.StatusCode)
	}

	var rspModel OpenrouterResponse
	if err := json.NewDecoder(rsp.Body).Decode(&rspModel); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	log.WithFields(logrus.Fields{
		"used_model": rspModel.Model,
		"usage":      rspModel.Usage,
	}).Info("Received response from AI provider")
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

	return strings.TrimSpace(choice.Message.Content), nil
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
