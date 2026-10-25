// Local-only real-browser regression. Uses existing p9 CA fixtures; never reads production credentials.
// source ~/.local/share/playwright-deps/env.sh; PLAYWRIGHT_MODULE=<path> node upload-ui.cjs <output-dir>
const fs = require('fs');
const path = require('path');
const assert = require('node:assert/strict');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE);
const ROOT = '/home/qlqq/workspace/sub2api-local';
const ORIGIN = 'http://127.0.0.1:28080';
assert.equal(new URL(ORIGIN).hostname, '127.0.0.1');
const env = Object.fromEntries(fs.readFileSync(ROOT + '/.env', 'utf8').split('\n').filter(l => l.includes('=') && !l.startsWith('#')).map(l => [l.slice(0, l.indexOf('=')), l.slice(l.indexOf('=') + 1)]));
const state = JSON.parse(fs.readFileSync(ROOT + '/e2e/runs/latest/state.json', 'utf8'));
const outDir = process.argv[2];
assert(outDir); fs.mkdirSync(outDir, { recursive: true });
const result = { steps: {}, csp: [], errors: [] };
const pause = ms => new Promise(r => setTimeout(r, ms));
(async () => {
  const browser = await chromium.launch();
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 1000 }, locale: 'zh-CN' });
  await ctx.addInitScript(() => document.addEventListener('securitypolicyviolation', e => console.warn('CSPV ' + JSON.stringify({ directive: e.violatedDirective, blocked: e.blockedURI }))));
  const page = await ctx.newPage();
  const cdp = await ctx.newCDPSession(page);
  await cdp.send('Network.enable', { maxTotalBufferSize: 128 * 1024 * 1024, maxResourceBufferSize: 64 * 1024 * 1024, maxPostDataSize: 0 });
  page.on('console', m => { if (m.text().startsWith('CSPV ')) result.csp.push(JSON.parse(m.text().slice(5))); });
  page.on('pageerror', e => result.errors.push(String(e).slice(0, 300)));
  let f, originalSettings;
  const shot = name => page.screenshot({ path: path.join(outDir, name + '.png') });
  const step = async (name, action) => {
    if (process.env.UPLOAD_CASES && !process.env.UPLOAD_CASES.split(',').includes(name)) return;
    try { result.steps[name] = { ok: true, ...await action() }; console.log('PASS', name); }
    catch (e) { result.steps[name] = { ok: false, error: String(e) }; console.log('FAIL', name, String(e)); await shot('FAIL-' + name).catch(() => {}); }
    fs.writeFileSync(path.join(outDir, 'result.json'), JSON.stringify(result, null, 2));
  };
  const api = async (endpoint, body) => f.evaluate(async ({ endpoint, body }) => {
    const response = await fetch('/_chat/api/v1/' + endpoint, { method: body === undefined ? 'GET' : 'POST', headers: { Authorization: 'Bearer ' + localStorage.getItem('owui:token'), 'Content-Type': 'application/json' }, ...(body === undefined ? {} : { body: JSON.stringify(body) }) });
    if (!response.ok) throw new Error('API ' + response.status);
    return response.json();
  }, { endpoint, body });
  const fresh = async (compression = false) => {
    await api('users/user/settings/update', { ...originalSettings, ui: { ...(originalSettings?.ui || {}), imageCompression: compression, imageCompressionSize: { width: 200, height: 150 }, models: ['claude-sonnet-4-5'], sub2api: { key_id: state.chat.keys.CA.chat, image_key_id: state.chat.keys.CA.img } } });
    await f.goto(ORIGIN + '/_chat/');
    await f.waitForSelector('#chat-input', { timeout: 45000 });
    await pause(1200);
    const selector = f.locator('[id^="model-selector-"][id$="-button"]').first();
    if (!(await selector.innerText()).includes('claude-sonnet-4-5')) {
      await selector.click();
      await f.locator('[role="option"]:has-text("claude-sonnet-4-5")').first().click();
    }
  };
  const fileInput = () => f.locator('input[type="file"][multiple]').first();
  const upload = async (fixture, method = 'choose') => {
    // Chromium may discard the inspector response for a 25 MiB POST. Observe a clone
    // inside the browser; leave the real upload request and application response untouched.
    await f.evaluate(() => {
      window.__uploadObserved = null;
      const originalFetch = window.fetch;
      window.fetch = async function (...args) {
        const response = await originalFetch.apply(this, args);
        if (String(args[0]).includes('/api/v1/files/?') && args[1]?.method === 'POST') {
          window.fetch = originalFetch;
          window.__uploadObserved = await response.clone().json();
        }
        return response;
      };
    });
    const responseP = page.waitForResponse(r => r.request().method() === 'POST' && r.url().includes('/_chat/api/v1/files/?'), { timeout: 25000 });
    if (method === 'choose') await fileInput().setInputFiles(fixture);
    else await f.evaluate(({ bytes, name, type, method }) => {
      const dt = new DataTransfer(); dt.items.add(new File([Uint8Array.from(bytes)], name, { type }));
      const target = document.querySelector('#chat-input');
      if (method === 'paste') target.dispatchEvent(new ClipboardEvent('paste', { clipboardData: dt, bubbles: true, cancelable: true }));
      else target.dispatchEvent(new DragEvent('drop', { dataTransfer: dt, bubbles: true, cancelable: true }));
    }, { bytes: [...fixture.buffer], name: fixture.name, type: fixture.mimeType, method });
    const response = await responseP;
    assert.equal(response.status(), 200);
    await f.waitForFunction(() => window.__uploadObserved !== null, null, { timeout: 25000 });
    const data = await f.evaluate(() => window.__uploadObserved);
    assert(data.id);
    await pause(1500);
    const stored = await ctx.request.get(ORIGIN + '/_chat/api/v1/files/' + data.id + '/content');
    assert.equal(stored.status(), 200);
    return { id: data.id, bytes: await stored.body(), data };
  };
  try {
    await page.goto(ORIGIN + '/login');
    await page.fill('input[type="email"], #email', state.users.CA.email);
    await page.fill('input[type="password"]', env.E2E_USER_PASSWORD || env.ADMIN_PASSWORD);
    await page.click('button[type="submit"]');
    await page.waitForURL(u => !u.pathname.startsWith('/login'));
    await page.keyboard.press('Escape');
    await page.goto(ORIGIN + '/chat');
    await page.waitForSelector('iframe[src^="/_chat/"]');
    for (let i = 0; i < 100; i++) { f = page.frames().find(x => x.url().includes('/_chat/')); if (f && await f.locator('#chat-input').count()) break; await pause(400); }
    await f.waitForSelector('#chat-input', { timeout: 45000 });
    originalSettings = await api('users/user/settings');
    const fixtures = {};
    for (const type of ['png', 'jpeg', 'webp']) {
      const url = await f.evaluate(type => { const c = document.createElement('canvas'); c.width = 800; c.height = 600; const g = c.getContext('2d'); g.fillStyle = '#2456ab'; g.fillRect(0, 0, 800, 600); g.fillStyle = '#ffffff'; g.font = '32px sans-serif'; g.fillText('UPLOAD REGRESSION', 60, 100); return c.toDataURL('image/' + type); }, type);
      let buffer = Buffer.from(url.split(',')[1], 'base64');
      if (type === 'png') buffer = Buffer.concat([buffer, Buffer.alloc(Math.max(0, 100 * 1024 - buffer.length))]);
      fixtures[type] = { name: 'upload-' + type + '.' + type, mimeType: 'image/' + type, buffer };
    }
    for (const type of ['png', 'jpeg', 'webp']) await step('choose-' + type, async () => {
      await fresh(); const uploaded = await upload(fixtures[type]);
      assert.deepEqual(uploaded.bytes, fixtures[type].buffer);
      await shot('choose-' + type);
      return { bytes: uploaded.bytes.length, exactOriginalBytes: true };
    });
    for (const method of ['drop', 'paste']) await step(method + '-png', async () => {
      await fresh(); const uploaded = await upload(fixtures.png, method);
      assert.deepEqual(uploaded.bytes, fixtures.png.buffer);
      return { bytes: uploaded.bytes.length };
    });
    await step('compression', async () => {
      await fresh(true); const uploaded = await upload(fixtures.png);
      const dimensions = await f.evaluate(async id => { const img = new Image(); img.src = '/_chat/api/v1/files/' + id + '/content'; await img.decode(); return [img.naturalWidth, img.naturalHeight]; }, uploaded.id);
      assert.deepEqual(dimensions, [200, 150]);
      await shot('compressed'); return { dimensions, bytes: uploaded.bytes.length };
    });
    await step('read-error-retry', async () => {
      await fresh();
      await f.evaluate(() => { const original = FileReader.prototype.readAsDataURL; FileReader.prototype.readAsDataURL = function () { FileReader.prototype.readAsDataURL = original; setTimeout(() => this.dispatchEvent(new Event('error')), 0); }; });
      await fileInput().setInputFiles(fixtures.png);
      await f.getByText('上传文件失败', { exact: false }).first().waitFor({ timeout: 6000 });
      const uploaded = await upload(fixtures.png); assert.deepEqual(uploaded.bytes, fixtures.png.buffer);
      return { recovered: true };
    });
    await step('screenshot', async () => {
      await fresh();
      await f.evaluate(() => { navigator.mediaDevices.getDisplayMedia = async () => { const c = document.createElement('canvas'); c.width = 320; c.height = 240; const g = c.getContext('2d'); g.fillStyle = '#2d7'; g.fillRect(0, 0, 320, 240); return c.captureStream(5); }; });
      const responseP = page.waitForResponse(r => r.request().method() === 'POST' && r.url().includes('/_chat/api/v1/files/?'), { timeout: 20000 });
      await f.locator('#input-menu-button').click();
      await f.getByText('截图', { exact: true }).click();
      const response = await responseP; assert.equal(response.status(), 200);
      const file = await response.json(); assert(file.filename.startsWith('screen-capture-'));
      await pause(1000); await shot('screenshot-upload');
      return { uploaded: true, captureSource: 'browser canvas MediaStream (OS picker simulated)' };
    });
    await step('pdf', async () => {
      await fresh();
      const content = 'BT /F1 14 Tf 30 100 Td (UPLOAD PDF PROBE) Tj ET';
      const objects = [
        '<< /Type /Catalog /Pages 2 0 R >>',
        '<< /Type /Pages /Kids [3 0 R] /Count 1 >>',
        '<< /Type /Page /Parent 2 0 R /MediaBox [0 0 300 200] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>',
        '<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>',
        '<< /Length ' + content.length + ' >>\nstream\n' + content + '\nendstream'
      ];
      let pdf = '%PDF-1.4\n'; const offsets = [0];
      objects.forEach((obj, i) => { offsets.push(Buffer.byteLength(pdf)); pdf += (i + 1) + ' 0 obj\n' + obj + '\nendobj\n'; });
      const xref = Buffer.byteLength(pdf);
      pdf += 'xref\n0 6\n0000000000 65535 f \n' + offsets.slice(1).map(n => String(n).padStart(10, '0') + ' 00000 n \n').join('') + 'trailer\n<< /Size 6 /Root 1 0 R >>\nstartxref\n' + xref + '\n%%EOF';
      const fixture = { name: 'upload.pdf', mimeType: 'application/pdf', buffer: Buffer.from(pdf) };
      const uploaded = await upload(fixture); assert.deepEqual(uploaded.bytes, fixture.buffer);
      const file = await api('files/' + uploaded.id);
      assert(JSON.stringify(file).includes('UPLOAD PDF PROBE'));
      return { extractedText: true, bytes: uploaded.bytes.length };
    });
    await step('txt-and-send-reload', async () => {
      await fresh();
      const fixture = { name: 'upload-probe.txt', mimeType: 'text/plain', buffer: Buffer.from('UPLOAD_TEXT_PROBE_20261025\nHello attachment!') };
      await upload(fixture); const image = await upload(fixtures.png);
      const completionP = page.waitForRequest(r => r.url().includes('/_chat/api/chat/completions') && r.method() === 'POST', { timeout: 20000 });
      await f.locator('#chat-input').fill('E2E_REPLY=md 请查看附件。');
      await f.locator('#chat-input').press('Enter');
      const request = await completionP;
      const body = request.postDataJSON();
      assert(JSON.stringify(body).includes(image.id));
      await f.waitForFunction(() => location.pathname.includes('/c/'), null, { timeout: 30000 });
      await f.getByText('第一点', { exact: true }).first().waitFor({ timeout: 30000 });
      await shot('sent-attachments');
      await f.goto(f.url()); await f.waitForSelector('#chat-input', { timeout: 45000 });
      await f.locator(`img[src*="${image.id}"]`).first().waitFor({ timeout: 15000 });
      await shot('reloaded-attachments');
      return { sentWithImage: true, reloaded: true };
    });
    await step('temporary-chat', async () => {
      await fresh(); await f.locator('#temporary-chat-button').click();
      let uploads = 0;
      const watch = r => { if (r.request().method() === 'POST' && r.url().includes('/_chat/api/v1/files/?')) uploads++; };
      page.on('response', watch);
      try {
        await fileInput().setInputFiles(fixtures.png);
        await f.locator('img[src^="data:image/png"]').first().waitFor({ timeout: 15000 });
        const requestP = page.waitForRequest(r => r.url().includes('/_chat/api/chat/completions') && r.method() === 'POST');
        await f.locator('#chat-input').fill('E2E_REPLY=md 临时图片。'); await f.locator('#chat-input').press('Enter');
        const request = await requestP;
        assert(JSON.stringify(request.postDataJSON()).includes('data:image/png;base64,'));
        assert.equal(uploads, 0); await pause(4000); await shot('temporary-image');
        return { uploads, sentInline: true };
      } finally { page.off('response', watch); }
    });
    await step('25MiB-boundary', async () => {
      await fresh();
      const fixture = { ...fixtures.png, name: 'maximum.png', buffer: Buffer.concat([fixtures.png.buffer, Buffer.alloc(25 * 1024 * 1024 - fixtures.png.buffer.length)]) };
      const uploaded = await upload(fixture); assert.deepEqual(uploaded.bytes, fixture.buffer);
      return { bytes: uploaded.bytes.length, multipartAccepted: true };
    });
    await step('over-limit-backend', async () => {
      await fresh();
      const token = await f.evaluate(() => localStorage.getItem('owui:token'));
      const response = await ctx.request.post(ORIGIN + '/_chat/api/v1/files/?process=false', {
        headers: { Authorization: 'Bearer ' + token },
        multipart: { file: { name: 'too-large.png', mimeType: 'image/png', buffer: Buffer.concat([fixtures.png.buffer, Buffer.alloc(25 * 1024 * 1024 + 1 - fixtures.png.buffer.length)]) } }
      });
      assert.equal(response.status(), 413); const body = await response.json(); assert(body.detail);
      return { status: 413, rejectedByFileLimit: true };
    });
    await step('over-limit-ui', async () => {
      await fresh();
      const fixture = { ...fixtures.png, name: 'oversize.png', buffer: Buffer.alloc(25 * 1024 * 1024 + 1) };
      await fileInput().setInputFiles(fixture);
      await f.getByText(/25 MB/).first().waitFor({ timeout: 8000 });
      return { rejected: true };
    });
  } catch (e) { result.fatal = String(e); console.log('FATAL', e); await shot('fatal').catch(() => {}); }
  finally {
    if (f && originalSettings) await api('users/user/settings/update', originalSettings).catch(e => { result.restoreError = String(e); });
    result.ok = !result.fatal && !result.restoreError && Object.values(result.steps).every(s => s.ok) && !result.csp.length && !result.errors.length;
    fs.writeFileSync(path.join(outDir, 'result.json'), JSON.stringify(result, null, 2));
    await browser.close();
    process.exitCode = result.ok ? 0 : 1;
  }
})();
