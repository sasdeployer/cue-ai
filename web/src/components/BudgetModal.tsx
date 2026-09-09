import { useState } from 'react';
import { Link } from '@tanstack/react-router';
import { setProviderKey } from '../lib/keys';
import type { SharedBudget } from '../lib/api';

interface Props {
  budget: SharedBudget;
  // Called after a key is saved, so the caller can retry the generation that
  // was refused. The prompt is still on screen — the point of taking the key
  // here rather than on /dashboard is that nothing the visitor typed is lost.
  onKeySaved: () => void;
  onClose: () => void;
}

const fmt = (n?: number) => (n ?? 0).toLocaleString();

/**
 * Shown when the shared demo key has spent its token allowance.
 *
 * This is NOT a paywall and there is no plan to upgrade to — Cue has no
 * accounts and no billing (see CLAUDE.md). The shared key belongs to whoever
 * deployed this instance, so the only thing on offer is BYOK: plug in your own
 * provider key and carry on, unmetered.
 */
export default function BudgetModal({ budget, onKeySaved, onClose }: Props) {
  const [input, setInput] = useState('');
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const save = async () => {
    const v = input.trim();
    if (!v) return;
    setBusy(true);
    setErr(null);
    try {
      await setProviderKey('openai', v);
      onKeySaved();
    } catch {
      setErr('Could not save that key in this browser.');
      setBusy(false);
    }
  };

  return (
    <div
      onClick={onClose}
      style={{
        position: 'fixed',
        inset: 0,
        background: 'rgba(0,0,0,0.6)',
        display: 'grid',
        placeItems: 'center',
        zIndex: 50,
      }}
    >
      <div
        onClick={(e) => e.stopPropagation()}
        style={{
          width: 460,
          maxWidth: '90vw',
          background: 'var(--surface)',
          border: '1px solid var(--border)',
          borderRadius: 16,
          padding: 24,
          boxShadow: '0 30px 80px -20px rgba(0,0,0,0.6)',
        }}
      >
        <div style={{ fontSize: 16, fontWeight: 600, marginBottom: 6 }}>
          The shared key is used up
        </div>
        <div
          style={{
            fontSize: 13.5,
            color: 'var(--fg-muted)',
            marginBottom: 18,
            lineHeight: 1.55,
          }}
        >
          Cue runs on a shared key so you can try it with no setup, and its{' '}
          {fmt(budget.cap)}-token allowance is now spent. Add your own OpenAI key to keep
          going — it stays encrypted in this browser, is sent only with your own
          generations, and is never stored on the server.
        </div>

        <input
          autoFocus
          type="password"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') save();
          }}
          placeholder="sk-…"
          style={{
            width: '100%',
            boxSizing: 'border-box',
            padding: '10px 12px',
            borderRadius: 8,
            border: '1px solid var(--border-2)',
            background: 'var(--bg)',
            color: 'var(--fg)',
            fontSize: 13,
            fontFamily: 'var(--mono)',
          }}
        />
        {err && (
          <div style={{ fontSize: 12.5, color: 'var(--danger, #ff6b6b)', marginTop: 8 }}>{err}</div>
        )}

        <button
          className="btn btn-primary"
          onClick={save}
          disabled={busy || !input.trim()}
          style={{ marginTop: 14, width: '100%', justifyContent: 'center' }}
        >
          {busy ? 'Saving…' : 'Save key and retry'}
        </button>

        <div
          style={{
            marginTop: 14,
            fontSize: 12.5,
            color: 'var(--fg-dim)',
            textAlign: 'center',
            lineHeight: 1.5,
          }}
        >
          Prefer Anthropic, or want to manage keys?{' '}
          <Link to="/dashboard" style={{ color: 'var(--accent)' }}>
            Open Settings
          </Link>
        </div>

        <button
          className="btn btn-ghost"
          onClick={onClose}
          style={{ marginTop: 12, width: '100%', justifyContent: 'center' }}
        >
          Not now
        </button>
      </div>
    </div>
  );
}
