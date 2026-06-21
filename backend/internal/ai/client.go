// Package ai wraps the Anthropic Claude API and exposes three planning agents
// (Planner, Priority, Breakdown). Agents depend on the Completer interface so
// they can be unit-tested with a fake; the real Client talks to Claude.
package ai

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// ErrNoAPIKey is returned when constructing a Client without an API key.
var ErrNoAPIKey = errors.New("ai: ANTHROPIC_API_KEY is not set")

// Completer issues a single system+user prompt to an LLM and returns the text
// response. Implemented by Client; faked in tests.
type Completer interface {
	Complete(ctx context.Context, system, user string) (string, error)
}

// Client is a Completer backed by the Anthropic Claude API.
type Client struct {
	api       anthropic.Client
	model     anthropic.Model
	maxTokens int64
	timeout   time.Duration
}

// Option configures a Client.
type Option func(*Client)

// WithMaxTokens sets the response token cap (default 4096).
func WithMaxTokens(n int64) Option { return func(c *Client) { c.maxTokens = n } }

// WithTimeout sets a per-request timeout (default 60s).
func WithTimeout(d time.Duration) Option { return func(c *Client) { c.timeout = d } }

// NewClient builds a Claude-backed Completer. The SDK retries 429/5xx with
// backoff automatically; we add a per-request timeout on top.
func NewClient(apiKey, model string, opts ...Option) (*Client, error) {
	if apiKey == "" {
		return nil, ErrNoAPIKey
	}
	c := &Client{
		api:       anthropic.NewClient(option.WithAPIKey(apiKey)),
		model:     anthropic.Model(model),
		maxTokens: 4096,
		timeout:   60 * time.Second,
	}
	for _, o := range opts {
		o(c)
	}
	return c, nil
}

// Complete sends one message and returns the concatenated text blocks.
func (c *Client) Complete(ctx context.Context, system, user string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.api.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     c.model,
		MaxTokens: c.maxTokens,
		System:    []anthropic.TextBlockParam{{Text: system}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(user)),
		},
	})
	if err != nil {
		return "", fmt.Errorf("claude request: %w", err)
	}

	var sb strings.Builder
	for _, block := range resp.Content {
		if t, ok := block.AsAny().(anthropic.TextBlock); ok {
			sb.WriteString(t.Text)
		}
	}
	out := strings.TrimSpace(sb.String())
	if out == "" {
		return "", errors.New("claude returned empty response")
	}
	return out, nil
}
