import { byokHeaders } from './keys';

// Same-origin by default in a production build: the Go server serves this
// bundle out of ./dist (STATIC_DIR) and answers /api/* on the same host, so a
// relative base is correct on Nexlayer and anywhere else it's deployed.
//
// It must NOT fall back to localhost:8080 in a production build. VITE_API_BASE
// is not set at build time (not in the Dockerfile, not in CI), so that default
// shipped in the bundle and every deployed visitor's browser called port 8080
// on THEIR OWN machine — ERR_CONNECTION_REFUSED for every API call, on every
// host. The page still loaded (static assets are same-origin), so it looked
// like generation was slow or broken rather than never leaving the browser.
//
// Dev is the only case that needs an absolute base: `dev.sh` serves the app
// from Vite on :5273 while the API stays on :8080.
export const API_BASE =
  import.meta.env.VITE_API_BASE ?? (import.meta.env.DEV ? 'http://localhost:8080' : '');

export interface DeckSummary {
  id: string;
  title: string;
  prompt: string;
  appTsx: string;
  tokensCss: string;
  createdAt: string;
}

export interface Deck {
  id: string;
  owner: string;
  title: string;
  prompt: string;
  appTsx: string;
  tokensCss: string;
  isPublic: boolean;
  createdAt: string;
}

export interface GenerateResult {
  id: string;
  title: string;
  message: string;
  appTsx: string;
  tokensCss: string;
}

// Mirrors server/agent.go's Step struct. kind: read | search | write | check |
// fix | think. status: start (in progress) | ok | error. The same id is
// re-sent as status changes; callers merge by id.
export interface Step {
  id: string;
  kind: string;
  label: string;
  target?: string;
  status: 'start' | 'ok' | 'error';
}

export async function listDecks(): Promise<DeckSummary[]> {
  const r = await fetch(`${API_BASE}/api/decks`);
  if (!r.ok) throw new Error('failed to load gallery');
  const data = await r.json();
  return data.decks ?? [];
}

export async function getDeck(id: string): Promise<Deck> {
  const r = await fetch(`${API_BASE}/api/decks/${id}`);
  if (!r.ok) throw new Error('deck not found');
  return r.json();
}

// Health reports which LLM this deployment is actually configured to use.
// `llm` is "canned" when no provider key is set server-side, which means every
// generation returns a FIXED SAMPLE TEMPLATE rather than an AI-authored deck.
// The UI has to surface that: a template streamed into the same chat panel is
// otherwise indistinguishable from real generation, and reads as "the AI is
// slow" rather than "there is no AI configured".
export interface Health {
  ok: boolean;
  llm: string;
}

export async function getHealth(): Promise<Health> {
  const r = await fetch(`${API_BASE}/api/health`);
  if (!r.ok) throw new Error('failed to load health');
  return r.json();
}

// SharedBudget is the state of the shared demo key's token allowance. The
// server runs on its own key so Cue is usable with zero setup; once that
// allowance is spent, generation stops and the visitor is asked to bring their
// own key (BYOK) instead. `capped: false` means this deployment set
// SHARED_TOKEN_CAP=0 and has no limit — self-hosted/private, nothing to show.
export interface SharedBudget {
  capped: boolean;
  used?: number;
  cap?: number;
  remaining?: number;
  exhausted?: boolean;
}

export async function getSharedBudget(): Promise<SharedBudget> {
  const r = await fetch(`${API_BASE}/api/shared-budget`);
  if (!r.ok) throw new Error('failed to load budget');
  return r.json();
}

export interface GenerateHandlers {
  onDelta?: (text: string) => void;
  onStep?: (step: Step) => void;
  onDone?: (result: GenerateResult) => void;
  onError?: (message: string) => void;
  // Fired instead of onDone when the shared key is out of tokens. Nothing was
  // generated and nothing was charged — the caller should prompt for BYOK.
  onBudgetExhausted?: (budget: SharedBudget) => void;
  signal?: AbortSignal;
}

/**
 * POST a JSON body and stream the SSE response, dispatching delta / done / error
 * events to the handlers. Shared by generateDeck and editDeck.
 */
async function streamSSE(url: string, body: unknown, h: GenerateHandlers): Promise<void> {
  let resp: Response;
  try {
    resp = await fetch(url, {
      method: 'POST',
      headers: { 'content-type': 'application/json', ...(await byokHeaders()) },
      body: JSON.stringify(body),
      signal: h.signal,
    });
  } catch (e) {
    // A transport failure (DNS, refused connection, offline, CORS) rejects the
    // fetch rather than returning a response. Unhandled, that rejection left the
    // caller's spinner running forever with nothing in the UI — which is how a
    // bundle pointing at the wrong API host read as "generation is slow".
    // An aborted request is the caller's own doing, so stay quiet for it.
    if ((e as Error)?.name === 'AbortError') return;
    h.onError?.('Could not reach the Cue API. Check your connection and try again.');
    return;
  }

  if (!resp.ok || !resp.body) {
    let msg = 'generation failed';
    try {
      const j = await resp.json();
      msg = j.error ?? msg;
    } catch { /* ignore */ }
    h.onError?.(msg);
    return;
  }

  const reader = resp.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';

  const dispatch = (block: string) => {
    let event = 'message';
    let data = '';
    for (const line of block.split('\n')) {
      if (line.startsWith('event:')) event = line.slice(6).trim();
      else if (line.startsWith('data:')) data += line.slice(5).trim();
    }
    if (!data) return;
    let payload: any;
    try { payload = JSON.parse(data); } catch { return; }
    if (event === 'delta') h.onDelta?.(payload.text ?? '');
    else if (event === 'step') h.onStep?.(payload as Step);
    else if (event === 'done') h.onDone?.(payload as GenerateResult);
    else if (event === 'budget') h.onBudgetExhausted?.({ capped: true, ...payload });
    else if (event === 'error') h.onError?.(payload.error ?? 'error');
  };

  // eslint-disable-next-line no-constant-condition
  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });
    let idx: number;
    while ((idx = buffer.indexOf('\n\n')) !== -1) {
      const block = buffer.slice(0, idx);
      buffer = buffer.slice(idx + 2);
      dispatch(block);
    }
  }
  if (buffer.trim()) dispatch(buffer);
}

/**
 * Stream a deck generation. Reads the SSE response body and dispatches
 * delta / done / error events.
 */
export async function generateDeck(prompt: string, h: GenerateHandlers): Promise<void> {
  return streamSSE(`${API_BASE}/api/decks`, { prompt }, h);
}

/**
 * Stream an edit of an existing deck. Sends the current deck source so the model
 * can revise it; the SSE shape matches generateDeck.
 */
export async function editDeck(
  id: string,
  instruction: string,
  current: { appTsx: string; tokensCss: string },
  h: GenerateHandlers,
): Promise<void> {
  return streamSSE(
    `${API_BASE}/api/decks/${id}/edit`,
    { instruction, appTsx: current.appTsx, tokensCss: current.tokensCss },
    h,
  );
}
