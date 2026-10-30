import assert from 'node:assert/strict';
import {pathToFileURL} from 'node:url';
const issuer=process.env.MYACCOUNTS_TEST_ISSUER,password=process.env.MYACCOUNTS_TEST_PASSWORD;
const host=process.env.AUTHENTICATOR_TEST_ORIGIN||'http://localhost:18798';
assert(issuer?.startsWith('http://localhost:')&&host.startsWith('http://localhost:')&&password&&process.env.MYACCOUNTS_FIXTURE);
const {chromium}=await import(pathToFileURL(process.env.MYACCOUNTS_PLAYWRIGHT).href);
const browser=await chromium.launch({headless:true,executablePath:'C:/Program Files/Google/Chrome/Application/chrome.exe'});
const desktop=await browser.newContext(),phone=await browser.newContext({viewport:{width:390,height:844}});
const errors=[];
try{
 const result=await desktop.request.post(issuer+'/api/login',{headers:{Origin:issuer},data:{organization:'built-in',username:'admin@example.com',password,application:'app-built-in',type:'login',signinMethod:'Password'}});assert.equal((await result.json()).status,'ok');
 const page=await desktop.newPage();page.on('pageerror',e=>errors.push(e.message));await page.goto(issuer+'/mfa/setup?mfaType=app');
 await page.getByLabel('Current OneAccount password',{exact:true}).fill(password);await page.getByLabel('Device label',{exact:true}).fill('Synthetic offline phone');await page.getByRole('button',{name:'Enable two-factor authentication',exact:true}).click();
 await page.getByRole('heading',{name:'Scan with One Authenticator',exact:true}).waitFor();const secret=(await page.locator('details code').textContent()).trim();assert(/^[A-Z2-7]{32}$/.test(secret));assert.equal(await page.locator('svg').count()>0,true);
 const mobile=await phone.newPage();mobile.on('pageerror',e=>errors.push(e.message));await mobile.goto(host);await mobile.evaluate(()=>navigator.serviceWorker.ready);await mobile.reload();await mobile.waitForFunction(()=>navigator.serviceWorker.controller);
 await mobile.getByRole('button',{name:'Add OneAccount',exact:true}).click();await mobile.getByLabel('OneAccount setup key or provisioning URI').fill(secret);await mobile.getByLabel('Account label',{exact:true}).fill('admin@example.test');await mobile.getByLabel('Device label',{exact:true}).fill('Synthetic offline phone');await mobile.getByLabel('Vault passphrase (14+ characters)',{exact:true}).fill('Independent synthetic UI vault passphrase');await mobile.getByLabel('Confirm vault passphrase',{exact:true}).fill('Independent synthetic UI vault passphrase');await mobile.getByRole('button',{name:'Encrypt and save',exact:true}).click();await mobile.getByRole('button',{name:'Lock vault',exact:true}).waitFor();
 await mobile.getByRole('button',{name:'Lock vault',exact:true}).click();await phone.setOffline(true);
 if(process.env.AUTHENTICATOR_TEST_HOST_PID)process.kill(Number(process.env.AUTHENTICATOR_TEST_HOST_PID));
 await mobile.reload();await mobile.getByLabel('Vault passphrase',{exact:true}).fill('Independent synthetic UI vault passphrase');await mobile.getByRole('button',{name:'Unlock',exact:true}).click();await mobile.getByRole('button',{name:'Lock vault',exact:true}).waitFor();
 const code=(await mobile.locator('.metric').textContent()).replaceAll(' ','');assert(/^\d{6}$/.test(code));await page.getByLabel('Six-digit code from One Authenticator',{exact:true}).fill(code);await page.getByRole('button',{name:'Verify and enable',exact:true}).click();
 await page.getByRole('heading',{name:'Save your ten recovery codes',exact:true}).waitFor();const recovery=(await page.locator('pre').textContent()).trim().split('\n');assert.equal(recovery.length,10);
 await page.getByRole('button',{name:'I have saved my recovery codes',exact:true}).click();await page.getByRole('heading',{name:'Two-factor authentication is ON',exact:true}).waitFor();
 if(process.env.MYACCOUNTS_SCREENSHOT_DIR)await page.screenshot({path:process.env.MYACCOUNTS_SCREENSHOT_DIR+'/oneaccount-security-enabled.png',fullPage:true});
 await page.getByLabel('Current OneAccount password',{exact:true}).fill(password);await page.getByLabel('Current code or recovery code (optional immediately after MFA sign-in)',{exact:true}).fill(recovery[0]);await page.getByLabel('Type DISABLE to remove the authenticator',{exact:true}).fill('DISABLE');await page.getByRole('button',{name:'Disable two-factor authentication',exact:true}).click();await page.waitForURL(url=>url.pathname.startsWith('/login')); 
 assert.deepEqual(errors,[]);console.log('PASS: actual OneAccount settings UI → encrypted PWA provisioning → phone-context offline reload with host stopped → genuine code accepted by server → ten recovery codes → authenticated disable');
}finally{await desktop.close();await phone.close();await browser.close();}
