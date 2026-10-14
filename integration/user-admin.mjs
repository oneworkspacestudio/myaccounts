import assert from 'node:assert/strict';
import {randomBytes} from 'node:crypto';
const origin=process.env.MYACCOUNTS_TEST_ISSUER,password=process.env.MYACCOUNTS_TEST_PASSWORD;
assert(origin&&password);
function client(){const jar=new Map();return async(route,body,form=false)=>{
 const r=await fetch(origin+route,{method:body?'POST':'GET',headers:{Origin:origin,Cookie:[...jar].map(([k,v])=>`${k}=${v}`).join('; '),'Content-Type':form?'application/x-www-form-urlencoded':'application/json'},body:body?(form?new URLSearchParams(body):JSON.stringify(body)):undefined});
 for(const value of r.headers.getSetCookie()){const pair=value.split(';')[0],i=pair.indexOf('=');jar.set(pair.slice(0,i),pair.slice(i+1));}
 return r.json();
};}
const admin=client(),suffix=randomBytes(6).toString('hex'),name='managed-'+suffix;
const initial=randomBytes(32).toString('base64url'),next=randomBytes(32).toString('base64url');
assert.equal((await admin('/api/login',{organization:'built-in',username:'admin@example.com',password,application:'app-built-in',type:'login',signinMethod:'Password'})).status,'ok');
const status=await admin('/api/myaccounts/status');assert.equal(status.status,'ok');assert.equal(status.data.sharedEmailConnected,false,'This fixture must prove administration without email');
const user={owner:'myaccounts',name,displayName:'Managed account fixture',email:name+'@example.com',emailVerified:false,type:'normal-user',isAdmin:false,passwordType:'plain',password:initial,signupApplication:'myaccounts',createdTime:new Date().toISOString(),properties:{}};
assert.equal((await admin('/api/add-user',user)).status,'ok','Administrator could not create a user without email');
const id='myaccounts/'+name;
let saved=(await admin('/api/get-user?id='+id)).data;assert(saved);assert.equal(saved.isAdmin,false);assert.equal(saved.emailVerified,false);
saved.displayName='Edited account fixture';
assert.equal((await admin('/api/update-user?id='+id+'&columns=displayName',saved)).status,'ok');
assert.equal((await admin('/api/get-user?id='+id)).data.displayName,saved.displayName);
const login=(api,pass)=>api('/api/login',{organization:'myaccounts',username:name,password:pass,application:'myaccounts',type:'login',signinMethod:'Password'});
const member=client();assert.equal((await login(member,initial)).status,'ok','Unverified email blocked administrator-created user');
assert.equal((await member('/api/myaccounts/status')).status,'error','Ordinary user accessed administration');
assert.equal((await member('/api/add-user',{...user,name:name+'-forbidden'})).status,'error','Ordinary user added an account');
assert.equal((await member('/api/set-password',{userOwner:'built-in',userName:'admin',newPassword:next},true)).status,'error','Ordinary user reset the administrator password');
assert.equal((await member('/api/delete-user',{owner:'built-in',name:'admin'})).status,'error');
// Reset requires neither the old password, a mailbox, nor a recovery code for an authorized administrator.
assert.equal((await admin('/api/set-password',{userOwner:'myaccounts',userName:name,newPassword:next},true)).status,'ok','Direct administrator reset required email');
const oldSession=await member('/api/get-account');assert(!oldSession.data||oldSession.status==='error','Reset left an old session active');
assert.equal((await login(client(),initial)).status,'error','Old password survived administrator reset');
assert.equal((await login(client(),next)).status,'ok','New administrator-set password failed');
assert.equal((await admin('/api/delete-user',{owner:'myaccounts',name})).status,'ok');
assert.equal((await login(client(),next)).status,'error','Deleted account still signed in');
assert.equal((await admin('/api/delete-user',{owner:'built-in',name:'admin'})).status,'error','Built-in administrator could be removed');
if(process.env.MYACCOUNTS_PLAYWRIGHT){
 const {chromium}=await import('@playwright/test');const {join}=await import('node:path');
 const context=await chromium.launchPersistentContext(join(process.env.MYACCOUNTS_FIXTURE,'admin-browser'),{channel:'chrome',headless:true});
 try{
  const auth=await context.request.post(origin+'/api/login',{headers:{Origin:origin},data:{organization:'built-in',username:'admin@example.com',password,application:'app-built-in',type:'login',signinMethod:'Password'}});assert.equal((await auth.json()).status,'ok');
  const page=context.pages()[0];await page.goto(origin+'/operations');await page.getByRole('heading',{name:'Administration',exact:true}).waitFor();
  await page.getByRole('link',{name:'Manage users',exact:true}).click();await page.locator('#add-button').click();
  const field=label=>page.locator('label').filter({hasText:new RegExp('^'+label+'[ *]*$')}).locator('..').locator('input').first();
  await field('Name').fill(name+'-ui');await field('Display name').fill('Browser managed fixture');await field('Email').fill(name+'-ui@example.com');
  // New accounts must have an administrator-chosen password rather than a shared default.
  assert.equal(await page.locator('input[type=password]').first().inputValue(),'');await page.locator('input[type=password]').first().fill(initial);
  let response=page.waitForResponse(r=>r.url().includes('/api/add-user')&&r.request().method()==='POST');await page.getByRole('button',{name:'Save',exact:true}).click();assert.equal((await (await response).json()).status,'ok');
  await page.getByRole('button',{name:/^(Modify|Set) password/}).waitFor();
  await field('Display name').fill('Browser edited fixture');response=page.waitForResponse(r=>r.url().includes('/api/update-user')&&r.request().method()==='POST');await page.getByRole('button',{name:'Save',exact:true}).click();assert.equal((await (await response).json()).status,'ok');
  await page.getByRole('button',{name:/^(Modify|Set) password/}).click();assert.equal(await page.locator('#old-password').count(),0);
  await page.locator('#new-password').fill(next);await page.locator('#re-password').fill(next);
  response=page.waitForResponse(r=>r.url().includes('/api/set-password'));await page.getByRole('button',{name:'Set Password',exact:true}).click();assert.equal((await (await response).json()).status,'ok');
  assert.equal((await client()('/api/login',{organization:'myaccounts',username:name+'-ui',password:next,application:'myaccounts',type:'login',signinMethod:'Password'})).status,'ok');
  await page.goto(origin+'/organizations/myaccounts/users');
  const row=page.getByRole('row').filter({has:page.getByRole('link',{name:name+'-ui',exact:true})});
  await row.getByRole('button',{name:/Delete/}).click();
  response=page.waitForResponse(r=>r.url().includes('/api/delete-user'));await page.getByRole('alertdialog').getByRole('button',{name:/^(OK|Delete|Confirm)$/}).click();assert.equal((await (await response).json()).status,'ok');
  console.log('PASS: real Chrome admin panel link, new-user form, profile edit, password modal without old-password/email and deletion');
 }finally{await context.close();await admin('/api/delete-user',{owner:'myaccounts',name:name+'-ui'});}
}
console.log('PASS: no-email administrator create/edit/delete/direct password reset, old password/session refusal, unverified user sign-in and ordinary-user access denial');
