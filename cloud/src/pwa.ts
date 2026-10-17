export const manifest = JSON.stringify({
  id: '/', name: 'Pinny capture', short_name: 'Pinny', start_url: '/', scope: '/',
  display: 'standalone', background_color: '#f5f5f7', theme_color: '#2459ad',
  icons: [{ src: '/icon.svg', sizes: 'any', type: 'image/svg+xml', purpose: 'any maskable' }],
});
export const icon = `<svg xmlns="http://www.w3.org/2000/svg" width="512" height="512" viewBox="0 0 512 512"><rect width="512" height="512" rx="96" fill="#2459ad"/><path fill="#fff" d="M148 112h216v288H148z"/><path fill="#2459ad" d="M184 164h144v24H184zm0 64h144v24H184zm0 64h104v24H184z"/></svg>`;

// Bump the cache version whenever the shell changes. Updates wait for explicit consent.
export const serviceWorker = `
const CACHE = 'pinny-shell-v3';
const PUBLIC = ['/', '/capture.js', '/manifest.webmanifest', '/icon.svg'];
self.addEventListener('install', event => event.waitUntil((async () => {
  const cache = await caches.open(CACHE);
  await cache.addAll(PUBLIC.map(path => new Request(path, {credentials:'omit', cache:'reload'})));
})()));
self.addEventListener('activate', event => event.waitUntil((async () => {
  for (const key of await caches.keys()) if (key.startsWith('pinny-shell-') && key !== CACHE) await caches.delete(key);
  await self.clients.claim();
})()));
self.addEventListener('message', event => { if (event.data === 'ACTIVATE') self.skipWaiting(); });
self.addEventListener('fetch', event => {
  const url = new URL(event.request.url);
  if (event.request.method !== 'GET' || url.origin !== self.location.origin || url.search || !PUBLIC.includes(url.pathname)) return;
  event.respondWith((async () => {
    const cached = await (await caches.open(CACHE)).match(url.pathname);
    return cached || fetch(new Request(event.request, {credentials:'omit'}));
  })());
});
`;
