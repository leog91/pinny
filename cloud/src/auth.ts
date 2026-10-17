export const sessionSeconds = 30 * 24 * 60 * 60;
export const cookieName = '__Host-pinny-session';
export async function hash(value: string) {
  return [...new Uint8Array(await crypto.subtle.digest('SHA-256', new TextEncoder().encode(value)))].map(b => b.toString(16).padStart(2, '0')).join('');
}
export function sessionToken(request: Request) {
  const value = request.headers.get('Cookie')?.split(';').map(s => s.trim()).find(s => s.startsWith(cookieName + '='))?.slice(cookieName.length + 1);
  return value && /^[0-9a-f]{64}$/.test(value) ? value : null;
}
export function sessionCookie(token: string, seconds = sessionSeconds) {
  return `${cookieName}=${token}; Path=/; Max-Age=${seconds}; Expires=${new Date(Date.now() + seconds * 1000).toUTCString()}; Secure; HttpOnly; SameSite=Strict`;
}
export async function sessionValid(request: Request, db: D1Database, credential: string) {
  const token = sessionToken(request);
  if (!token || !credential || credential.length < 32) return false;
  return !!await db.prepare('SELECT token_hash FROM device_sessions WHERE token_hash = ? AND credential_hash = ? AND expires_at > ?')
    .bind(await hash(token), await hash(credential), Date.now()).first();
}
export function sameOrigin(request: Request) {
  return request.headers.get('Origin') === new URL(request.url).origin && !['cross-site', 'same-site'].includes(request.headers.get('Sec-Fetch-Site') ?? '');
}
