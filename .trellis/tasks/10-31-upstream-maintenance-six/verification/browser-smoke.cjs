// Local-only browser acceptance for the six-patch candidate. No external model calls.
const fs = require('fs');
const path = require('path');
const assert = require('node:assert/strict');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE);
const BASE = 'http://127.0.0.1:28080';
const env = Object.fromEntries(fs.readFileSync('/home/qlqq/workspace/sub2api-local/.env', 'utf8')
  .split('\n').filter(l => l.includes('=') && !l.startsWith('#')).map(l => [l.slice(0, l.indexOf('=')), l.slice(l.indexOf('=') + 1)]));
const out = '/tmp/sub2api-six-browser';
fs.mkdirSync(out, { recursive: true });
const checks = [], pageErrors = [], csp = [], consoleErrors = [];
function check(name, ok) { assert.ok(ok, name); checks.push(name); console.log('PASS ' + name); }
(async () => {
  const browser = await chromium.launch();
  const context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, locale: 'zh-CN' });
  await context.addInitScript(() => {
    if (!localStorage.getItem('sub2api_locale')) localStorage.setItem('sub2api_locale', 'zh');
    window.__cspViolations = [];
    document.addEventListener('securitypolicyviolation', e => window.__cspViolations.push({ directive: e.effectiveDirective, blocked: e.blockedURI, document: e.documentURI, line: e.lineNumber }));
  });
  const page = await context.newPage();
  if (process.env.DISABLE_CACHE === '1') {
    const session = await context.newCDPSession(page);
    await session.send('Network.enable');
    await session.send('Network.setCacheDisabled', { cacheDisabled: true });
  }
  page.on('pageerror', e => pageErrors.push(e.message));
  page.on('console', m => { if (m.type() === 'error') consoleErrors.push(m.text()); });
  let token, oldSearch, channelId, planId, planGroupId; const groups = [];
  const api = async (method, route, data) => {
    const r = await context.request.fetch(BASE + '/api/v1' + route, { method, data, headers: token ? { Authorization: 'Bearer ' + token } : {} });
    assert.ok(r.ok(), `${method} ${route}: ${r.status()} ${await r.text()}`);
    return (await r.json()).data;
  };
  const dismissTour = async () => {
    await page.waitForTimeout(1500);
    for (let i = 0; i < 3; i++) { await page.keyboard.press('Escape'); await page.waitForTimeout(200); }
  };
  const capture = async name => {
    csp.push(...await page.evaluate(() => window.__cspViolations || []));
    const text = await page.locator('body').innerText();
    check(name + ' renders content', text.length > 100);
    check(name + ' no raw Qx locale keys', !/qxTheme\.(hero|nav|dashboard)\.[a-zA-Z]/.test(text));
    fs.writeFileSync(path.join(out, name + '.txt'), text);
    await page.screenshot({ path: path.join(out, name + '.png'), fullPage: true });
  };
  const visit = async (route, name) => {
    const r = await page.goto(BASE + route, { waitUntil: 'networkidle' });
    const policy = r.headers()['content-security-policy'] || '';
    check(name + ' CSP present without unsafe-eval', policy.length > 0 && !policy.includes("'unsafe-eval'"));
    await dismissTour();
    await capture(name);
  };
  try {
    await visit('/', 'home-zh');
    await visit('/login', 'login-zh');
    await page.locator('input[type="email"]').fill(env.ADMIN_EMAIL);
    await page.locator('input[type="password"]').fill(env.ADMIN_PASSWORD);
    await page.locator('button[type="submit"]').click();
    await page.waitForURL(u => !u.pathname.startsWith('/login'), { timeout: 20000 });
    const auth = await api('POST', '/auth/login', { email: env.ADMIN_EMAIL, password: env.ADMIN_PASSWORD });
    token = auth.access_token;
    const planGroup = await api('POST', '/admin/groups', { name: `six-ui-plan-${Date.now()}`, platform: 'openai', subscription_type: 'subscription', is_exclusive: false, rate_multiplier: 1 });
    planGroupId = planGroup.id;
    const plan = await api('POST', '/admin/payment/plans', { group_id: planGroupId, name: 'Six i18n monthly plan', price: 1, currency: 'USD', validity_days: 1, validity_unit: 'months', features: '[]', for_sale: true });
    planId = plan.id;
    await visit('/dashboard', 'dashboard-zh');
    // Exercise the actual language switcher and its reload, not only storage injection.
    await page.locator('button[title="中文"]').click();
    await page.getByRole('button', { name: /English/ }).click();
    await page.waitForFunction(() => document.documentElement.lang === 'en');
    await page.waitForLoadState('networkidle');
    await dismissTour();
    check('English dashboard title', (await page.title()).includes('Dashboard'));
    await capture('dashboard-en');
    await visit('/purchase?tab=subscription', 'purchase-en');
    const planCard = page.locator('div.group').filter({ has: page.getByRole('heading', { name: 'Six i18n monthly plan', exact: true }) });
    check('English monthly plan translated', (await planCard.innerText()).includes('/ month') && (await planCard.innerText()).includes('Subscribe Now'));
    await page.locator('button[title="English"]').click();
    await page.getByRole('button', { name: /中文/ }).click();
    await page.waitForFunction(() => document.documentElement.lang === 'zh');
    await page.waitForLoadState('networkidle');
    await dismissTour();
    await capture('purchase-zh');
    check('Chinese monthly plan translated', (await planCard.innerText()).includes('/ 月') && (await planCard.innerText()).includes('立即开通'));
    await visit('/admin/users', 'admin-users-zh');
    oldSearch = await api('GET', '/admin/settings/web-search-emulation');
    await api('PUT', '/admin/settings/web-search-emulation', { ...oldSearch, enabled: true, providers: oldSearch.providers?.length ? oldSearch.providers : [{ type: 'tavily', api_key: 'local-ui-unused-no-search-request' }] });
    const run = Date.now();
    for (const platform of ['anthropic', 'openai']) {
      const g = await api('POST', '/admin/groups', { name: `six-ui-${platform}-${run}`, platform, subscription_type: 'standard', is_exclusive: false, rate_multiplier: 1 });
      groups.push(g.id);
    }
    const name = `six-ui-channel-${run}`;
    const channel = await api('POST', '/admin/channels', { name, group_ids: groups, features_config: { web_search_emulation: { anthropic: true, openai: true, command_code: true } } });
    channelId = channel.id;
    // The API must retain dimensions omitted by a partial/older client.
    const partial = await api('PUT', `/admin/channels/${channelId}`, { features_config: { web_search_emulation: { anthropic: false } } });
    check('partial API save keeps other platforms', partial.features_config.web_search_emulation.openai === true && partial.features_config.web_search_emulation.command_code === true && partial.features_config.web_search_emulation.anthropic === false);
    await visit('/admin/channels', 'channels-zh');
    const openEdit = async () => {
      await page.locator('tr').filter({ hasText: name }).getByRole('button', { name: '编辑', exact: true }).click();
      await page.locator('#channel-form').waitFor();
      await page.locator('.channel-tab').filter({ hasText: 'OpenAI' }).click();
    };
    await openEdit();
    // Hidden platform tabs remain in the DOM, so address the visible switch only.
    const toggle = page.locator('#channel-form label:visible').filter({ hasText: /^Web Search 模拟$/ }).locator('..').locator('..').getByRole('switch');
    check('OpenAI search toggle initially on', await toggle.getAttribute('aria-checked') === 'true');
    await toggle.click();
    const saved = page.waitForResponse(r => r.url().endsWith(`/admin/channels/${channelId}`) && r.request().method() === 'PUT');
    await page.locator('button[form="channel-form"]').click();
    check('channel save succeeds', (await saved).status() === 200);
    await page.reload({ waitUntil: 'networkidle' });
    await openEdit();
    check('OpenAI search off persists after reload', await toggle.getAttribute('aria-checked') === 'false');
    await capture('channel-openai-off');
    await page.keyboard.press('Escape');
    await page.setViewportSize({ width: 390, height: 844 });
    await visit('/dashboard', 'dashboard-mobile');
    check('no uncaught browser errors', pageErrors.length === 0);
    check('no CSP violations', csp.length === 0);
  } finally {
    if (planId) await api('DELETE', `/admin/payment/plans/${planId}`);
    if (planGroupId) await api('DELETE', `/admin/groups/${planGroupId}`);
    if (channelId) await api('DELETE', `/admin/channels/${channelId}`);
    for (const id of groups) await api('DELETE', `/admin/groups/${id}`);
    if (oldSearch) await api('PUT', '/admin/settings/web-search-emulation', oldSearch);
    fs.writeFileSync(path.join(__dirname, 'browser-results.json'), JSON.stringify({ checks, pageErrors, csp, consoleErrors, cacheDisabled: process.env.DISABLE_CACHE === '1', screenshots: out }, null, 2));
    await browser.close();
  }
})().catch(e => { console.error(e); process.exitCode = 1; });
