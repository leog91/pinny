export const page = `<!doctype html>
<html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="theme-color" content="#2459ad"><link rel="manifest" href="/manifest.webmanifest"><link rel="icon" href="/icon.svg">
<title>Pinny capture</title><style>
body{font:17px system-ui;margin:2rem auto;padding:0 1rem;max-width:36rem;background:#f5f5f7;color:#202328}
label{display:block;margin:1rem 0 .3rem}input,select,textarea,button{font:inherit;box-sizing:border-box;width:100%;padding:.7rem;border:1px solid #999;border-radius:.4rem}
input[type=checkbox]{width:auto}textarea{min-height:12rem}#status{min-height:3rem}[hidden]{display:none!important}
button{appearance:none;width:auto;min-width:9rem;min-height:48px;margin:1rem .5rem .3rem 0;padding:.75rem 1.5rem;border:1px solid #174786;border-radius:.6rem;background:#2459ad linear-gradient(#ffffff14,#00000008);color:white;font-weight:700;cursor:pointer;touch-action:manipulation;box-shadow:inset 0 1px 0 #ffffff40,0 3px 0 #163c77,0 5px 8px #162a4526;transition:background-color .12s,box-shadow .12s,transform .12s}
button.secondary{background:#fff;color:#24456f;border-color:#95a3b8;box-shadow:inset 0 1px 0 #fff,0 3px 0 #aab6c7,0 5px 8px #162a4514}
button.danger{color:#942d35;border-color:#bf8b90}
@media(hover:hover){button:not(:disabled):hover{background-color:#194b96;box-shadow:inset 0 1px 0 #ffffff30,0 3px 0 #102e60,0 6px 10px #162a4533}button.secondary:not(:disabled):hover{background-color:#eaf0f8;color:#18385f;box-shadow:0 3px 0 #8c9cb2,0 6px 10px #162a4526}button.danger:not(:disabled):hover{background-color:#fff0f0;color:#7d1f27}}
button:not(:disabled):active{transform:translateY(2px);box-shadow:inset 0 2px 3px #162a4526,0 1px 0 #163c77}
button:focus-visible{outline:3px solid #101d35;outline-offset:4px}
button:disabled{background:#d8e0eb;color:#465365;border-color:#aab6c7;box-shadow:none;transform:none;cursor:not-allowed}
@media(max-width:30rem){button{width:100%;margin-right:0}}
@media(prefers-reduced-motion:reduce){button{transition:none}}
</style><h1>Pinny capture</h1><p>Send to the cloud inbox. Your PC imports notes separately when sync runs.</p>
<p id="auth-state" role="status">Checking device login…</p>
<form id="login" action="/api/v1/session" method="post">
<label for="username">Login name (for your password manager)</label><input id="username" name="username" autocomplete="username" value="Pinny capture" readonly>
<label for="password">Capture credential</label><input id="password" name="password" type="password" required autocomplete="current-password">
<label><input id="remember" name="remember" type="checkbox"> Remember this device for 30 days</label>
<p>Use your capture credential, never a Cloudflare account token. Only remember private devices.</p>
<button id="login-submit">Log in</button></form><button id="forget" class="secondary" type="button" hidden>Forget this device / Log out</button>
<noscript><p>JavaScript is required for secure login and recoverable capture.</p></noscript>
<form id="capture" action="/api/v1/items" method="post"><label for="kind">Type</label><select id="kind" name="kind"><option value="note">Note</option><option value="todo">Todo</option></select>
<label for="title">Title (optional)</label><input id="title" name="title" maxlength="100">
<label for="text">Text</label><textarea id="text" name="text" required maxlength="16000"></textarea>
<button id="submit">Send</button><button id="discard" class="secondary danger" type="button" hidden>Discard saved retry</button></form>
<p id="status" role="status" aria-live="polite"></p>
<p>Drafts and pending notes are stored on this device, unencrypted. Offline? Keep the app open or reopen it and tap Retry when connected. Nothing is delivered while the app is closed.</p>
<p>Install from your browser menu: Install app / Add to Home screen. Each browser or installation may need its own login.</p>
<button id="update" class="secondary" type="button" hidden>Update app (reload saved draft)</button><script src="/capture.js" defer></script></html>`;

export const script = `
const $ = selector => document.querySelector(selector);
const form = $('#capture'), login = $('#login'), status = $('#status'), authState = $('#auth-state');
const button = $('#submit'), discard = $('#discard'), forget = $('#forget'), update = $('#update');
const field = name => form.elements.namedItem(name);
let pending, draft, token = '', authenticated = false, busy = false, waiting, authVersion = 0;
try {
  pending = JSON.parse(localStorage.getItem('pinny-pending') || 'null');
  draft = JSON.parse(localStorage.getItem('pinny-draft') || 'null');
} catch { status.textContent = 'Could not read saved content. Do not clear browser data if recovering a note.'; }
function auth(value, message) {
  authenticated = value; login.hidden = value; forget.hidden = !value;
  authState.textContent = message || (value ? 'Logged in. Ready to capture.' : 'Log in to send. You can still write a draft offline.');
}
function restore() {
  const saved = pending || draft;
  if (saved) for (const name of ['kind','title','text']) field(name).value = saved[name];
  discard.hidden = !pending;
  button.textContent = pending ? 'Retry pending note' : 'Send';
  for (const name of ['kind','title','text']) field(name).disabled = !!pending;
}
function saveDraft() {
  if (pending) return;
  draft = {kind:field('kind').value,title:field('title').value,text:field('text').value};
  localStorage.setItem('pinny-draft', JSON.stringify(draft));
}
form.oninput = () => {
  try { saveDraft(); status.textContent = 'Draft saved on this device. Not sent.'; }
  catch { status.textContent = 'Draft could not be saved. Keep this page open and copy your text somewhere safe.'; }
};
restore();
if (pending) status.textContent = 'Pending: not confirmed. Retry keeps the original ID and content.';
else if (draft) status.textContent = 'Draft restored. Not sent.';
auth(false);
fetch('/api/v1/session', {cache:'no-store', credentials:'same-origin'}).then(r => r.ok ? r.json() : null)
  .then(data => { if (data && data.authenticated && authVersion === 0) auth(true, 'This device is remembered. Ready to capture.'); }).catch(() => {});
login.onsubmit = async event => {
  event.preventDefault(); authVersion++; $('#login-submit').disabled = true;
  try {
    const credential = $('#password').value;
    const remembered = $('#remember').checked;
    const response = await fetch('/api/v1/session', {method:'POST', credentials:'same-origin', cache:'no-store',
      headers:{'Content-Type':'application/json', Authorization:'Bearer ' + credential}, body:JSON.stringify({remember:remembered}), signal:AbortSignal.timeout(20000)});
    if (!response.ok) throw new Error('login');
    token = remembered ? '' : credential; $('#password').value = '';
    auth(true, remembered ? 'Device remembered for 30 days.' : 'Logged in for this page only.');
  } catch { auth(false, 'Login not confirmed. Check your capture credential and connection. Your content is preserved.'); }
  finally { $('#login-submit').disabled = false; }
};
forget.onclick = async () => {
  authVersion++; forget.disabled = true;
  try {
    const response = await fetch('/api/v1/session', {method:'DELETE', credentials:'same-origin', cache:'no-store', signal:AbortSignal.timeout(20000)});
    if (!response.ok) throw new Error('logout');
    token = ''; $('#password').value = ''; auth(false, 'Device forgotten. Saved content remains on this device.');
  } catch { authState.textContent = 'Could not revoke the device. Reconnect and try Forget again.'; }
  finally { forget.disabled = false; }
};
discard.onclick = () => {
  if (busy || !confirm('Discard this retry? If a previous request succeeded, the PC may still receive it.')) return;
  try { localStorage.removeItem('pinny-pending'); localStorage.removeItem('pinny-draft'); }
  catch { status.textContent = 'Could not discard saved content.'; return; }
  pending = draft = null; restore(); status.textContent = 'Retry discarded. Visible text is now an unsaved draft.';
};
form.onsubmit = async event => {
  event.preventDefault(); if (busy) return;
  busy = true; button.disabled = discard.disabled = update.disabled = true;
  try {
    if (!pending) pending = {id:crypto.randomUUID(),kind:field('kind').value,title:field('title').value,text:field('text').value,completed:false};
    localStorage.setItem('pinny-pending', JSON.stringify(pending));
    restore();
    if (!authenticated) { status.textContent = 'Pending on this device. Log in, then tap Retry.'; return; }
    status.textContent = 'Sending… Waiting for server acceptance.';
    const response = await fetch('/api/v1/items', {method:'POST', credentials:'same-origin', cache:'no-store',
      headers:{'Content-Type':'application/json', ...(token ? {Authorization:'Bearer ' + token} : {})},
      body:JSON.stringify(pending),signal:AbortSignal.timeout(20000)});
    if (response.status === 401) { authVersion++; token = ''; auth(false, 'Login expired. Log in again; your pending note is preserved.'); }
    if (!response.ok) throw new Error('request');
    const accepted = await response.json();
    if (accepted.id !== pending.id) throw new Error('unconfirmed');
    localStorage.removeItem('pinny-draft'); localStorage.removeItem('pinny-pending');
    pending = draft = null; restore(); field('text').value = ''; field('title').value = '';
    status.textContent = 'Server accepted. Saved to the cloud inbox; the Windows desktop has not necessarily imported it yet.';
  } catch { status.textContent = 'Pending: not confirmed. Check login and connection, then tap Retry. Do not clear browser data.'; }
  finally { busy = false; button.disabled = discard.disabled = update.disabled = false; }
};
window.addEventListener('online', () => { if (pending && !busy) status.textContent = 'Connection available. Tap Retry for your pending note.'; });
if ('serviceWorker' in navigator) {
  navigator.serviceWorker.register('/sw.js').then(reg => {
    const offer = () => { waiting = reg.waiting; update.hidden = !waiting; };
    offer(); reg.addEventListener('updatefound', () => {
      const installing = reg.installing;
      installing.addEventListener('statechange', () => { if (installing.state === 'installed') offer(); });
    });
  }).catch(() => {});
  update.onclick = () => {
    if (busy || !waiting) return;
    try { saveDraft(); } catch { status.textContent = 'Save failed. Copy your draft before updating.'; return; }
    navigator.serviceWorker.addEventListener('controllerchange', () => location.reload(), {once:true});
    waiting.postMessage('ACTIVATE');
  };
}
`;
