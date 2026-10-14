export const page = `<!doctype html>
<html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Pinny capture</title><style>
body{font:17px system-ui;margin:2rem auto;padding:0 1rem;max-width:36rem;background:#f5f5f7;color:#202328}
label{display:block;margin:1rem 0 .3rem}input,select,textarea,button{font:inherit;box-sizing:border-box;width:100%;padding:.7rem;border:1px solid #999;border-radius:.4rem}
textarea{min-height:12rem}button{margin-top:1rem;background:#2459ad;color:white}#status{min-height:3rem}
</style><h1>Pinny capture</h1><p>Send a note to your PC. It stays in the inbox until Pinny saves it.</p>
<form id="capture"><label for="token">Capture credential</label><input id="token" type="password" required autocomplete="off">
<p>The credential stays in memory only. Use your password manager; never enter a Cloudflare account token.</p>
<label for="kind">Type</label><select id="kind"><option value="note">Note</option><option value="todo">Todo</option></select>
<label for="title">Title (optional)</label><input id="title" maxlength="100">
<label for="text">Text</label><textarea id="text" required maxlength="16000"></textarea>
<button id="submit">Send</button><button id="discard" type="button" hidden>Discard saved retry</button></form>
<p id="status" role="status" aria-live="polite"></p><script src="/capture.js" defer></script></html>`;

export const script = `
const form = document.querySelector('#capture'), status = document.querySelector('#status');
const button = document.querySelector('#submit'), discard = document.querySelector('#discard');
const field = name => form.elements.namedItem(name);
let pending;
try { pending = JSON.parse(localStorage.getItem('pinny-pending') || 'null'); } catch {}
function restore() {
  if (pending) { field('kind').value = pending.kind; field('title').value = pending.title; field('text').value = pending.text;
    status.textContent = 'An unsent submission is saved on this device. Send to retry the same ID.'; }
  discard.hidden = !pending;
  for (const name of ['kind','title','text']) field(name).disabled = !!pending;
}
restore();
discard.onclick = () => {
  if (!confirm('Discard this retry? If a previous request succeeded, the PC may still receive it.')) return;
  localStorage.removeItem('pinny-pending'); pending = null; restore(); status.textContent = 'Retry discarded.';
};
form.onsubmit = async event => {
  event.preventDefault(); button.disabled = true; discard.disabled = true;
  try {
    if (!pending) {
      pending = {id:crypto.randomUUID(),kind:field('kind').value,title:field('title').value,text:field('text').value,completed:false};
      // Persist before sending so a lost response or reload reuses the exact ID and payload.
    }
    localStorage.setItem('pinny-pending', JSON.stringify(pending));
    restore();
    const response = await fetch('/api/v1/items', {method:'POST',headers:{'Content-Type':'application/json',
      Authorization:'Bearer ' + field('token').value},body:JSON.stringify(pending),signal:AbortSignal.timeout(20000)});
    if (!response.ok) throw new Error('request');
    localStorage.removeItem('pinny-pending'); pending = null; restore(); field('text').value = ''; field('title').value = '';
    status.textContent = 'Saved to the inbox. Your PC will collect it when sync runs.';
  } catch { status.textContent = 'Not confirmed. Check your credential or connection, then Send to retry. Do not clear browser data until confirmed.'; }
  finally { button.disabled = false; discard.disabled = false; }
};
`;
