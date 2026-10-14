import { test } from 'node:test';
import assert from 'node:assert/strict';
import { runInNewContext } from 'node:vm';
import { randomUUID } from 'node:crypto';
import { script } from '../src/website.ts';

function browser(storage, fetch) {
  const fields = Object.fromEntries(['token', 'kind', 'title', 'text'].map(name => [name, { value: '', disabled: false }]));
  fields.kind.value = 'todo';
  // HTMLFormElement.title is a string, not the title input.
  const form = { title: '', elements: { namedItem: name => fields[name] } };
  const status = { textContent: '' }, button = {}, discard = {};
  runInNewContext(script, {
    document: { querySelector: selector => ({ '#capture': form, '#status': status, '#submit': button, '#discard': discard })[selector] },
    localStorage: storage, fetch, crypto: { randomUUID }, AbortSignal, confirm: () => true,
  });
  return { form, fields, status, button, discard, submit: () => form.onsubmit({ preventDefault() {} }) };
}
function localStorage() {
  const values = new Map();
  return { getItem: key => values.get(key) ?? null, setItem: (key, value) => values.set(key, value), removeItem: key => values.delete(key) };
}
test('successful submission clears draft but never persists the credential', async () => {
  const storage = localStorage(); let sent;
  const app = browser(storage, async (_url, request) => { sent = JSON.parse(request.body); return { ok: true }; });
  app.fields.token.value = 'synthetic-capture-credential'; app.fields.text.value = 'Synthetic mobile note'; app.fields.title.value = 'Todo';
  await app.submit();
  assert.equal(sent.kind, 'todo'); assert.equal(sent.title, 'Todo');
  assert.equal(storage.getItem('pinny-pending'), null);
  assert.equal(app.fields.text.value, ''); assert.match(app.status.textContent, /Saved/);
});
test('lost response and reload reuse the exact persisted ID and payload', async () => {
  const storage = localStorage(); let first;
  const app = browser(storage, async (_url, request) => { first = JSON.parse(request.body); throw new Error('lost response'); });
  app.fields.token.value = 'do-not-store-this-credential'; app.fields.text.value = 'Offline todo';
  await app.submit();
  assert.ok(first.id); assert.equal(app.button.disabled, false); assert.equal(app.fields.text.disabled, true);
  assert.ok(!storage.getItem('pinny-pending').includes('do-not-store-this-credential'));
  let second;
  const restarted = browser(storage, async (_url, request) => { second = JSON.parse(request.body); return { ok: true }; });
  assert.equal(restarted.fields.token.value, ''); assert.equal(restarted.fields.text.value, first.text);
  restarted.fields.token.value = 'do-not-store-this-credential';
  await restarted.submit();
  assert.deepEqual(first, second); assert.equal(storage.getItem('pinny-pending'), null);
});
test('storage failures do not send a non-durable retry', async () => {
  let requests = 0;
  const app = browser({ getItem: () => null, setItem: () => { throw new Error('storage unavailable'); } }, async () => { requests++; });
  app.fields.text.value = 'Test'; await app.submit();
  assert.equal(requests, 0); assert.match(app.status.textContent, /Not confirmed/);
});
