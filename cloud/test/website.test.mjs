import { test } from 'node:test';
import assert from 'node:assert/strict';
import { runInNewContext } from 'node:vm';
import { randomUUID } from 'node:crypto';
import { script, page } from '../src/website.ts';
import { serviceWorker, manifest } from '../src/pwa.ts';

function storage() {
  const values = new Map();
  return { getItem: key => values.get(key) ?? null, setItem: (key, value) => values.set(key, value), removeItem: key => values.delete(key) };
}
async function browser(localStorage, send = async () => { throw Error('offline'); }, remembered = true, environment = {}) {
  const fields = Object.fromEntries(['kind','title','text'].map(name => [name, {value:'',disabled:false}])); fields.kind.value = 'todo';
  const nodes = Object.fromEntries(['capture','login','status','auth-state','submit','discard','forget','update','password','remember','login-submit'].map(name => [name, {value:'',checked:false}]));
  nodes.capture.elements = {namedItem:name => fields[name]};
  const events = {};
  runInNewContext(script, {
    document:{querySelector:selector => nodes[selector.slice(1)]}, localStorage,
    fetch:async (url, request) => url === '/api/v1/session' && !request.method ? {ok:true,json:async () => ({authenticated:remembered})} : send(url,request),
    crypto:{randomUUID}, AbortSignal, confirm:() => true, navigator:environment.navigator || {}, location:environment.location,
    window:{addEventListener:(name,fn) => events[name] = fn},
  });
  await new Promise(resolve => setImmediate(resolve));
  return {fields,nodes,events,submit:() => nodes.capture.onsubmit({preventDefault(){}}),login:() => nodes.login.onsubmit({preventDefault(){}})};
}
const accepted = request => ({ok:true,json:async () => ({id:JSON.parse(request.body).id})});
test('accessible password-manager login form and standalone manifest', () => {
  assert.match(page, /name="username" autocomplete="username"/);
  assert.match(page, /name="password" type="password" required autocomplete="current-password"/);
  assert.equal(JSON.parse(manifest).display, 'standalone');
  assert.equal(JSON.parse(manifest).start_url, '/');
});
test('remembered startup opens capture and successful submission clears content only after acceptance', async () => {
  const saved = storage(); let sent;
  const app = await browser(saved, async (_url, request) => {sent = JSON.parse(request.body); return accepted(request);});
  assert.equal(app.nodes.login.hidden,true);
  app.fields.text.value = 'Synthetic mobile note'; app.fields.title.value = 'Todo';
  app.nodes.capture.oninput(); assert.ok(saved.getItem('pinny-draft'));
  await app.submit();
  assert.equal(sent.title,'Todo'); assert.equal(saved.getItem('pinny-pending'),null); assert.equal(saved.getItem('pinny-draft'),null);
  assert.equal(app.fields.text.value,''); assert.match(app.nodes.status.textContent,/Server accepted/);
});
test('offline drafts survive reload; lost response reuses unchanged pending ID and payload', async () => {
  const saved = storage(); let first;
  const app = await browser(saved, async (_url,request) => {first = JSON.parse(request.body); throw Error('lost');});
  app.fields.text.value = 'Offline todo'; app.nodes.capture.oninput();
  const draftReload = await browser(saved); assert.equal(draftReload.fields.text.value,'Offline todo');
  await app.submit(); assert.ok(first.id); assert.equal(app.fields.text.disabled,true);
  assert.match(app.nodes.status.textContent,/Pending/);
  let second;
  const restarted = await browser(saved, async (_url,request) => {second = JSON.parse(request.body); return accepted(request);});
  restarted.events.online(); assert.match(restarted.nodes.status.textContent,/Tap Retry/);
  await restarted.submit(); assert.deepEqual(first,second); assert.equal(saved.getItem('pinny-pending'),null);
});
test('expired authentication preserves pending content through reauthentication without storing credentials', async () => {
  const saved = storage(); let payload, loggedIn = false;
  const app = await browser(saved, async (url,request) => {
    if (url.endsWith('/session')) { loggedIn = true; return {ok:true}; }
    payload = JSON.parse(request.body);
    return loggedIn ? accepted(request) : {ok:false,status:401};
  });
  app.fields.text.value = 'Keep this'; await app.submit();
  assert.equal(app.nodes.login.hidden,false); const original = saved.getItem('pinny-pending');
  app.nodes.password.value = 'never-store-this'; app.nodes.remember.checked = true; await app.login();
  assert.equal(app.nodes.password.value,''); assert.equal(saved.getItem('pinny-pending'),original);
  assert.ok(!JSON.stringify([saved.getItem('pinny-draft'),original]).includes('never-store-this'));
  await app.submit(); assert.deepEqual(payload,JSON.parse(original));
});
test('unauthenticated offline capture is pending; storage failures do not send', async () => {
  let requests = 0;
  const app = await browser(storage(), async () => { requests++; }, false);
  app.fields.text.value = 'Offline'; await app.submit(); assert.equal(requests,0); assert.match(app.nodes.status.textContent,/Log in/);
  const broken = await browser({getItem:() => null,setItem:() => {throw Error('quota');}},async () => {requests++;});
  broken.fields.text.value = 'Test'; await broken.submit(); assert.equal(requests,0); assert.match(broken.nodes.status.textContent,/not confirmed/);
});
test('overlap is blocked and no success is shown before valid server acceptance', async () => {
  let resolve, requests = 0;
  const app = await browser(storage(), async () => {requests++; return new Promise(r => resolve = r);});
  app.fields.text.value = 'Test'; const sending = app.submit();
  assert.match(app.nodes.status.textContent,/Sending/); assert.equal(app.nodes.update.disabled,true);
  await app.submit(); assert.equal(requests,1);
  resolve({ok:true,json:async () => ({id:'wrong'})}); await sending;
  assert.match(app.nodes.status.textContent,/not confirmed/);
});
test('service worker caches only the explicit public shell, never APIs, POSTs, secrets or query URLs', async () => {
  const events = {}, stored = new Map(); let fetched = 0, skipped = 0, deleted = [];
  const cache = {addAll:async requests => {for(const r of requests) {assert.equal(r.credentials,'omit'); stored.set(new URL(r.url).pathname,{});}},match:async path => stored.get(path)};
  // Resolve service-worker-relative Request URLs in this isolated harness.
  class SWRequest extends Request {constructor(url,options) {super(typeof url === 'string' ? new URL(url,'https://pinny.test') : url,options);}}
  runInNewContext(serviceWorker, {URL,Request:SWRequest,
    self:{location:{origin:'https://pinny.test'},addEventListener:(name,fn) => events[name] = fn,clients:{claim:async () => {}},skipWaiting:() => skipped++},
    caches:{open:async () => cache,keys:async () => ['pinny-shell-old','unrelated','pinny-shell-v3'],delete:async key => deleted.push(key)},fetch:async () => {fetched++; return {};},
  });
  let work; events.install({waitUntil:p => work = p}); await work;
  assert.deepEqual([...stored.keys()],['/','/capture.js','/manifest.webmanifest','/icon.svg']);
  for (const [path,method] of [['/api/v1/session','GET'],['/api/v1/items','POST'],['/?secret=x','GET'],['/sw.js','GET'],['https://evil.test/','GET']]) {
    let handled = false; events.fetch({request:new Request(new URL(path,'https://pinny.test'),{method}),respondWith:() => handled = true}); assert.equal(handled,false);
  }
  let result; events.fetch({request:new Request('https://pinny.test/'),respondWith:p => result = p}); assert.ok(await result); assert.equal(fetched,0);
  assert.equal(skipped,0); events.message({data:'ACTIVATE'}); assert.equal(skipped,1);
  events.activate({waitUntil:p => work = p}); await work; assert.deepEqual(deleted,['pinny-shell-old']);
});
test('updates require consent, save drafts, preserve pending payloads, and cannot interrupt sending', async () => {
  const saved = storage(); let activate = 0, reload = 0, controllerChange, finish;
  const environment = {navigator:{serviceWorker:{
    register:async () => ({waiting:{postMessage:message => {assert.equal(message,'ACTIVATE'); activate++;}},addEventListener(){}}),
    addEventListener:(name,fn) => {assert.equal(name,'controllerchange'); controllerChange = fn;},
  }},location:{reload:() => reload++}};
  const app = await browser(saved,async () => new Promise(r => finish = r),true,environment);
  assert.equal(app.nodes.update.hidden,false); assert.equal(activate,0);
  app.fields.text.value = 'Keep draft through update';
  app.nodes.update.onclick(); assert.equal(activate,1);
  assert.equal(JSON.parse(saved.getItem('pinny-draft')).text,'Keep draft through update');
  controllerChange(); assert.equal(reload,1);
  const sending = app.submit(); const original = saved.getItem('pinny-pending');
  app.nodes.update.onclick(); assert.equal(activate,1);
  finish({ok:false,status:503}); await sending;
  app.nodes.update.onclick(); assert.equal(activate,2); assert.equal(saved.getItem('pinny-pending'),original);
});
