package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

// LLMClient streams a deck generation. Prose deltas and agent steps are delivered
// via opts; the full accumulated text is returned when generation completes.
type LLMClient interface {
	Generate(ctx context.Context, prompt string, opts GenOptions) (string, error)
	Name() string
}

// ---------- Anthropic ----------

type AnthropicClient struct {
	apiKey string
	model  string
	http   *http.Client
}

func NewAnthropicClient(apiKey, model string) *AnthropicClient {
	return &AnthropicClient{
		apiKey: apiKey,
		model:  model,
		http:   &http.Client{Timeout: 5 * time.Minute},
	}
}

func (c *AnthropicClient) Name() string { return "anthropic:" + c.model }

func (c *AnthropicClient) Generate(ctx context.Context, prompt string, opts GenOptions) (string, error) {
	reqBody := map[string]any{
		"model":      c.model,
		"max_tokens": 16000,
		"stream":     true,
		"system":     systemPrompt(),
		"messages": []map[string]any{
			{"role": "user", "content": prompt},
		},
	}
	b, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.anthropic.com/v1/messages", bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var sb strings.Builder
		s := bufio.NewScanner(resp.Body)
		for s.Scan() {
			sb.WriteString(s.Text())
		}
		return "", fmt.Errorf("anthropic %d: %s", resp.StatusCode, sb.String())
	}

	var full strings.Builder
	var inTokens, outTokens int64
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 1024*1024), 8*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		var ev struct {
			Type  string `json:"type"`
			Delta struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"delta"`
			// input_tokens arrives on message_start, output_tokens on message_delta.
			Message *struct {
				Usage struct {
					InputTokens  int64 `json:"input_tokens"`
					OutputTokens int64 `json:"output_tokens"`
				} `json:"usage"`
			} `json:"message"`
			Usage *struct {
				InputTokens  int64 `json:"input_tokens"`
				OutputTokens int64 `json:"output_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			continue
		}
		if ev.Type == "content_block_delta" && ev.Delta.Text != "" {
			full.WriteString(ev.Delta.Text)
			opts.delta(ev.Delta.Text)
		}
		// Anthropic reports output_tokens *cumulatively* — message_delta repeats a
		// running total, it does not report an increment. Summing the events would
		// overcount badly, so track the latest value and emit once after the
		// stream instead of calling opts.usage per event.
		if ev.Type == "message_start" && ev.Message != nil {
			inTokens = ev.Message.Usage.InputTokens
			outTokens = ev.Message.Usage.OutputTokens
		}
		if ev.Type == "message_delta" && ev.Usage != nil {
			outTokens = ev.Usage.OutputTokens
		}
	}
	opts.usage(Usage{PromptTokens: inTokens, CompletionTokens: outTokens})
	if err := scanner.Err(); err != nil {
		return full.String(), err
	}
	return full.String(), nil
}

// ---------- OpenAI (Chat Completions, streaming) ----------

type OpenAIClient struct {
	apiKey    string
	model     string
	reasoning string // "low" | "medium" | "high" — only sent for gpt-5* models
	http      *http.Client
}

func NewOpenAIClient(apiKey, model, reasoning string) *OpenAIClient {
	return &OpenAIClient{
		apiKey:    apiKey,
		model:     model,
		reasoning: reasoning,
		http:      &http.Client{Timeout: 5 * time.Minute},
	}
}

func (c *OpenAIClient) Name() string { return "openai:" + c.model }

// Generate dispatches to the tool-using agent loop when opts.Tools is set,
// else a single streamed completion.
func (c *OpenAIClient) Generate(ctx context.Context, prompt string, opts GenOptions) (string, error) {
	if opts.Tools {
		return c.generateAgentic(ctx, prompt, opts)
	}
	msgs := []map[string]any{
		{"role": "system", "content": systemPrompt()},
		{"role": "user", "content": prompt},
	}
	content, _, err := c.streamTurn(ctx, msgs, nil, opts)
	if err != nil {
		return "", err
	}
	return content, nil
}

// generateAgentic runs a tool-calling loop: the model may call fetch_url to
// ground the deck in real facts, then returns the deck in the strict format.
// Each fetch surfaces as a real step. The final turn (no tools) forces an answer.
func (c *OpenAIClient) generateAgentic(ctx context.Context, prompt string, opts GenOptions) (string, error) {
	messages := []map[string]any{
		{"role": "system", "content": systemPrompt()},
		{"role": "system", "content": agentToolHint},
		{"role": "user", "content": prompt},
	}
	tools := []map[string]any{fetchTool()}

	const maxRounds = 6
	for round := 0; round < maxRounds; round++ {
		turnTools := tools
		if round == maxRounds-1 {
			turnTools = nil // last round: no tools, force the deck
		}
		content, calls, err := c.streamTurn(ctx, messages, turnTools, opts)
		if err != nil {
			return "", err
		}
		if len(calls) == 0 {
			return content, nil
		}
		messages = append(messages, map[string]any{
			"role":       "assistant",
			"content":    content,
			"tool_calls": rawToolCalls(calls),
		})
		for _, tc := range calls {
			messages = append(messages, map[string]any{
				"role":         "tool",
				"tool_call_id": tc.ID,
				"content":      execToolCall(ctx, tc, opts),
			})
		}
	}
	return "", fmt.Errorf("agent exceeded max rounds without producing a deck")
}

// tcAccum accumulates one streamed tool call (id/name arrive once, args in pieces).
type tcAccum struct {
	id   string
	name string
	args strings.Builder
}

// streamTurn runs one streamed chat-completions turn, accumulating both prose
// content and any tool calls. Content is forwarded to opts.delta the instant it
// arrives — buffering the whole turn and flushing at the end made the client sit
// on a single spinner for the entire generation (~43s) and then receive
// everything at once. Token usage is reported per turn via opts.usage.
func (c *OpenAIClient) streamTurn(ctx context.Context, messages, tools []map[string]any, opts GenOptions) (string, []toolCall, error) {
	reqBody := map[string]any{
		"model":                 c.model,
		"stream":                true,
		"max_completion_tokens": 32000,
		"messages":              messages,
		// Ask for the usage chunk; without this the stream reports no token counts
		// at all and the shared-key budget can't be metered.
		"stream_options": map[string]any{"include_usage": true},
	}
	if len(tools) > 0 {
		reqBody["tools"] = tools
		reqBody["tool_choice"] = "auto"
	}
	if strings.HasPrefix(c.model, "gpt-5") {
		if len(tools) > 0 {
			reqBody["reasoning_effort"] = "none" // gpt-5 requires this to use function tools on chat/completions
		} else if c.reasoning != "" {
			reqBody["reasoning_effort"] = c.reasoning
		}
	}
	b, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.openai.com/v1/chat/completions", bytes.NewReader(b))
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var sb strings.Builder
		s := bufio.NewScanner(resp.Body)
		for s.Scan() {
			sb.WriteString(s.Text())
		}
		return "", nil, fmt.Errorf("openai %d: %s", resp.StatusCode, sb.String())
	}

	var full strings.Builder
	accum := map[int]*tcAccum{}
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 1024*1024), 8*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		var ev struct {
			Choices []struct {
				Delta struct {
					Content   string `json:"content"`
					ToolCalls []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
			} `json:"choices"`
			Usage *struct {
				PromptTokens     int64 `json:"prompt_tokens"`
				CompletionTokens int64 `json:"completion_tokens"`
				TotalTokens      int64 `json:"total_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			continue
		}
		// The include_usage chunk arrives last and carries no choices — read it
		// before the choices guard below, or it's dropped.
		if ev.Usage != nil {
			opts.usage(Usage{
				PromptTokens:     ev.Usage.PromptTokens,
				CompletionTokens: ev.Usage.CompletionTokens,
				TotalTokens:      ev.Usage.TotalTokens,
			})
		}
		if len(ev.Choices) == 0 {
			continue
		}
		d := ev.Choices[0].Delta
		if d.Content != "" {
			full.WriteString(d.Content)
			opts.delta(d.Content)
		}
		for _, t := range d.ToolCalls {
			a := accum[t.Index]
			if a == nil {
				a = &tcAccum{}
				accum[t.Index] = a
			}
			if t.ID != "" {
				a.id = t.ID
			}
			if t.Function.Name != "" {
				a.name = t.Function.Name
			}
			a.args.WriteString(t.Function.Arguments)
		}
	}
	if err := scanner.Err(); err != nil {
		return full.String(), nil, err
	}

	var calls []toolCall
	if len(accum) > 0 {
		idxs := make([]int, 0, len(accum))
		for i := range accum {
			idxs = append(idxs, i)
		}
		sort.Ints(idxs)
		for _, i := range idxs {
			a := accum[i]
			calls = append(calls, toolCall{ID: a.id, Name: a.name, Args: a.args.String()})
		}
	}
	return full.String(), calls, nil
}

// ---------- Canned fallback (no API key) ----------

type CannedClient struct{}

func (c *CannedClient) Name() string { return "canned" }

// Generate returns the sample deck immediately. It deliberately does NOT pace
// the stream: an 8ms sleep per 24-char chunk spent ~2.4s of an otherwise ~10ms
// request pretending to be a model, which made a fixed template indistinguishable
// from slow AI generation — the single reason "generation is slow" was reported
// against a deployment that was never calling a model at all. Chunking is kept so
// the client's delta handling is exercised on the same code path as a real
// provider; only the artificial delay is gone.
func (c *CannedClient) Generate(ctx context.Context, prompt string, opts GenOptions) (string, error) {
	out := cannedDeck(prompt)
	for _, chunk := range chunkString(out, 24) {
		select {
		case <-ctx.Done():
			return out, ctx.Err()
		default:
		}
		opts.delta(chunk)
	}
	return out, nil
}

func chunkString(s string, n int) []string {
	var out []string
	for len(s) > n {
		out = append(out, s[:n])
		s = s[n:]
	}
	if s != "" {
		out = append(out, s)
	}
	return out
}
