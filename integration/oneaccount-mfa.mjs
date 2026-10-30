import assert from 'node:assert/strict';
import {randomBytes} from 'node:crypto';
import * as OTPAuth from 'otpauth';
import * as oidc from 'openid-client';
import {DatabaseSync} from 'node:sqlite';
import path from 'node:path';
const issuer=process.env.MYACCOUNTS_TEST_ISSUER,password=process.env.MYACCOUNTS_TEST_PASSWORD;
assert(issuer?.startsWith('http://localhost:')&&password,'Use a protected synthetic fixture');
assert(process.env.MYACCOUNTS_FIXTURE&&process.env.MYACCOUNTS_EXE,'Fixture ownership must be supplied by the fresh smoke harness');
const fixtureDB=new DatabaseSync(path.join(process.env.MYACCOUNTS_FIXTURE,'users.db'));
fixtureDB.exec('PRAGMA busy_timeout=10000');
const testStarted=Date.now();
const primary={organization:'built-in',username:'admin@example.com',password,application:'app-built-in',type:'login',signinMethod:'Password'};
const owner={owner:'built-in',name:'admin',mfaType:'app'};
function browser(){const jar=new Map();return async(path,body,form=false,headers={})=>{
 const response=await fetch(issuer+path,{method:body?'POST':'GET',headers:{Origin:issuer,Cookie:[...jar].map(([k,v])=>`${k}=${v}`).join('; '),'Content-Type':form?'application/x-www-form-urlencoded':'application/json',...headers},body:body?(form?new URLSearchParams(body).toString():JSON.stringify(body)):undefined});
 for(const value of response.headers.getSetCookie()){const pair=value.split(';')[0],i=pair.indexOf('=');if(/max-age=0(?:;|$)/i.test(value))jar.delete(pair.slice(0,i));else jar.set(pair.slice(0,i),pair.slice(i+1));}
 try{return await response.json();}catch{return {status:'error',httpStatus:response.status};}
 };}
const admin=browser(),old=browser(),anonymous=browser();
assert.equal((await admin('/api/login',primary)).status,'ok');assert.equal((await old('/api/login',primary)).status,'ok');
const template=(await admin('/api/get-application?id=admin/app-built-in')).data;
const clientId='mfa-'+randomBytes(8).toString('hex'),clientSecret=randomBytes(32).toString('hex'),callback='http://localhost:19999/mfa-callback';
const app={...template,name:clientId,displayName:'Synthetic MFA SSO',clientId,clientSecret,redirectUris:[callback],grantTypes:['authorization_code','password','refresh_token'],enableAutoSignin:false,enableSigninSession:true,enableSignUp:false,promptPage:[],providers:[],scopes:[]};
assert.equal((await admin('/api/add-application',app)).status,'ok');
const config=await oidc.discovery(new URL(issuer),clientId,clientSecret,oidc.ClientSecretPost(clientSecret),{execute:[oidc.allowInsecureRequests,oidc.enableNonRepudiationChecks]});
async function authorize(api){const verifier=oidc.randomPKCECodeVerifier(),challenge=await oidc.calculatePKCECodeChallenge(verifier),nonce=oidc.randomNonce(),state=oidc.randomState();const query=new URLSearchParams({clientId,responseType:'code',redirectUri:callback,scope:'openid profile email',state,nonce,code_challenge:challenge,code_challenge_method:'S256'});return {response:await api('/api/login?'+query,{type:'code',application:clientId}),verifier,nonce,state};}
async function exchange(f){const url=new URL(callback);url.searchParams.set('code',f.response.data);url.searchParams.set('state',f.state);if(config.serverMetadata().authorization_response_iss_parameter_supported)url.searchParams.set('iss',issuer);return oidc.authorizationCodeGrant(config,url,{expectedState:f.state,expectedNonce:f.nonce,pkceCodeVerifier:f.verifier});}
const before=await authorize(old);assert.equal(before.response.status,'ok');const oldTokens=await exchange(before);
assert.equal((await anonymous('/api/mfa/setup/initiate',owner,true)).status,'error');assert.equal((await admin('/api/mfa/setup/initiate',owner,true)).status,'error','Missing step-up accepted');
const passwordProof=api=>api('/api/check-user-password',{owner:'built-in',name:'admin',password});
assert.equal((await passwordProof(admin)).status,'ok');let init=await admin('/api/mfa/setup/initiate',{...owner,deviceLabel:'Synthetic phone'},true);assert.equal(init.status,'ok');assert(init.data.secret&&init.data.url);assert.equal(init.data.recoveryCodes,undefined);
assert.equal((await admin('/api/mfa/setup/enable',{...owner,secret:init.data.secret,recoveryCodes:'chosen-by-client'},true)).status,'error','Unverified activation accepted');
assert.equal((await admin('/api/mfa/setup/initiate',{...owner,action:'cancel'},true)).status,'ok');assert.equal((await admin('/api/mfa/setup/verify',{...owner,passcode:'000000'},true)).status,'error');
init=await admin('/api/mfa/setup/initiate',owner,true);fixtureDB.exec('UPDATE oneaccount_mfa_pending SET expires=0');assert.equal((await admin('/api/mfa/setup/verify',{...owner,passcode:OTPAuth.URI.parse(init.data.url).generate()},true)).status,'error','Expired pending secret accepted');assert.equal((await admin('/api/mfa/setup/enable',owner,true)).status,'error');
init=await admin('/api/mfa/setup/initiate',owner,true);const authenticator=OTPAuth.URI.parse(init.data.url);assert.equal(authenticator.issuer,'OneAccount');assert.equal(authenticator.period,30);assert.equal(authenticator.digits,6);
assert.equal((await admin('/api/mfa/setup/verify',{...owner,secret:'ATTACKER-CHOSEN-SECRET',passcode:authenticator.generate()},true)).status,'ok','Pending secret was not bound on server');
const enabled=await admin('/api/mfa/setup/enable',owner,true);assert.equal(enabled.status,'ok');const recovery=enabled.data.recoveryCodes;assert.equal(new Set(recovery).size,10);assert(recovery.every(c=>c.replaceAll('-','').length===32));
const status=await admin('/api/mfa/setup/initiate',{...owner,action:'status'},true);assert.equal(status.data.enabled,true);assert.equal(status.data.recoveryCodesRemaining,10);
const profile=await admin('/api/get-account');assert(!JSON.stringify(profile).includes(init.data.secret));assert(!JSON.stringify(profile).includes(recovery[0]));
assert.notEqual((await old('/api/get-account')).status,'ok','Old cookie retained privileged access');
await assert.rejects(()=>oidc.fetchUserInfo(config,oldTokens.access_token,oldTokens.claims().sub),'Pre-MFA token remained usable');await assert.rejects(()=>oidc.refreshTokenGrant(config,oldTokens.refresh_token),'Pre-MFA refresh remained usable');
const pg=await fetch(issuer+'/api/login/oauth/access_token',{method:'POST',headers:{'Content-Type':'application/x-www-form-urlencoded'},body:new URLSearchParams({grant_type:'password',client_id:clientId,client_secret:clientSecret,username:'admin@example.com',password,scope:'openid profile email'})});assert(!(await pg.json()).access_token,'Password grant bypassed MFA');
async function challenge(){const api=browser(),r=await api('/api/login',primary);assert.equal(r.status,'ok');assert.equal(r.data,'NextMfa','Password-only login bypass');assert.notEqual((await api('/api/get-account')).status,'ok','Pre-MFA API bypass');return api;}
const second={organization:'built-in',username:'',password:'',application:'app-built-in',type:'login',mfaType:'app',enableMfaRemember:true};
const expiredChallenge=await challenge();fixtureDB.exec('UPDATE oneaccount_mfa_challenge SET expires=0 WHERE used=0');assert.equal((await expiredChallenge('/api/login',{...second,passcode:authenticator.generate()})).status,'error','Expired pre-MFA challenge accepted');
async function nextCode(){await new Promise(r=>setTimeout(r,30000-Date.now()%30000+150));return authenticator.generate();}
const login=await challenge(),valid=await nextCode();assert.notEqual((await login('/api/login',{...second,passcode:valid==='000000'?'111111':'000000'})).status,'ok');assert.notEqual((await login('/api/login',{...second,passcode:authenticator.generate({timestamp:Date.now()-90000})})).status,'ok','Expired OTP accepted');assert.equal((await login('/api/login',{...second,passcode:valid})).status,'ok');
const flow=await authorize(login);assert.equal(flow.response.status,'ok');const tokens=await exchange(flow);assert(tokens.access_token&&tokens.id_token);assert.equal((await oidc.fetchUserInfo(config,tokens.access_token,tokens.claims().sub)).sub,tokens.claims().sub);
const replay=await challenge();assert.notEqual((await replay('/api/login',{...second,passcode:valid})).status,'ok','TOTP replay');
const recovered=await challenge();assert.equal((await recovered('/api/login',{...second,recoveryCode:recovery[0]})).status,'ok');
const reuse=await challenge();assert.notEqual((await reuse('/api/login',{...second,recoveryCode:recovery[0].toLowerCase().replaceAll('-','')})).status,'ok','Recovery reuse');
const [a,b]=await Promise.all([challenge(),challenge()]),code=await nextCode();const results=await Promise.all([a('/api/login',{...second,passcode:code}),b('/api/login',{...second,passcode:code})]);assert.equal(results.filter(r=>r.status==='ok').length,1,'Concurrent OTP replay');
// Advance only the synthetic DB's bucket epoch to model a five-minute wait.
// This is fixture state, never an endpoint or a production limit override.
await new Promise(resolve=>setTimeout(resolve,Math.max(0,61000-(Date.now()-testStarted))));
fixtureDB.exec('UPDATE oneaccount_mfa_limits SET window=window-1');
const [ra,rb]=await Promise.all([challenge(),challenge()]);const recoveryResults=await Promise.all([ra('/api/login',{...second,recoveryCode:recovery[2]}),rb('/api/login',{...second,recoveryCode:recovery[2].toLowerCase().replaceAll('-','')})]);assert.equal(recoveryResults.filter(r=>r.status==='ok').length,1,'Concurrent normalized recovery reuse');
const winner=recoveryResults[0].status==='ok'?ra:rb;assert.equal((await winner('/api/delete-mfa',owner,true)).status,'error');assert.equal((await passwordProof(winner)).status,'ok');
const regenerated=await winner('/api/mfa/setup/initiate',{...owner,action:'regenerate',recoveryCode:recovery[3]},true);assert.equal(regenerated.status,'ok');const newRecovery=regenerated.data.recoveryCodes;assert.equal(new Set(newRecovery).size,10);
const oldRecovery=await challenge();assert.equal((await oldRecovery('/api/login',{...second,recoveryCode:recovery[4]})).status,'error','Old recovery set survived regeneration');const newLogin=await challenge();assert.equal((await newLogin('/api/login',{...second,recoveryCode:newRecovery[0]})).status,'ok');
assert.equal((await passwordProof(newLogin)).status,'ok');assert.equal((await newLogin('/api/delete-mfa',owner,true)).status,'ok','Verified disable');
const manager=browser();assert.equal((await manager('/api/login',primary)).status,'ok');
assert.equal((await browser()('/api/login',primary)).data,'built-in/admin','Opt-in migration path');assert.equal((await passwordProof(manager)).status,'ok');const pending=await manager('/api/mfa/setup/initiate',owner,true);assert.equal(pending.status,'ok');
let blocked=false;for(let i=0;i<11;i++){const r=await manager('/api/mfa/setup/verify',{...owner,passcode:'000000'},true,{'X-Forwarded-For':`198.51.100.${i+1}`});if(r.code==='mfa_rate_limited')blocked=true;}assert(blocked,'Guessing protection reset across challenge/IP/flow');
assert.equal((await manager('/api/mfa/setup/verify',{...owner,passcode:OTPAuth.URI.parse(pending.data.url).generate()},true)).status,'error');await manager('/api/mfa/setup/initiate',{...owner,action:'cancel'},true);fixtureDB.close();
console.log('PASS: native pending/cancel/activation, ten recovery codes, prior session/token/refresh invalidation, password-grant denial, password→TOTP→signed OIDC SSO, invalid/replay/concurrent OTP, recovery reuse, step-up disable and cross-flow guessing limits');
