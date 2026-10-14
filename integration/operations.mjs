import assert from 'node:assert/strict';
const origin=process.env.MYACCOUNTS_TEST_ISSUER;const password=process.env.MYACCOUNTS_TEST_PASSWORD;const email=process.env.MYACCOUNTS_TEST_EMAIL||'admin@example.com';assert(origin&&password);
const jar=new Map();async function request(route,body){const r=await fetch(origin+route,{redirect:'manual',method:body?'POST':'GET',headers:{Origin:origin,Cookie:[...jar].map(([k,v])=>`${k}=${v}`).join('; '),'Content-Type':'application/json'},body:body?JSON.stringify(body):undefined});for(const value of r.headers.getSetCookie()){const item=value.split(';')[0];const i=item.indexOf('=');jar.set(item.slice(0,i),item.slice(i+1));}return r;}
const anonymous=await (await fetch(origin+'/api/myaccounts/status')).json();assert.equal(anonymous.status,'error');assert.equal(anonymous.data??null,null,'Anonymous administrator data was returned');
const login=await (await request('/api/login',{organization:'built-in',username:email,password,application:'app-built-in',type:'login',signinMethod:'Password'})).json();assert.equal(login.status,'ok');
const page=await request('/api/myaccounts/console');assert.equal(page.status,200);assert((await page.text()).includes('Manage users'));
const status=await (await request('/api/myaccounts/status')).json();assert.equal(status.status,'ok');assert.equal(status.data.databaseHealthy,true);
const phrase='synthetic isolated operations backup passphrase';
const backup=await (await request('/api/myaccounts/backup',{passphrase:phrase})).json();assert.equal(backup.status,'ok','Online administrator backup failed');
const download=await request('/api/myaccounts/download?name='+encodeURIComponent(backup.data));assert.equal(download.status,200);assert((await download.arrayBuffer()).byteLength>0);
const traversal=await request('/api/myaccounts/download?name=..%2Fusers.db');assert.equal(traversal.status,400);
const schedule=await (await request('/api/myaccounts/schedule',{hours:1,passphrase:phrase})).json();assert.equal(schedule.status,'ok');
const configured=await (await request('/api/myaccounts/status')).json();assert.equal(configured.data.backupHours,1);assert.equal(configured.data.scheduleCredentialReady,true);
assert.equal((await (await request('/api/myaccounts/schedule',{hours:0})).json()).status,'ok');
console.log('PASS: administrator-only operations page, consistent online backup/download, traversal refusal, protected schedule configure/disable');
