import { useEffect, useRef, useState, useCallback } from 'react';
import { useSearch, Link } from '@tanstack/react-router';
import { Aperture } from 'lucide-react';
import Logo from '../components/Logo';
import PromptBox from '../components/PromptBox';
import DeckPreview from '../components/DeckPreview';
import StepGroup from '../components/StepGroup';
import ShareModal from '../components/ShareModal';
import BudgetModal from '../components/BudgetModal';
import { generateDeck, editDeck, getDeck, getHealth } from '../lib/api';
import type { GenerateHandlers, SharedBudget, Step } from '../lib/api';
import { hasProviderKey } from '../lib/keys';

interface Msg {
  role: 'user' | 'assistant';
  text: string;
  streaming?: boolean;
  steps?: Step[];
}

interface DeckState {
  id: string;
  title: string;
  appTsx: string;
  tokensCss: string;
}

// strip the fenced code + TITLE line so chat shows only the model's prose
function chatText(raw: string): string {
  let t = raw;
  const f = t.indexOf('```');
  if (f >= 0) t = t.slice(0, f);
  const m = t.search(/^\s*TITLE:/m);
  if (m >= 0) t = t.slice(0, m);
  return t.trim();
}

export default function Builder() {
  const { prompt, deckId } = useSearch({ from: '/build' });

  const [messages, setMessages] = useState<Msg[]>([]);
  const [busy, setBusy] = useState(false);
  const [deck, setDeck] = useState<DeckState | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [expanded, setExpanded] = useState(false);
  const [sharing, setSharing] = useState(false);
  const [budget, setBudget] = useState<SharedBudget | null>(null);
  // True only when the server has no provider key AND the visitor hasn't supplied
  // one — i.e. every deck here is the fixed sample template. A visitor's own key
  // overrides the server's client per-request (see llmFor), so holding one means
  // generation is real regardless of how the server booted.
  const [sampleMode, setSampleMode] = useState(false);

  const scrollRef = useRef<HTMLDivElement>(null);
  const started = useRef(false); // guard StrictMode double-run
  // The call the shared-key cap refused, replayed verbatim once the visitor
  // supplies their own key — so a refusal costs them their place, not their prompt.
  const refused = useRef<(() => void) | null>(null);

  // Drive a generate/edit SSE call, wiring streaming prose + the final deck into
  // chat + preview. `p` is the prompt shown as the user message; `stream` performs
  // the actual API call with the shared handlers.
  const runStream = useCallback(
    async (p: string, placeholder: string, stream: (h: GenerateHandlers) => Promise<void>) => {
      setBusy(true);
      setError(null);
      setMessages((m) => [
        ...m,
        { role: 'user', text: p },
        { role: 'assistant', text: '', streaming: true, steps: [] },
      ]);

      // merge a step into the streaming assistant message's step list by id
      const mergeStep = (step: Step) => {
        setMessages((m) => {
          const copy = [...m];
          const last = copy[copy.length - 1];
          const steps = last.steps ?? [];
          const i = steps.findIndex((s) => s.id === step.id);
          const nextSteps = i === -1 ? [...steps, step] : steps.map((s, j) => (j === i ? step : s));
          copy[copy.length - 1] = { ...last, steps: nextSteps };
          return copy;
        });
      };

      let raw = '';
      await stream({
        onDelta: (d) => {
          raw += d;
          const shown = chatText(raw) || placeholder;
          setMessages((m) => {
            const copy = [...m];
            copy[copy.length - 1] = { ...copy[copy.length - 1], text: shown, streaming: true };
            return copy;
          });
        },
        onStep: mergeStep,
        onDone: (res) => {
          setDeck({ id: res.id, title: res.title, appTsx: res.appTsx, tokensCss: res.tokensCss });
          setMessages((m) => {
            const copy = [...m];
            copy[copy.length - 1] = {
              ...copy[copy.length - 1],
              text: res.message || 'Ready for the stage.',
              streaming: false,
            };
            return copy;
          });
          setBusy(false);
        },
        onBudgetExhausted: (b) => {
          // Nothing was generated and nothing was charged. Stash the exact call
          // so BudgetModal's "save key and retry" can replay it.
          refused.current = () => void runStreamRef.current?.(p, placeholder, stream);
          setBudget(b);
          setMessages((m) => {
            const copy = [...m];
            copy[copy.length - 1] = {
              ...copy[copy.length - 1],
              text: '⚠️ The shared key has run out of tokens.',
              streaming: false,
            };
            return copy;
          });
          setBusy(false);
        },
        onError: (msg) => {
          setError(msg);
          setMessages((m) => {
            const copy = [...m];
            copy[copy.length - 1] = { ...copy[copy.length - 1], text: `⚠️ ${msg}`, streaming: false };
            return copy;
          });
          setBusy(false);
        },
      });
    },
    [],
  );

  // runStream replays itself on BYOK retry; a ref avoids a self-referential
  // useCallback dependency.
  const runStreamRef = useRef(runStream);
  runStreamRef.current = runStream;

  const run = useCallback(
    (p: string) => runStream(p, 'Designing your deck…', (h) => generateDeck(p, h)),
    [runStream],
  );

  // initial: load an existing deck, or auto-run the landing prompt (once)
  useEffect(() => {
    if (started.current) return;
    started.current = true;

    if (deckId) {
      getDeck(deckId)
        .then((d) => {
          setDeck({ id: d.id, title: d.title, appTsx: d.appTsx, tokensCss: d.tokensCss });
          setMessages([
            { role: 'user', text: d.prompt },
            { role: 'assistant', text: `Loaded “${d.title}”. Ask for changes or start a new deck.` },
          ]);
        })
        .catch(() => setError('Could not load that deck.'));
    } else if (prompt) {
      run(prompt);
    }
  }, [deckId, prompt, run]);

  useEffect(() => {
    scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight, behavior: 'smooth' });
  }, [messages]);

  // Ask the server what it's actually running. Re-checked when the budget modal
  // closes, since saving a key there flips this off. A failed probe leaves the
  // notice hidden: guessing "sample mode" on a network blip would be worse than
  // saying nothing.
  useEffect(() => {
    let live = true;
    getHealth()
      .then((h) => {
        if (!live) return;
        const byok = hasProviderKey('openai') || hasProviderKey('anthropic');
        setSampleMode(h.llm === 'canned' && !byok);
      })
      .catch(() => {});
    return () => {
      live = false;
    };
  }, [budget]);

  const onSubmit = (p: string) => {
    // follow-up on a loaded deck → edit in place (hot-swaps into the live preview,
    // preserving the current slide); no deck yet → fresh generation
    if (deck) {
      const current = { appTsx: deck.appTsx, tokensCss: deck.tokensCss };
      runStream(p, 'Updating your deck…', (h) => editDeck(deck.id, p, current, h));
    } else {
      run(p);
    }
  };

  return (
    <div style={{ height: '100vh', display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
      {/* top bar */}
      <header
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: '10px 16px',
          borderBottom: '1px solid var(--border)',
          flexShrink: 0,
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: 14 }}>
          <Logo size={17} />
          <span style={{ color: 'var(--border-2)' }}>/</span>
          <span style={{ fontSize: 14, color: 'var(--fg-muted)', maxWidth: 320, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
            {deck?.title ?? 'New deck'}
          </span>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
          <button className="btn btn-ghost" disabled={!deck} onClick={() => setSharing(true)}>
            Share
          </button>
          <button className="btn btn-primary" disabled={!deck} onClick={() => setSharing(true)}>
            Publish
          </button>
          <div style={{ width: 1, height: 22, background: 'var(--border)', margin: '0 2px' }} />
          <Link to="/dashboard" className="btn btn-ghost">
            Settings
          </Link>
        </div>
      </header>

      {/* split */}
      <div style={{ flex: 1, display: 'flex', minHeight: 0 }}>
        {/* chat */}
        <aside
          style={{
            width: 'clamp(340px, 34%, 460px)',
            borderRight: '1px solid var(--border)',
            display: 'flex',
            flexDirection: 'column',
            minHeight: 0,
            background: 'var(--bg-1)',
          }}
        >
          {sampleMode && (
            <div
              style={{
                display: 'flex',
                alignItems: 'baseline',
                gap: 10,
                padding: '10px 18px',
                borderBottom: '1px solid var(--border)',
                background: 'var(--bg-2)',
                fontSize: 13,
                lineHeight: 1.5,
                color: 'var(--fg-muted)',
              }}
            >
              <span style={{ color: 'var(--accent)', flexShrink: 0 }}>Sample mode</span>
              <span>
                This deployment has no API key, so decks are a fixed template — not written
                from your prompt.{' '}
                <Link to="/dashboard" style={{ color: 'var(--fg)', textDecoration: 'underline' }}>
                  Add your key
                </Link>{' '}
                to generate for real.
              </span>
            </div>
          )}

          <div ref={scrollRef} style={{ flex: 1, overflowY: 'auto', padding: '20px 18px' }}>
            {messages.length === 0 && (
              <div style={{ color: 'var(--fg-dim)', fontSize: 14, marginTop: 20 }}>
                Describe the deck you want. Cue will build it on the right.
              </div>
            )}
            <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
              {messages.map((m, i) => (
                <ChatBubble key={i} msg={m} />
              ))}
            </div>
          </div>
          <div style={{ padding: 14, borderTop: '1px solid var(--border)' }}>
            <PromptBox onSubmit={onSubmit} compact busy={busy} placeholder="Ask for a new deck…" />
          </div>
        </aside>

        {/* preview */}
        <main style={{ flex: 1, display: 'flex', flexDirection: 'column', minWidth: 0, background: 'var(--bg-2)' }}>
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              padding: '8px 14px',
              borderBottom: '1px solid var(--border)',
              flexShrink: 0,
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 13, color: 'var(--fg-muted)' }}>
              <span style={{ width: 8, height: 8, borderRadius: 999, background: deck ? 'var(--accent)' : 'var(--border-2)' }} />
              {busy ? 'Taking the cue…' : deck ? 'Preview' : 'Idle'}
            </div>
            {deck && (
              <button className="btn btn-ghost" onClick={() => setExpanded(true)} style={{ padding: '6px 12px' }}>
                ⛶ Full screen
              </button>
            )}
          </div>

          <div style={{ flex: 1, minHeight: 0, position: 'relative' }}>
            {deck ? (
              <DeckPreview appTsx={deck.appTsx} tokensCss={deck.tokensCss} title={deck.title} />
            ) : (
              <EmptyPreview busy={busy} error={error} />
            )}
          </div>
        </main>
      </div>

      {/* fullscreen preview overlay */}
      {expanded && deck && (
        <div style={{ position: 'fixed', inset: 0, zIndex: 50, background: '#000' }}>
          <button
            onClick={() => setExpanded(false)}
            className="btn btn-ghost"
            style={{ position: 'absolute', top: 14, right: 14, zIndex: 51 }}
          >
            ✕ Close
          </button>
          <DeckPreview appTsx={deck.appTsx} tokensCss={deck.tokensCss} title={deck.title} />
        </div>
      )}

      {sharing && deck && (
        <ShareModal
          url={`${window.location.origin}/build?deckId=${deck.id}`}
          title={deck.title}
          onClose={() => setSharing(false)}
        />
      )}

      {budget && (
        <BudgetModal
          budget={budget}
          onKeySaved={() => {
            setBudget(null);
            const replay = refused.current;
            refused.current = null;
            replay?.();
          }}
          onClose={() => setBudget(null)}
        />
      )}
    </div>
  );
}

function ChatBubble({ msg }: { msg: Msg }) {
  const isUser = msg.role === 'user';
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 6, alignItems: isUser ? 'flex-end' : 'flex-start' }}>
      {!isUser && (
        <div style={{ fontSize: 12, fontWeight: 600, color: 'var(--fg-dim)', display: 'flex', alignItems: 'center', gap: 6 }}>
          <span
            aria-hidden
            style={{
              display: 'inline-grid',
              placeItems: 'center',
              width: 16,
              height: 16,
              borderRadius: 5,
              background: 'var(--accent)',
              flexShrink: 0,
            }}
          >
            <Aperture size={11} color="#fff" strokeWidth={2.25} />
          </span>
          Cue
        </div>
      )}
      {!isUser && msg.steps && msg.steps.length > 0 && (
        <div style={{ width: '100%', maxWidth: '92%' }}>
          <StepGroup steps={msg.steps} />
        </div>
      )}
      <div
        style={{
          maxWidth: '92%',
          fontSize: 14.5,
          lineHeight: 1.55,
          padding: isUser ? '10px 14px' : 0,
          borderRadius: 14,
          background: isUser ? 'var(--surface-2)' : 'transparent',
          border: isUser ? '1px solid var(--border)' : 'none',
          color: isUser ? 'var(--fg)' : 'var(--fg-muted)',
          whiteSpace: 'pre-wrap',
        }}
      >
        {msg.text}
        {msg.streaming && <span className="spinner" style={{ display: 'inline-block', marginLeft: 8, verticalAlign: 'middle' }} />}
      </div>
    </div>
  );
}

function EmptyPreview({ busy, error }: { busy: boolean; error: string | null }) {
  return (
    <div
      style={{
        position: 'absolute',
        inset: 0,
        display: 'grid',
        placeItems: 'center',
        color: 'var(--fg-dim)',
        textAlign: 'center',
        padding: 24,
      }}
    >
      <div>
        <div
          style={{
            display: 'inline-grid',
            placeItems: 'center',
            width: 52,
            height: 52,
            borderRadius: 14,
            background: 'var(--accent)',
            opacity: 0.5,
            marginBottom: 14,
          }}
        >
          <Aperture size={28} color="#fff" strokeWidth={2} />
        </div>
        {error ? (
          <div style={{ color: 'var(--error)', fontSize: 14, maxWidth: 360 }}>{error}</div>
        ) : busy ? (
          <div style={{ display: 'flex', alignItems: 'center', gap: 10, fontSize: 14 }}>
            <span className="spinner" /> Taking the cue…
          </div>
        ) : (
          <>
            <div style={{ fontSize: 15, color: 'var(--fg-muted)' }}>Your Cue deck will show here.</div>
            <div style={{ fontSize: 13, marginTop: 6 }}>
              <Link to="/" style={{ color: 'var(--accent)' }}>Start from a prompt →</Link>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
