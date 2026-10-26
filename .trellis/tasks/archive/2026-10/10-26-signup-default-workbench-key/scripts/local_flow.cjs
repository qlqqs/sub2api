// Local-only browser checks. Credentials/state are read only from the isolated local environment.
const fs = require('node:fs');
const assert = require('node:assert/strict');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE);
const BASE = 'http://127.0.0.1:28080';
const OUT = '/tmp/s2a-workbench-e2e/flow'; fs.mkdirSync(OUT, { recursive: true });
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
 for(const tag of ['WB','WZERO']) {
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
  const taskResponse=page.waitForResponse(r=>new URL(r.url()).pathname==='/api/nova/tasks' && r.request().method()==='POST');
  await f.locator('textarea').first().fill('E2E workbench default image '+tag);
  await f.locator('button:has(svg.lucide-arrow-up):not([disabled])').click();
  const created=await (await taskResponse).json();
  assert.ok(created.taskId,'image task created');
  let task;
  for(let i=0;i<45;i++) {
   task=await page.evaluate(async id=>await(await fetch('/api/nova/tasks/'+id,{headers:{Authorization:'Bearer '+localStorage.auth_token}})).json(),created.taskId);
   if(['completed','failed'].includes(task.status))break;await sleep(1000);
  }
  assert.equal(task.status,tag==='WB'?'completed':'failed');
  if(tag==='WZERO') {
   await f.getByText('账户余额不足，请充值后重试',{exact:false}).first().waitFor({timeout:15000});
   assert.ok(await f.getByText('去充值',{exact:true}).count());
   const visible=await f.locator('body').innerText();
   assert.ok(!visible.includes('403'),'do not display raw 403 for balance errors');
  }
  await sleep(800);await page.screenshot({path:`${OUT}/${tag}-image-result.png`});
  report.checks.push(tag+' image result');
  const c=await chat(page);const settings=await chatSettings(c);
  fs.writeFileSync(`${OUT}/${tag}-chat.txt`,await c.locator('body').innerText());
  await page.screenshot({path:`${OUT}/${tag}-chat.png`});
  assert.equal(settings.ui?.sub2api_initialized,true,'chat initialized');
  assert.equal(settings.ui.sub2api.key_id,tag==='WNONE'?null:state.users[tag].key_id);
  assert.deepEqual(settings.ui.models,['gpt-6.1-sol']);
  assert.equal(settings.ui.sub2api_reasoning['gpt-6.1-sol'],'medium');
  report.checks.push(tag+' chat defaults');
  const start=Date.now()/1000;
  await c.locator('#chat-input').fill('E2E workbench default chat '+tag);
  await c.locator('#chat-input').press('Enter');
  await sleep(12000);
  await page.screenshot({path:`${OUT}/${tag}-chat-result.png`});
  const requests=report.requests.filter(r=>r.path.includes('chat/completions'));
  assert.ok(requests.length>0,'chat request sent');
  assert.equal(requests.at(-1).model,'gpt-6.1-sol');
  assert.equal(requests.at(-1).params.reasoning_effort,'medium');
  if(tag==='WB') {
   const bodies=await(await fetch('http://127.0.0.1:28998/_poc_bodies')).json();
   assert.ok(bodies.some(r=>r.at>=start && r.path.endsWith('/v1/responses') && r.body.model==='gpt-6.1-sol' && r.body.reasoning?.effort==='medium'),'upstream receives exact text model/effort');
   report.checks.push('upstream Responses model and medium');
  } else {
   const text=await c.locator('body').innerText();assert.ok(text.includes('充值'),'chat zero balance recharge hint');
  }
  report.checks.push(tag+' chat request');
  await context.close();
 }
})().catch(e=>{ report.failure=String(e.stack||e); console.error(e); process.exitCode=1; }).finally(async()=>{
 fs.writeFileSync(`${OUT}/report.json`,JSON.stringify(report,null,2)); if(browser)await browser.close();
 console.log(JSON.stringify({checks:report.checks,errors:report.errors,failure:report.failure},null,2));
});
