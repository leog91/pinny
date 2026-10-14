import { SELF, env } from 'cloudflare:test';
import { describe, it, expect } from 'vitest';
import worker from '../src/index';
const id = '12345678-1234-4234-8234-123456789abc';
const item = { id, kind: 'todo', text: 'Buy milk', title: 'Shopping', completed: false };
function call(path: string, token: string, data?: unknown) {
  return SELF.fetch(`https://pinny.test/api/v1/${path}`, {
    method: data === undefined ? 'GET' : 'POST',
    headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
    body: data === undefined ? undefined : JSON.stringify(data),
  });
}
describe('D1 capture inbox', () => {
  it('requires separate credentials and rejects cross-origin submissions', async () => {
    expect((await call('items', '', item)).status).toBe(401);
    expect((await call('items', env.SYNC_TOKEN, item)).status).toBe(401);
    expect((await call('pending', env.CAPTURE_TOKEN)).status).toBe(401);
    expect((await call('ack', env.CAPTURE_TOKEN, { ids: [id] })).status).toBe(401);
    expect((await SELF.fetch('https://pinny.test/api/v1/items', { method: 'POST',
      headers: { Authorization: `Bearer ${env.CAPTURE_TOKEN}`, Origin: 'https://evil.test' } })).status).toBe(403);
  });
  it('retains offline items, retries idempotently, and acknowledges repeatedly', async () => {
    expect((await call('items', env.CAPTURE_TOKEN, item)).status).toBe(200);
    const retries = await Promise.all(Array.from({ length: 8 }, () => call('items', env.CAPTURE_TOKEN, item)));
    expect(retries.every(response => response.status === 200)).toBe(true);
    expect((await call('items', env.CAPTURE_TOKEN, item)).status).toBe(200);
    expect((await call('items', env.CAPTURE_TOKEN, { ...item, text: 'changed' })).status).toBe(409);
    for (let n = 0; n < 3; n++) {
      const body = await (await call('pending', env.SYNC_TOKEN)).json<{ items: unknown[] }>();
      expect(body.items).toHaveLength(1);
      expect(body.items[0]).toMatchObject(item);
    }
    for (let n = 0; n < 2; n++) expect((await call('ack', env.SYNC_TOKEN, { ids: [id] })).status).toBe(200);
    expect(await (await call('pending', env.SYNC_TOKEN)).json()).toEqual({ items: [] });
    expect((await call('items', env.CAPTURE_TOKEN, item)).status).toBe(200);
    expect(await (await call('pending', env.SYNC_TOKEN)).json()).toEqual({ items: [] });
  });
  it('validates payloads without storing partial or malformed items', async () => {
    for (const invalid of [ { ...item, id: "' OR 1=1" }, { ...item, text: '' }, { ...item, kind: 'unknown' },
      { ...item, text: 'x'.repeat(16001) }, { ...item, title: 'x'.repeat(101) }, { ...item, text: '\0' },
      { ...item, kind: 'note', completed: true }, { ...item, text: 'x'.repeat(70001) } ]) {
      expect((await call('items', env.CAPTURE_TOKEN, invalid)).status).toBe(400);
    }
    expect((await call('ack', env.SYNC_TOKEN, { ids: [] })).status).toBe(400);
    expect((await call('ack', env.SYNC_TOKEN, { ids: ["' OR 1=1"] })).status).toBe(400);
    expect(await (await call('pending', env.SYNC_TOKEN)).json()).toEqual({ items: [] });
  });
  it('bounds pending batches and delivers subsequent items after acknowledgment', async () => {
    await env.DB.batch(Array.from({ length: 55 }, (_, n) => env.DB.prepare(
      'INSERT INTO inbox(id, kind, text, title, completed) VALUES (?, ?, ?, ?, ?)'
    ).bind(`12345678-1234-4234-8234-${String(n).padStart(12, '0')}`, 'note', 'test', '', 0)));
    const first = await (await call('pending', env.SYNC_TOKEN)).json<{ items: { id: string }[] }>();
    expect(first.items).toHaveLength(50);
    expect((await call('ack', env.SYNC_TOKEN, { ids: first.items.map(i => i.id) })).status).toBe(200);
    const second = await (await call('pending', env.SYNC_TOKEN)).json<{ items: unknown[] }>();
    expect(second.items).toHaveLength(5);
  });
  it('serves a mobile form without embedded secrets or cacheable content', async () => {
    const response = await SELF.fetch('https://pinny.test/');
    const html = await response.text();
    expect(html).toContain('width=device-width');
    expect(html).not.toContain(env.CAPTURE_TOKEN);
    expect(html).not.toContain(env.SYNC_TOKEN);
    expect(response.headers.get('Cache-Control')).toBe('no-store');
    expect(response.headers.get('Content-Security-Policy')).toContain("frame-ancestors 'none'");
  });
  it('returns a redacted failure when database access fails', async () => {
    const request = new Request('https://pinny.test/api/v1/pending', { headers: { Authorization: `Bearer ${env.SYNC_TOKEN}` } });
    const response = await worker.fetch(request, { ...env, DB: {
      prepare() { throw new Error('private database diagnostics and note content'); },
    } as unknown as D1Database });
    expect(response.status).toBe(503);
    expect(await response.json()).toEqual({ error: 'temporarily_unavailable' });
  });
});
