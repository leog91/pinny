import { page, script } from './website';

export interface Env { DB: D1Database; CAPTURE_TOKEN: string; SYNC_TOKEN: string }
export const idPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
const headers = {
  'Cache-Control': 'no-store', 'X-Content-Type-Options': 'nosniff',
  'Referrer-Policy': 'no-referrer',
  'Content-Security-Policy': "default-src 'none'; script-src 'self'; style-src 'unsafe-inline'; connect-src 'self'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'",
};
function json(value: unknown, status = 200) {
  return Response.json(value, { status, headers });
}
async function authorized(request: Request, secret?: string) {
  if (!secret || secret.length < 32) return false;
  const value = request.headers.get('Authorization') ?? '';
  // Compare fixed-size digests, never log the credential or body.
  const digest = async (s: string) => new Uint8Array(await crypto.subtle.digest('SHA-256', new TextEncoder().encode(s)));
  const [a, b] = await Promise.all([digest(value), digest(`Bearer ${secret}`)]);
  return a.reduce((diff, byte, i) => diff | (byte ^ b[i]), 0) === 0;
}
async function body(request: Request): Promise<Record<string, unknown>> {
  if (!request.headers.get('Content-Type')?.startsWith('application/json')) throw new Error('invalid');
  // Bound the stream, including chunked requests, before parsing.
  const reader = request.body?.getReader();
  if (!reader) throw new Error('invalid');
  const chunks: Uint8Array[] = []; let size = 0;
  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    size += value.length;
    if (size > 70000) { await reader.cancel(); throw new Error('invalid'); }
    chunks.push(value);
  }
  const bytes = new Uint8Array(size); let offset = 0;
  for (const chunk of chunks) { bytes.set(chunk, offset); offset += chunk.length; }
  const data = JSON.parse(new TextDecoder('utf-8', { fatal: true }).decode(bytes));
  if (!data || typeof data !== 'object' || Array.isArray(data)) throw new Error('invalid');
  return data;
}
export default {
  async fetch(request: Request, env: Env): Promise<Response> {
    const url = new URL(request.url);
    if (request.method === 'GET' && (url.pathname === '/' || url.pathname === '/capture.js')) {
      return new Response(url.pathname === '/' ? page : script, {
        headers: { ...headers, 'Content-Type': url.pathname === '/' ? 'text/html; charset=utf-8' : 'text/javascript; charset=utf-8' },
      });
    }
    const capture = url.pathname === '/api/v1/items' && request.method === 'POST';
    const pending = url.pathname === '/api/v1/pending' && request.method === 'GET';
    const ack = url.pathname === '/api/v1/ack' && request.method === 'POST';
    if (!capture && !pending && !ack) return json({ error: 'not_found' }, 404);
    if (!await authorized(request, capture ? env.CAPTURE_TOKEN : env.SYNC_TOKEN)) return json({ error: 'unauthorized' }, 401);
    // No cookies or CORS; reject browser cross-origin submission even with credentials.
    const origin = request.headers.get('Origin');
    if (origin && origin !== url.origin) return json({ error: 'forbidden' }, 403);
    try {
      if (pending) {
        const result = await env.DB.prepare(`SELECT id, kind, text, title, completed, created_at AS createdAt FROM inbox
          WHERE delivered_at IS NULL ORDER BY created_at, id LIMIT 50`).all();
        return json({ items: result.results.map(row => ({ ...row, completed: !!row.completed })) });
      }
      let data: Record<string, unknown>;
      try { data = await body(request); } catch { return json({ error: 'invalid_request' }, 400); }
      if (capture) {
        const { id, kind, text, title = '', completed = false } = data;
        if (typeof id !== 'string' || !idPattern.test(id) || typeof kind !== 'string' || !['note', 'todo'].includes(kind) ||
          typeof text !== 'string' || !text.trim() || new TextEncoder().encode(text).length > 16000 || text.includes('\0') ||
          typeof title !== 'string' || [...title].length > 100 || /[\r\n\0]/.test(title) ||
          typeof completed !== 'boolean' || (kind === 'note' && completed)) return json({ error: 'invalid_request' }, 400);
        await env.DB.prepare('INSERT OR IGNORE INTO inbox(id, kind, text, title, completed) VALUES (?, ?, ?, ?, ?)')
          .bind(id, kind, text, title, Number(completed)).run();
        const row = await env.DB.prepare('SELECT kind, text, title, completed FROM inbox WHERE id = ?').bind(id).first();
        if (!row || row.kind !== kind || row.text !== text || row.title !== title || !!row.completed !== completed)
          return json({ error: 'id_conflict' }, 409);
        return json({ id }, 200);
      }
      const ids = data.ids;
      if (!Array.isArray(ids) || ids.length < 1 || ids.length > 50 || ids.some(id => typeof id !== 'string' || !idPattern.test(id)))
        return json({ error: 'invalid_request' }, 400);
      // Retain IDs and contents for idempotency comparison, including after delivery.
      await env.DB.prepare(`UPDATE inbox SET delivered_at = COALESCE(delivered_at, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
        WHERE id IN (${ids.map(() => '?').join(',')})`).bind(...ids).run();
      return json({ ok: true });
    } catch {
      return json({ error: 'temporarily_unavailable' }, 503);
    }
  },
} satisfies ExportedHandler<Env>;
