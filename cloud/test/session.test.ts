import { SELF, env } from 'cloudflare:test';
import { describe, it, expect } from 'vitest';
import { hash, sessionSeconds } from '../src/auth';
import worker from '../src/index';

const origin = 'https://pinny.test';
function login(remember = true, token = env.CAPTURE_TOKEN, site: string | null = origin, cookie = '') {
  return SELF.fetch(origin + '/api/v1/session', { method: 'POST', headers: {
    Authorization: `Bearer ${token}`, 'Content-Type': 'application/json', Cookie: cookie, ...(site ? {Origin:site} : {}),
  }, body: JSON.stringify({ remember }) });
}
async function remembered() {
  const response = await login();
  expect(response.status).toBe(200);
  return response.headers.get('Set-Cookie')!.split(';')[0];
}
function capture(cookie: string, site: string | null = origin) {
  return SELF.fetch(origin + '/api/v1/items', { method: 'POST', headers: {
    Cookie:cookie, 'Content-Type':'application/json', ...(site ? {Origin:site} : {}),
  }, body:JSON.stringify({id:'12345678-1234-4234-8234-123456789abc',kind:'note',text:'session note'}) });
}
describe('device sessions', () => {
  it('requires the capture credential and exact Origin even at login', async () => {
    expect((await login(true, env.SYNC_TOKEN)).status).toBe(401);
    expect((await login(true, env.CAPTURE_TOKEN, 'https://evil.test')).status).toBe(403);
    expect((await login(true, env.CAPTURE_TOKEN, null)).status).toBe(403);
    const response = await login(false);
    expect(response.status).toBe(200);
    expect(response.headers.get('Set-Cookie')).toContain('Max-Age=0');
    expect((await env.DB.prepare('SELECT * FROM device_sessions').all()).results).toHaveLength(0);
  });
  it('issues random persistent host-only cookies and stores only hashes', async () => {
    const response = await login();
    const header = response.headers.get('Set-Cookie')!;
    expect(header).toMatch(/^__Host-pinny-session=[0-9a-f]{64};/);
    for (const attr of ['Path=/', 'Secure', 'HttpOnly', 'SameSite=Strict', `Max-Age=${sessionSeconds}`, 'Expires=']) expect(header).toContain(attr);
    expect(header).not.toMatch(/Domain=/i);
    expect(response.headers.get('Cache-Control')).toBe('no-store');
    const token = header.split(';')[0].split('=')[1];
    const row = await env.DB.prepare('SELECT * FROM device_sessions').first();
    expect(row?.token_hash).toBe(await hash(token));
    expect(row?.credential_hash).toBe(await hash(env.CAPTURE_TOKEN));
    expect(JSON.stringify(row)).not.toContain(token);
    const second = await remembered();
    expect(second).not.toBe(header.split(';')[0]);
  });
  it('allows capture but never desktop sync, and strictly blocks CSRF', async () => {
    const cookie = await remembered();
    expect((await capture(cookie)).status).toBe(200);
    expect((await capture(cookie, null)).status).toBe(403);
    expect((await capture(cookie, 'null')).status).toBe(403);
    expect((await capture(cookie, 'https://evil.test')).status).toBe(403);
    expect((await SELF.fetch(origin + '/api/v1/items', {method:'POST',headers:{Cookie:cookie,Origin:origin,'Sec-Fetch-Site':'cross-site'}})).status).toBe(403);
    expect((await SELF.fetch(origin + '/api/v1/pending', {headers:{Cookie:cookie}})).status).toBe(401);
    expect((await SELF.fetch(origin + '/api/v1/ack', {method:'POST',headers:{Cookie:cookie,Origin:origin}})).status).toBe(401);
    const state = await SELF.fetch(origin + '/api/v1/session', {headers:{Cookie:cookie}});
    expect(await state.json()).toEqual({authenticated:true});
    expect(state.headers.get('Cache-Control')).toBe('no-store');
    expect(state.headers.get('Access-Control-Allow-Origin')).toBeNull();
  });
  it('expires, revokes, rotates and invalidates sessions on credential rotation', async () => {
    const cookie = await remembered();
    const rotated = (await login(true, env.CAPTURE_TOKEN, origin, cookie)).headers.get('Set-Cookie')!.split(';')[0];
    expect((await capture(cookie)).status).toBe(401);
    expect((await capture(rotated)).status).toBe(200);
    const changedCredential = await worker.fetch(new Request(origin + '/api/v1/items', {method:'POST',headers:{Cookie:rotated,Origin:origin}}), {...env,CAPTURE_TOKEN:'different-capture-credential-at-least-32'});
    expect(changedCredential.status).toBe(401);
    expect((await SELF.fetch(origin + '/api/v1/session', {method:'DELETE',headers:{Cookie:rotated}})).status).toBe(403);
    const logout = await SELF.fetch(origin + '/api/v1/session', {method:'DELETE',headers:{Cookie:rotated,Origin:origin}});
    expect(logout.status).toBe(200);
    expect(logout.headers.get('Set-Cookie')).toContain('Max-Age=0');
    expect((await capture(rotated)).status).toBe(401);
    const expired = await remembered();
    await env.DB.prepare('UPDATE device_sessions SET expires_at = ?').bind(Date.now() - 1).run();
    expect((await capture(expired)).status).toBe(401);
  });
  it('switching to page-only login revokes the previous remembered session', async () => {
    const cookie = await remembered();
    const response = await login(false, env.CAPTURE_TOKEN, origin, cookie);
    expect(response.status).toBe(200);
    expect(response.headers.get('Set-Cookie')).toContain('Max-Age=0');
    expect((await capture(cookie)).status).toBe(401);
  });
});
