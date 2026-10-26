// Local-only browser checks. Credentials/state are read only from the isolated local environment.
const fs = require('node:fs');
const assert = require('node:assert/strict');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE);
const BASE = 'http://127.0.0.1:28080';
const OUT = '/tmp/s2a-workbench-e2e/ui'; fs.mkdirSync(OUT, { recursive: true });
const env = Object.fromEntries(fs.readFileSync('/home/qlqq/workspace/sub2api-local/.env', 'utf8').split('\n').filter(l => l.includes('=') && !l.startsWith('#')).map(l => [l.slice(0,l.indexOf('=')), l.slice(l.indexOf('=')+1)]));
const state = JSON.parse(fs.readFileSync('/tmp/s2a-workbench-e2e/state.json','utf8'));
const report = { checks: [], errors: [], requests: [] };
const sleep = ms => new Promise(r => setTimeout(r, ms));
let browser;
async function login(tag) {
 const context = await browser.newContext({ viewport: { width:1440, height:1000 }, locale:'zh-CN', timezoneId:'Asia/Shanghai' });
 const page = await context.newPage();
 page.on('pageerror', e => report.errors.push(String(e).slice(0,300)));
 page.on('request', req => {
  if (req.method()==='POST' && /\/api\/chat\/completions|\/api\/nova\/tasks/.test(req.url())) {
   try { const b=JSON.parse(req.postData()); report.requests.push({path:new URL(req.url()).pathname,model:b.model,params:b.params}); } catch {}
  }
 });
 await page.goto(BASE+'/login');
 await page.locator('input[type="email"]').fill(state.users[tag].email);
 await page.locator('input[type="password"]').fill(env.E2E_USER_PASSWORD || env.ADMIN_PASSWORD);
 await page.locator('button[type="submit"]').click();
 await page.waitForURL(u=>!u.pathname.startsWith('/login'),{timeout:30000});
 await page.keyboard.press('Escape');
 return { context,page };
}
async function studio(page) {
 await page.goto(BASE+'/studio',{waitUntil:'load'});
 const el=await page.waitForSelector('iframe.qx-studio-frame',{timeout:30000});
 const frame=await el.contentFrame(); await sleep(5000); return frame;
}
async function chat(page) {
 await page.goto(BASE+'/chat',{waitUntil:'load'});
 let frame;
 for(let i=0;i<100;i++){frame=page.frames().find(f=>f.url().includes('/_chat/')&&!f.url().includes('/embed/'));if(frame&&await frame.locator('#chat-input').count())break; await sleep(500);}
 assert.ok(frame,'chat iframe');await sleep(4000);return frame;
}
async function defaults(page) {return page.evaluate(async()=> (await (await fetch('/api/v1/user/workbench/studio',{headers:{Authorization:'Bearer '+localStorage.auth_token}})).json()).data);}
async function chatSettings(frame) {return frame.evaluate(async()=> await (await fetch('/_chat/api/v1/users/user/settings',{headers:{Authorization:'Bearer '+localStorage.getItem('owui:token')}})).json());}
(async()=>{
 browser=await chromium.launch();
 for(const tag of ['WB','WZERO','WNONE']) {
  const {context,page}=await login(tag);
  const f=await studio(page);
  const d=await defaults(page);
  fs.writeFileSync(`${OUT}/${tag}-studio.txt`, await f.locator('body').innerText());
  await page.screenshot({path:`${OUT}/${tag}-studio.png`});
  assert.equal(d.initialized,true,'studio initialized');
  if(tag!=='WNONE') {
   assert.deepEqual(d.preferences,{image:{keyId:state.users[tag].key_id,model:'gpt-image-2.5-sunburst'},text:{keyId:state.users[tag].key_id,model:'gpt-6.1-sol'}});
  } else { assert.deepEqual(d.preferences,{}); }
  report.checks.push(tag+' studio defaults');
  const c=await chat(page);const settings=await chatSettings(c);
  fs.writeFileSync(`${OUT}/${tag}-chat.txt`,await c.locator('body').innerText());
  await page.screenshot({path:`${OUT}/${tag}-chat.png`});
  assert.equal(settings.ui?.sub2api_initialized,true,'chat initialized');
  assert.equal(settings.ui.sub2api.key_id,tag==='WNONE'?null:state.users[tag].key_id);
  assert.deepEqual(settings.ui.models,['gpt-6.1-sol']);
  assert.equal(settings.ui.sub2api_reasoning['gpt-6.1-sol'],'medium');
  report.checks.push(tag+' chat defaults');
  await context.close();
 }
})().catch(e=>{ report.failure=String(e.stack||e); console.error(e); process.exitCode=1; }).finally(async()=>{
 fs.writeFileSync(`${OUT}/report.json`,JSON.stringify(report,null,2)); if(browser)await browser.close();
 console.log(JSON.stringify({checks:report.checks,errors:report.errors,failure:report.failure},null,2));
});
