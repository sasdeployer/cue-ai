package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type API struct {
	store *Store
	llm   LLMClient
	cfg   Config
}

// llmFor resolves which LLMClient to use for this request. A visitor's own
// key (X-User-OpenAI-Key / X-User-Anthropic-Key, set from the Settings page's
// BYOK form) takes priority over the server's own configured client. It's
// used to build a one-off client for this single request only — never
// logged, never persisted, never reused across requests.
// The bool result reports whether this request spends the SERVER's key, which
// is what the shared-token cap meters. A visitor's own key is never metered,
// and canned mode calls no provider at all, so neither counts as shared.
func (a *API) llmFor(c *gin.Context) (LLMClient, bool) {
	if k := c.GetHeader("X-User-OpenAI-Key"); k != "" {
		return NewOpenAIClient(k, a.cfg.OpenAIModel, a.cfg.OpenAIReasoning), false
	}
	if k := c.GetHeader("X-User-Anthropic-Key"); k != "" {
		return NewAnthropicClient(k, a.cfg.AnthropicModel), false
	}
	_, canned := a.llm.(*CannedClient)
	return a.llm, !canned
}

// sharedBudget reports the shared key's spend against the cap. exhausted is
// always false when the cap is disabled (SHARED_TOKEN_CAP=0).
func (a *API) sharedBudget(ctx context.Context) (used, limit int64, exhausted bool, err error) {
	limit = a.cfg.SharedTokenCap
	if limit <= 0 {
		return 0, 0, false, nil
	}
	used, err = a.store.SharedUsage(ctx)
	if err != nil {
		return 0, limit, false, err
	}
	return used, limit, used >= limit, nil
}

// meterShared returns an OnUsage callback that records token spend against the
// shared key, or nil when this request is BYOK/canned and costs nothing.
func (a *API) meterShared(shared bool) func(Usage) {
	if !shared || a.cfg.SharedTokenCap <= 0 {
		return nil
	}
	return func(u Usage) {
		// Deliberately NOT the request context: the client may have navigated
		// away mid-stream, but those tokens were still spent and still have to be
		// charged against the cap, or a page-refresh loop would be free.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := a.store.AddSharedUsage(ctx, u); err != nil {
			log.Printf("shared usage meter: %v", err)
		}
	}
}

// budgetPayload is the SSE `budget` event and the /api/shared-budget body. The
// client shows the BYOK modal on `exhausted`.
func budgetPayload(used, limit int64) gin.H {
	remaining := limit - used
	if remaining < 0 {
		remaining = 0
	}
	return gin.H{
		"used":      used,
		"cap":       limit,
		"remaining": remaining,
		"exhausted": limit > 0 && used >= limit,
	}
}

type generateReq struct {
	Prompt string `json:"prompt"`
}

type editReq struct {
	Instruction string `json:"instruction"`
	AppTSX      string `json:"appTsx"`
	TokensCSS   string `json:"tokensCss"`
}

// sse writes one Server-Sent Event and flushes.
func sse(c *gin.Context, event string, payload any) bool {
	b, _ := json.Marshal(payload)
	if _, err := c.Writer.WriteString("event: " + event + "\ndata: " + string(b) + "\n\n"); err != nil {
		return false
	}
	c.Writer.Flush()
	return true
}

// sseHeaders sets the streaming response headers and flushes the status line.
func sseHeaders(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.Flush()
}

// stepper emits agent activity steps as SSE `step` events. begin() streams a
// step in the "start" state and returns its id; finish() re-streams the same
// step with a terminal status. The client merges by id.
type stepper struct {
	c *gin.Context
	n int
	m map[string]Step
}

func newStepper(c *gin.Context) *stepper {
	return &stepper{c: c, m: map[string]Step{}}
}

func (s *stepper) begin(kind, label, target string) string {
	s.n++
	id := fmt.Sprintf("s%d", s.n)
	st := Step{ID: id, Kind: kind, Label: label, Target: target, Status: "start"}
	s.m[id] = st
	sse(s.c, "step", st)
	return id
}

func (s *stepper) finish(id, status string) {
	st := s.m[id]
	st.Status = status
	sse(s.c, "step", st)
}

// generateChecked runs the model with a compile-check + silent-retry loop, and
// streams a real activity feed of the work: the agent generation (which may
// itself fetch the web and emit search steps), the tokens.css/App.tsx writes,
// the type-check, and any fix-on-retry. Attempt 1 streams prose deltas; retries
// are silent (their progress is the step feed). Returns the parsed deck once
// CheckDeck passes, or the last error if none did.
// onUsage, when non-nil, is invoked with the token spend of every provider
// round across all attempts — retries included, since a failed compile still
// costs tokens.
func (a *API) generateChecked(ctx context.Context, c *gin.Context, llm LLMClient, prompt string, sp *stepper, onUsage func(Usage)) (message, title, tokensCSS, appTSX string, err error) {
	var lastErr error
	for i := 0; i < 3; i++ {
		p := prompt
		if i > 0 {
			fx := sp.begin("fix", "Retake", "")
			sp.finish(fx, "ok")
			p = prompt + "\n\nThe previous attempt did not compile:\n" + lastErr.Error() +
				"\nReturn the COMPLETE corrected deck in the same format."
		}

		opts := GenOptions{
			OnStep:  func(st Step) { sse(c, "step", st) },
			OnUsage: onUsage,
			Tools:   true,
		}
		if i == 0 {
			opts.OnDelta = func(delta string) { sse(c, "delta", gin.H{"text": delta}) }
		}

		label := "Setting the scene"
		if i > 0 {
			label = "Calling for a retake"
		}
		think := sp.begin("think", label, "")

		raw, gerr := llm.Generate(ctx, p, opts)
		if gerr != nil {
			sp.finish(think, "error")
			return "", "", "", "", gerr
		}

		message, title, tokensCSS, appTSX = parseGeneration(raw)
		if appTSX == "" {
			sp.finish(think, "error")
			lastErr = errors.New("the model did not return a valid deck (no App.tsx)")
			continue
		}
		sp.finish(think, "ok")

		if tokensCSS != "" {
			w := sp.begin("write", "Lighting the set", "src/styles/tokens.css")
			sp.finish(w, "ok")
		}
		wa := sp.begin("write", "Blocking the shot", "src/App.tsx")
		sp.finish(wa, "ok")

		chk := sp.begin("check", "Rolling tape", "")
		if cerr := CheckDeck(appTSX); cerr != nil {
			sp.finish(chk, "error")
			lastErr = cerr
			continue
		}
		sp.finish(chk, "ok")
		return message, title, tokensCSS, appTSX, nil
	}
	return "", "", "", "", lastErr
}

// POST /api/decks — stream generation, compile-check + retry, persist, return the deck.
func (a *API) generate(c *gin.Context) {
	var req generateReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Prompt == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "prompt is required"})
		return
	}

	sseHeaders(c)
	ctx := c.Request.Context()

	llm, shared := a.llmFor(c)
	if shared {
		if used, limit, exhausted, berr := a.sharedBudget(ctx); berr != nil {
			// Fail open: a meter outage shouldn't take generation down.
			log.Printf("shared budget check: %v", berr)
		} else if exhausted {
			sse(c, "budget", budgetPayload(used, limit))
			return
		}
	}

	message, title, tokensCSS, appTSX, err := a.generateChecked(ctx, c, llm, req.Prompt, newStepper(c), a.meterShared(shared))
	if err != nil {
		log.Printf("generate error: %v", err)
		sse(c, "error", gin.H{"error": "we couldn't build a working deck — please try again."})
		return
	}
	if title == "" {
		title = "Untitled deck"
	}

	deck := &Deck{
		Owner: "anon", Title: title, Prompt: req.Prompt,
		AppTSX: appTSX, TokensCSS: tokensCSS, IsPublic: true,
	}
	if err := a.store.CreateDeck(ctx, deck); err != nil {
		log.Printf("store error: %v", err)
		sse(c, "error", gin.H{"error": "failed to save deck"})
		return
	}

	sse(c, "done", gin.H{
		"id":        deck.ID,
		"title":     deck.Title,
		"message":   message,
		"appTsx":    deck.AppTSX,
		"tokensCss": deck.TokensCSS,
	})
}

// POST /api/decks/:id/edit — apply an instruction to an existing deck, streaming
// the reworked design, compile-checking + retrying, then overwriting the deck.
func (a *API) editDeck(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	var req editReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Instruction == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "instruction is required"})
		return
	}

	ctx := c.Request.Context()
	deck, err := a.store.GetDeck(ctx, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	sseHeaders(c)

	llm, shared := a.llmFor(c)
	if shared {
		if used, limit, exhausted, berr := a.sharedBudget(ctx); berr != nil {
			log.Printf("shared budget check: %v", berr)
		} else if exhausted {
			sse(c, "budget", budgetPayload(used, limit))
			return
		}
	}

	prompt := editPrompt(req.Instruction, req.AppTSX, req.TokensCSS)
	message, title, tokensCSS, appTSX, err := a.generateChecked(ctx, c, llm, prompt, newStepper(c), a.meterShared(shared))
	if err != nil {
		log.Printf("edit error: %v", err)
		sse(c, "error", gin.H{"error": "we couldn't apply that change — please try again."})
		return
	}
	if title != "" {
		deck.Title = title
	}
	deck.AppTSX = appTSX
	deck.TokensCSS = tokensCSS

	if err := a.store.UpdateDeck(ctx, deck); err != nil {
		log.Printf("store error: %v", err)
		sse(c, "error", gin.H{"error": "failed to save deck"})
		return
	}

	sse(c, "done", gin.H{
		"id":        deck.ID,
		"title":     deck.Title,
		"message":   message,
		"appTsx":    deck.AppTSX,
		"tokensCss": deck.TokensCSS,
	})
}

// GET /api/decks — public gallery.
func (a *API) list(c *gin.Context) {
	decks, err := a.store.ListPublicDecks(c.Request.Context(), 60)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"decks": decks})
}

// GET /api/decks/:id — full deck.
func (a *API) get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	deck, err := a.store.GetDeck(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, deck)
}

// GET /api/shared-budget — how much of the shared key's allowance is left, so
// the UI can warn before a generation rather than only after one is refused.
// `capped: false` means this deployment runs uncapped (SHARED_TOKEN_CAP=0).
func (a *API) sharedBudgetStatus(c *gin.Context) {
	used, limit, _, err := a.sharedBudget(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not read budget"})
		return
	}
	if limit <= 0 {
		c.JSON(http.StatusOK, gin.H{"capped": false})
		return
	}
	p := budgetPayload(used, limit)
	p["capped"] = true
	c.JSON(http.StatusOK, p)
}
