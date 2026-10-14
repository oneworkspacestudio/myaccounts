// UI-only fixture: never contacts the shared service or uses a real mailbox.
import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {readFile,mkdir} from 'node:fs/promises';
import {resolve,join} from 'node:path';
import {chromium} from '@playwright/test';

const root=resolve(import.meta.dirname,'..'),output=join(root,'dist','setup-review');await mkdir(output,{recursive:true});
const identity=await readFile(join(root,'internal/portable/setup.html'));
const operator=await readFile(join(root,'internal/mailservice/setup.html'));
let setupBody;
const server=createServer(async(req,res)=>{
 res.setHeader('Content-Type','application/json');
 if(req.method==='POST'){
  let text='';for await(const chunk of req)text+=chunk;
  const body=JSON.parse(text);assert.equal(req.headers['x-setup-token'],'fixture-token');
  if(req.url==='/setup')setupBody=body;
  if(req.url==='/owner/finish')assert.equal(body.code,'123456');
  res.end('{"ok":true}');return;
 }
 if(req.url==='/setup-info'){res.end(JSON.stringify({dataPath:'F:/MyAccounts/fixture',sharedEmail:true}));return}
 res.setHeader('Content-Type','text/html; charset=utf-8');res.end(req.url==='/operator'?operator:identity);
});
await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
const origin=`http://127.0.0.1:${server.address().port}`;
const browser=await chromium.launch({channel:'chrome',headless:true});
try{
 for(const [size,viewport] of [['desktop',{width:1360,height:1000}],['mobile',{width:390,height:844}]]){
  const page=await browser.newPage({viewport});
  const errors=[];page.on('pageerror',e=>errors.push(e.message));
  await page.goto(origin+'/#fixture-token');
  await page.locator('#name').fill('Fixture Administrator');await page.locator('#email').fill('fixture@example.com');
  await page.locator('#password').fill('a long fixture password');await page.locator('#confirm').fill('a long fixture password');
  await page.locator('#next').click();await page.locator('#next').click();
  assert.equal(await page.locator('#emailMode').inputValue(),'none');
  await page.locator('#emailMode').selectOption('shared');
  await page.locator('#next').click();assert.match(await page.locator('#error').innerText(),/Verify your email/);
  assert.equal(await page.locator('#smtpHost').isVisible(),false);
  await page.locator('#connectEmail').click();await page.locator('#emailCode').fill('123456');await page.locator('#verifyEmail').click();
  await page.locator('#emailStatus').filter({hasText:'Email connected'}).waitFor();
  assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false);
  await page.screenshot({path:join(output,`installer-email-${size}.png`),fullPage:true});
  await page.locator('#next').click();await page.locator('#finish').click();
  await page.locator('#error').filter({hasText:'Account created'}).waitFor();assert.equal(setupBody.emailMode,'shared');assert(!setupBody.smtpPassword);assert.equal(await page.locator('#error').innerText(),'Account created. Starting your dashboard…');
  await page.goto(origin+'/operator#fixture-token');
  await page.locator('#email').fill('fixture@example.com');await page.locator('#password').fill('fake-owner-app-password');
  await page.locator('#next').click();await page.locator('#mode').selectOption('manual');await page.locator('#next').click();
  await page.locator('#send').click();await page.locator('#code').fill('123456');assert.equal(await page.locator('#finish').isVisible(),true);
  assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false);
  await page.screenshot({path:join(output,`service-owner-${size}.png`),fullPage:true});
  await page.locator('#finish').click();await page.getByText('Email service configured.',{exact:false}).waitFor();
  await page.goto(origin+'/#fixture-token');
  await page.locator('#name').fill('Fixture Administrator');await page.locator('#email').fill('fixture@example.com');
  await page.locator('#password').fill('a long fixture password');await page.locator('#confirm').fill('a long fixture password');
  await page.locator('#next').click();await page.locator('#next').click();
  assert.equal(await page.locator('#emailMode').inputValue(),'none');
  await page.locator('#next').click();await page.locator('#finish').click();
  await page.locator('#error').filter({hasText:'Account created'}).waitFor();assert.equal(setupBody.emailMode,'none');assert(!setupBody.smtpPassword);assert(!setupBody.emailCode);
  await page.goto(origin+'/operator#fixture-token');
  await page.locator('#skip').click();await page.getByRole('heading',{name:'Email setup skipped'}).waitFor();
  assert.equal(await page.locator('#email').count(),0);
  assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false);
  assert.deepEqual(errors,[]);await page.close();
 }
 console.log('PASS: desktop/mobile installer email enrollment, required verification, hidden SMTP settings, owner wizard, optional no-email installation/owner skip and no horizontal overflow');
}finally{await browser.close();await new Promise(resolve=>server.close(resolve))}
