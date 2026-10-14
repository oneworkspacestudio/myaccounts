import assert from 'node:assert/strict';
const origin=process.env.MYACCOUNTS_TEST_ISSUER,password=process.env.MYACCOUNTS_TEST_PASSWORD;assert(origin&&password);
function browser(){const jar=new Map();const request=async(route,body)=>{const r=await fetch(origin+route,{method:body?'POST':'GET',headers:{Origin:origin,'Content-Type':'application/json',Cookie:[...jar].map(([k,v])=>`${k}=${v}`).join('; ')},body:body?JSON.stringify(body):undefined});for(const value of r.headers.getSetCookie()){const item=value.split(';')[0],i=item.indexOf('=');jar.set(item.slice(0,i),item.slice(i+1));}return r.json();};return {request,jar};}
const a=browser(),b=browser();const login={organization:'built-in',username:'admin@example.com',password,application:'app-built-in',type:'login',signinMethod:'Password'};
assert.equal((await a.request('/api/login',login)).status,'ok');assert.equal((await b.request('/api/login',login)).status,'ok');
const ownCookie=a.jar.get('casdoor_session_id'),otherCookie=b.jar.get('casdoor_session_id');assert(ownCookie&&otherCookie&&ownCookie!==otherCookie,'Session ID not independently generated');
const list=await a.request('/api/get-sessions?owner=built-in');assert.equal(list.status,'ok');const record=list.data.find(s=>s.name==='admin'&&s.application==='app-built-in');assert(record);
const {createHash}=await import('node:crypto');const hash=createHash('sha256').update(otherCookie).digest('hex');
const revoked=await a.request('/api/delete-session?sessionId='+hash,record);assert.equal(revoked.status,'ok');
const after=await b.request('/api/get-account');assert(!after.data||after.status==='error','Revoked session still authorized');
assert.equal((await a.request('/api/get-account')).status,'ok','Selected revocation killed the caller session');
assert.equal((await b.request('/api/login',login)).status,'ok');const all=await a.request('/api/sso-logout?logoutAll=true',{});assert.equal(all.status,'ok');const caller=await a.request('/api/get-account'),other=await b.request('/api/get-account');assert.equal(caller.status,'error','All-session sign-out left caller authorized');assert.equal(other.status,'error','All-session sign-out left peer authorized');
console.log('PASS: independent browser sessions, selected-session revocation and all-session sign-out');
