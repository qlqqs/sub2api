// Local-only browser checks. Credentials/state are read only from the isolated local environment.
const fs = require('node:fs');
const assert = require('node:assert/strict');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE);
const BASE = 'http://127.0.0.1:28080';
const OUT = '/tmp/s2a-workbench-e2e/persistence'; fs.mkdirSync(OUT, { recursive: true });
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
 let {context,page}=await login('WB');
 let f=await studio(page);
 const original=await defaults(page);
 // Explicit changes use exactly the same authenticated endpoints as the settings UI.
 const manual=await page.evaluate(async gid=>{
  const headers={Authorization:'Bearer '+localStorage.auth_token,'Content-Type':'application/json'};
  const k=(await(await fetch('/api/v1/keys',{method:'POST',headers,body:JSON.stringify({name:'手动工作台 Key',group_id:gid})})).json()).data;
  const prefs={image:{keyId:k.id,model:'gpt-image-2.5-sunburst'},text:{keyId:k.id,model:'gpt-6.1-sol'}};
  const r=await fetch('/api/v1/user/workbench/studio',{method:'PUT',headers,body:JSON.stringify({preferences:prefs})});
  if(!r.ok)throw new Error('save failed');return prefs;
 },state.workbench_group_id);
 await context.close();
 ({context,page}=await login('WB'));f=await studio(page);
 assert.deepEqual((await defaults(page)).preferences,manual);
 report.checks.push('Nova manual Key survives fresh browser/login');
 await page.evaluate(async()=>{
  const r=await fetch('/api/v1/user/workbench/studio',{method:'PUT',headers:{Authorization:'Bearer '+localStorage.auth_token,'Content-Type':'application/json'},body:JSON.stringify({preferences:{}})});
  if(!r.ok)throw new Error('clear failed');
 });
 await context.close();({context,page}=await login('WB'));f=await studio(page);
 assert.deepEqual((await defaults(page)).preferences,{});
 await page.screenshot({path:OUT+'/studio-cleared.png'});
 report.checks.push('Nova explicit clear survives fresh browser/login');
 let c=await chat(page);
 await c.locator('[data-testid="s2a-reasoning-trigger"]').click();
 await c.locator('[data-testid="s2a-reasoning-slider"]').press('ArrowRight');
 await sleep(1500);
 assert.equal((await chatSettings(c)).ui.sub2api_reasoning['gpt-6.1-sol'],'high');
 await context.close();({context,page}=await login('WB'));c=await chat(page);
 assert.equal((await chatSettings(c)).ui.sub2api_reasoning['gpt-6.1-sol'],'high');
 assert.ok((await c.locator('[data-testid="s2a-reasoning-trigger"]').innerText()).includes('高'));
 await page.screenshot({path:OUT+'/chat-high.png'});
 report.checks.push('chat high choice survives fresh browser/login and UI displays high');
 await c.evaluate(async()=>{
  const r=await fetch('/_chat/api/v1/users/user/settings/update',{method:'POST',headers:{Authorization:'Bearer '+localStorage.getItem('owui:token'),'Content-Type':'application/json'},body:JSON.stringify({ui:{sub2api:{key_id:null,image_key_id:null}}})});
  if(!r.ok)throw new Error('chat clear failed');
 });
 await context.close();({context,page}=await login('WB'));c=await chat(page);
 assert.deepEqual((await chatSettings(c)).ui.sub2api,{key_id:null,image_key_id:null});
 await page.screenshot({path:OUT+'/chat-cleared.png'});
 report.checks.push('chat explicit clear survives fresh browser/login');
 // Restore only this fixture's choices for repeatable local demonstrations.
 await page.evaluate(async prefs=>await fetch('/api/v1/user/workbench/studio',{method:'PUT',headers:{Authorization:'Bearer '+localStorage.auth_token,'Content-Type':'application/json'},body:JSON.stringify({preferences:prefs})}),original.preferences);
 await c.evaluate(async keyId=>await fetch('/_chat/api/v1/users/user/settings/update',{method:'POST',headers:{Authorization:'Bearer '+localStorage.getItem('owui:token'),'Content-Type':'application/json'},body:JSON.stringify({ui:{sub2api:{key_id:keyId,image_key_id:keyId},sub2api_reasoning:{'gpt-6.1-sol':'medium'}}})}),state.users.WB.key_id);
 await context.close();
})().catch(e=>{report.failure=String(e.stack||e);console.error(e);process.exitCode=1;}).finally(async()=>{
 fs.writeFileSync(OUT+'/report.json',JSON.stringify(report,null,2));if(browser)await browser.close();
 console.log(JSON.stringify(report,null,2));
});
