package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Step is one entry in the agent activity feed, streamed to the client as an SSE
// `step` event. Kind is one of: read | search | write | check | fix | think.
// Status is: start (in progress) | ok | error. The same ID is re-sent as the
// status changes; the client merges by ID.
type Step struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Label  string `json:"label"`
	Target string `json:"target,omitempty"`
	Status string `json:"status"`
}

// Usage is the token cost of one generation, summed across every round of the
// agent loop (tool rounds included) and every compile-check retry. It backs the
// shared-key budget — see Store.AddSharedUsage.
type Usage struct {
	PromptTokens     int64 `json:"promptTokens"`
	CompletionTokens int64 `json:"completionTokens"`
	TotalTokens      int64 `json:"totalTokens"`
}

// total is the billable token count for one round. OpenAI reports a total
// directly; Anthropic reports only input/output, so fall back to their sum.
func (u Usage) total() int64 {
	if u.TotalTokens > 0 {
		return u.TotalTokens
	}
	return u.PromptTokens + u.CompletionTokens
}

func (u *Usage) add(o Usage) {
	u.PromptTokens += o.PromptTokens
	u.CompletionTokens += o.CompletionTokens
	u.TotalTokens += o.total()
}

// GenOptions carries the streaming + agent callbacks into an LLMClient.Generate.
// All callbacks are optional; use the nil-safe helpers.
type GenOptions struct {
	OnDelta func(string) // streamed prose deltas, emitted as the model writes
	OnStep  func(Step)   // agent activity steps
	OnUsage func(Usage)  // token usage, once per provider round
	Tools   bool         // enable the tool-using agent loop (OpenAI)
}

func (o GenOptions) delta(s string) {
	if o.OnDelta != nil {
		o.OnDelta(s)
	}
}

func (o GenOptions) step(s Step) {
	if o.OnStep != nil {
		o.OnStep(s)
	}
}

func (o GenOptions) usage(u Usage) {
	if o.OnUsage != nil && (u.TotalTokens > 0 || u.PromptTokens > 0 || u.CompletionTokens > 0) {
		o.OnUsage(u)
	}
}

// toolCall is one assistant tool invocation accumulated from the stream.
type toolCall struct {
	ID   string
	Name string
	Args string
}

// agentToolHint is appended as a second system message only on the agentic
// (OpenAI) path — it tells the model it may reach the web to ground the deck.
const agentToolHint = `You have a fetch_url tool. If the user's prompt references a URL, product, ` +
	`company, or website, FIRST call fetch_url on the most relevant URL(s) to ground the deck in ` +
	`real facts before writing it. Never invent facts, metrics, or claims about a real brand — if ` +
	`you cannot fetch it, keep claims generic. After gathering what you need, output the deck in the ` +
	`strict format described in the system prompt.`

// fetchTool is the OpenAI function-tool schema for fetch_url.
func fetchTool() map[string]any {
	return map[string]any{
		"type": "function",
		"function": map[string]any{
			"name": "fetch_url",
			"description": "Fetch a web page and return its readable text. Use this to ground the deck " +
				"in real facts when the prompt references a URL, product, or company.",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"url": map[string]any{
						"type":        "string",
						"description": "Absolute http(s) URL to fetch",
					},
				},
				"required": []string{"url"},
			},
		},
	}
}

// rawToolCalls rebuilds the assistant message tool_calls array to send back to
// the model on the next turn.
func rawToolCalls(tcs []toolCall) []map[string]any {
	out := make([]map[string]any, len(tcs))
	for i, tc := range tcs {
		out[i] = map[string]any{
			"id":   tc.ID,
			"type": "function",
			"function": map[string]any{
				"name":      tc.Name,
				"arguments": tc.Args,
			},
		}
	}
	return out
}

// execToolCall runs a single tool call, emitting start/ok/error steps, and
// returns the tool result string fed back to the model. Errors are returned as
// content (not Go errors) so the model can adapt instead of the run aborting.
func execToolCall(ctx context.Context, tc toolCall, opts GenOptions) string {
	switch tc.Name {
	case "fetch_url":
		var args struct {
			URL string `json:"url"`
		}
		_ = json.Unmarshal([]byte(tc.Args), &args)
		host := args.URL
		if u, err := url.Parse(args.URL); err == nil && u.Host != "" {
			host = u.Host
		}
		opts.step(Step{ID: tc.ID, Kind: "search", Label: "Researching " + host, Target: args.URL, Status: "start"})
		text, err := fetchURL(ctx, args.URL)
		if err != nil {
			opts.step(Step{ID: tc.ID, Kind: "search", Label: "Couldn't reach " + host, Target: args.URL, Status: "error"})
			return "fetch_url error: " + err.Error() + "\nProceed without it; do not invent facts about the site."
		}
		opts.step(Step{ID: tc.ID, Kind: "search", Label: "Read " + host, Target: args.URL, Status: "ok"})
		return "Content of " + args.URL + ":\n" + text
	default:
		return "unknown tool: " + tc.Name
	}
}

var reScriptStyle = regexp.MustCompile(`(?is)<(script|style|noscript)\b[^>]*>.*?</(script|style|noscript)>`)
var reTag = regexp.MustCompile(`(?s)<[^>]+>`)
var reInlineWS = regexp.MustCompile(`[ \t\r\f\v]+`)
var reBlankLines = regexp.MustCompile(`\n{3,}`)

// fetchURL GETs an http/https page and returns readable text (scripts/styles and
// tags stripped, whitespace collapsed, capped). Only http(s) is allowed.
func fetchURL(ctx context.Context, raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", fmt.Errorf("invalid or unsupported URL: %q", raw)
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; CueBot/1.0)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,text/plain")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return "", err
	}
	return htmlToText(string(body)), nil
}

// htmlToText renders raw HTML down to grounding-quality plain text.
func htmlToText(in string) string {
	s := reScriptStyle.ReplaceAllString(in, " ")
	s = reTag.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	s = reInlineWS.ReplaceAllString(s, " ")
	s = reBlankLines.ReplaceAllString(s, "\n\n")
	s = strings.TrimSpace(s)
	const limit = 8000
	if len(s) > limit {
		s = s[:limit] + "\n…[truncated]"
	}
	return s
}
