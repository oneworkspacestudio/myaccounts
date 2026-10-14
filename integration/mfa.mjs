import assert from 'node:assert/strict';
import * as OTPAuth from 'otpauth';
const issuer=process.env.MYACCOUNTS_TEST_ISSUER;const password=process.env.MYACCOUNTS_TEST_PASSWORD;
assert(issuer?.startsWith('http://localhost:'));assert(password);
const primary={organization:'built-in',username:'admin@example.com',password,application:'app-built-in',type:'login',signinMethod:'Password'};
function browser(){const jar=new Map();return async(path,body,form=false)=>{const cookie=[...jar].map(([k,v])=>`${k}=${v}`).join('; ');const r=await fetch(issuer+path,{method:body?'POST':'GET',headers:{Origin:issuer,Cookie:cookie,'Content-Type':form?'application/x-www-form-urlencoded':'application/json'},body:body?(form?new URLSearchParams(body).toString():JSON.stringify(body)):undefined});for(const value of r.headers.getSetCookie()){const pair=value.split(';')[0];const i=pair.indexOf('=');const name=pair.slice(0,i);if(/max-age=0(?:;|$)/i.test(value))jar.delete(name);else jar.set(name,pair.slice(i+1));}assert.equal(r.status,200);return r.json();};}
const owner={owner:'built-in',name:'admin',mfaType:'app'};const admin=browser();assert.equal((await admin('/api/login',primary)).status,'ok');
const init=await admin('/api/mfa/setup/initiate',owner,true);assert.equal(init.status,'ok');assert(init.data.secret);assert(init.data.url);assert(init.data.recoveryCodes?.length);
const authenticator=OTPAuth.URI.parse(init.data.url);
assert.equal((await admin('/api/mfa/setup/verify',{...owner,secret:init.data.secret,passcode:authenticator.generate()},true)).status,'ok');
assert.equal((await admin('/api/mfa/setup/enable',{...owner,secret:init.data.secret,recoveryCodes:init.data.recoveryCodes[0]},true)).status,'ok');
async function challenge(){const api=browser();const r=await api('/api/login',primary);assert.equal(r.status,'ok');assert.equal(r.data,'NextMfa','Password alone bypassed enrolled MFA');return api;}
const second={organization:'built-in',username:'',password:'',application:'app-built-in',type:'login',mfaType:'app',enableMfaRemember:false};
const login=await challenge();const valid=authenticator.generate();const wrong=valid==='000000'?'111111':'000000';assert.notEqual((await login('/api/login',{...second,passcode:wrong})).status,'ok','Invalid TOTP accepted');
assert.equal((await login('/api/login',{...second,passcode:valid})).status,'ok','Valid independent authenticator code rejected');
const replay=await challenge();assert.notEqual((await replay('/api/login',{...second,passcode:valid})).status,'ok','TOTP replay accepted');
const [a,b]=await Promise.all([challenge(),challenge()]);const recovery={...second,recoveryCode:init.data.recoveryCodes[0]};const recovered=await Promise.all([a('/api/login',recovery),b('/api/login',recovery)]);assert.equal(recovered.filter(r=>r.status==='ok').length,1,'Concurrent recovery code was not single-use');
const winner=recovered[0].status==='ok'?a:b;
assert.equal((await winner('/api/delete-mfa',owner,true)).status,'ok');
console.log('PASS: independent TOTP enrollment, password/MFA challenge, invalid code refusal, valid code, replay refusal, concurrent single-use recovery');
