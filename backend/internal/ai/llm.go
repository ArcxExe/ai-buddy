package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"aibuddy/backend/internal/mcp"
)

var affordableTokensRegex = regexp.MustCompile(`can only afford\s+(\d+)`)

// ToolCallFunction represents the tool name and JSON-encoded arguments chosen by the LLM.
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolCall represents a single OpenAI/OpenRouter tool invocation entry.
type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

// ChatCompletionMessage models a message in multi-turn LLM tool-calling conversations.
type ChatCompletionMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	Name       string     `json:"name,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

// ChatCompletionRequest holds input parameters for an LLM chat completion request.
type ChatCompletionRequest struct {
	Model       string                  `json:"model"`
	Messages    []ChatCompletionMessage `json:"messages"`
	Tools       []mcp.OpenAITool        `json:"tools,omitempty"`
	Temperature float64                 `json:"temperature,omitempty"`
	MaxTokens   int                     `json:"max_tokens,omitempty"`
}

// ChatCompletionResponse contains the assistant text and any requested tool calls.
type ChatCompletionResponse struct {
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	Model     string     `json:"model,omitempty"`
}

// LLMClient abstracts external LLM inference (OpenRouter or test mocks) per AGENTS.md §5.3.
type LLMClient interface {
	ChatCompletion(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, error)
}

// OpenRouterClient calls OpenRouter's OpenAI-compatible /chat/completions API.
type OpenRouterClient struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// NewOpenRouterClient configures an OpenRouter HTTP client with sensible timeouts.
func NewOpenRouterClient(baseURL, apiKey string, httpClient *http.Client, timeout time.Duration) *OpenRouterClient {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://openrouter.ai/api/v1"
	}
	if timeout <= 0 {
		timeout = 25 * time.Second
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: timeout}
	}
	return &OpenRouterClient{
		BaseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		APIKey:     strings.TrimSpace(apiKey),
		HTTPClient: httpClient,
	}
}

// ChatCompletion sends a chat completion call (with optional OpenAI function tools) to OpenRouter.
func (c *OpenRouterClient) ChatCompletion(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, error) {
	if c == nil || c.APIKey == "" {
		return nil, errors.New("openrouter api key is empty")
	}
	if req.MaxTokens <= 0 {
		req.MaxTokens = defaultMaxTokens
	}

	statusCode, body, err := c.sendChatRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	if statusCode == http.StatusPaymentRequired {
		if clamped, ok := clampAffordableTokens(string(body), req.MaxTokens); ok {
			req.MaxTokens = clamped
			statusCode, body, err = c.sendChatRequest(ctx, req)
			if err != nil {
				return nil, err
			}
		}
	}

	if statusCode >= 300 {
		return nil, fmt.Errorf("openrouter HTTP %d: %s", statusCode, strings.TrimSpace(string(body)))
	}

	var parsed struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content   *string    `json:"content"`
				ToolCalls []ToolCall `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("openrouter decode response: %w", err)
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return nil, fmt.Errorf("openrouter error: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return nil, errors.New("openrouter returned empty choices")
	}

	var content string
	if parsed.Choices[0].Message.Content != nil {
		content = strings.TrimSpace(*parsed.Choices[0].Message.Content)
	}

	return &ChatCompletionResponse{
		Content:   content,
		ToolCalls: parsed.Choices[0].Message.ToolCalls,
		Model:     parsed.Model,
	}, nil
}

func (c *OpenRouterClient) sendChatRequest(ctx context.Context, req ChatCompletionRequest) (int, []byte, error) {
	payloadBytes, err := json.Marshal(req)
	if err != nil {
		return 0, nil, err
	}

	endpoint := c.BaseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payloadBytes))
	if err != nil {
		return 0, nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	httpReq.Header.Set("HTTP-Referer", "https://avito.ru/ai-buddy-prototype")
	httpReq.Header.Set("X-Title", "AI-buddy Avito Start Prototype")

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, body, nil
}

func clampAffordableTokens(errBody string, currentMax int) (int, bool) {
	m := affordableTokensRegex.FindStringSubmatch(errBody)
	if len(m) < 2 {
		return 0, false
	}
	afford, err := strconv.Atoi(m[1])
	if err != nil || afford < 64 || afford >= currentMax {
		return 0, false
	}
	clamped := afford * 8 / 10
	if clamped < 64 {
		clamped = 64
	}
	if clamped >= afford {
		clamped = afford - 1
	}
	if clamped <= 0 {
		return 0, false
	}
	return clamped, true
}
