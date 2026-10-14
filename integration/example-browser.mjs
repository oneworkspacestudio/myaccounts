import assert from 'node:assert/strict';
import {pathToFileURL} from 'node:url';
const {chromium}=await import(pathToFileURL(process.env.MYACCOUNTS_PLAYWRIGHT).href);
const context=await chromium.launchPersistentContext(process.env.MYACCOUNTS_BROWSER_DATA,{executablePath:'C:/Program Files/Google/Chrome/Application/chrome.exe',headless:true});
try{for(const origin of ['http://localhost:18994','http://localhost:18995']){
 const page=await context.newPage();await page.goto(origin);await page.getByRole('link',{name:'Sign in with MyAccounts'}).click();await page.waitForURL(u=>u.hostname==='login.m8e.co.uk');await page.locator('#username').waitFor({state:'visible'});await page.locator('#username').fill(process.env.MYACCOUNTS_ADMIN_EMAIL);if(!await page.locator('#password').isVisible())await page.getByRole('button',{name:'Next',exact:true}).click();await page.locator('#password').fill(process.env.MYACCOUNTS_ADMIN_PASSWORD);await page.getByRole('button',{name:'Sign In',exact:true}).click();await page.waitForURL(u=>u.origin===origin,{timeout:60000});
 const response=await page.request.get(origin+'/api/me');assert.equal(response.status(),200);assert((await response.json()).sub);await page.getByRole('button',{name:'Sign out',exact:true}).click();assert.equal((await page.request.get(origin+'/api/me')).status(),401);console.log('PASS: real browser '+(origin.endsWith('18994')?'openid-client Node':'Authlib Python')+' code/PKCE callback, verified identity and local sign-out');await page.close();
}}finally{await context.close()}
