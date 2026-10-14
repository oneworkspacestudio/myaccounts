import assert from 'node:assert/strict';
import {randomBytes} from 'node:crypto';
import * as oidc from 'openid-client';

// An independently maintained relying party verifies the engine's real tokens.
// A disposable installation supplies credentials through process-only environment variables.
const issuer=process.env.MYACCOUNTS_TEST_ISSUER;
const password=process.env.MYACCOUNTS_TEST_PASSWORD;
assert(issuer?.startsWith('http://localhost:'),'Use an isolated localhost test installation');
assert(password,'Missing test credential');
let cookie='';const jar=new Map();
async function api(path,body){
 const response=await fetch(issuer+path,{method:body?'POST':'GET',headers:{Origin:issuer,'Content-Type':'application/json',Cookie:cookie},body:body?JSON.stringify(body):undefined});
 const cookies=response.headers.getSetCookie();for(const value of cookies){const pair=value.split(';')[0];const i=pair.indexOf('=');const name=pair.slice(0,i);if(/max-age=0(?:;|$)/i.test(value))jar.delete(name);else jar.set(name,pair.slice(i+1));}cookie=[...jar].map(([name,value])=>`${name}=${value}`).join('; ');
 assert.equal(response.status,200,`HTTP failure at ${path.split('?')[0]}`);return response.json();
}
const baseForm={organization:'built-in',username:'admin@example.com',password,application:'app-built-in',signinMethod:'Password',type:'login'};
assert.equal((await api('/api/login',baseForm)).status,'ok','Administrator login');
const result=await api('/api/get-application?id=admin/app-built-in');assert.equal(result.status,'ok');
const clientId='integration-'+randomBytes(8).toString('hex');const secret=randomBytes(32).toString('hex');
const callback='http://localhost:19999/callback';
const app={...result.data,name:clientId,displayName:'Independent OIDC test',clientId,clientSecret:secret,redirectUris:[callback],grantTypes:['authorization_code'],enableAutoSignin:false,enableSigninSession:true,enableSignUp:false,promptPage:[],providers:[],scopes:[]};
assert.equal((await api('/api/add-application',app)).status,'ok','Client registration');
const config=await oidc.discovery(new URL(issuer),clientId,secret,oidc.ClientSecretPost(secret),{execute:[oidc.allowInsecureRequests,oidc.enableNonRepudiationChecks]});
assert.equal(config.serverMetadata().issuer,issuer);
async function getCode(redirect=callback){
 const verifier=oidc.randomPKCECodeVerifier();const challenge=await oidc.calculatePKCECodeChallenge(verifier);const state=oidc.randomState();const nonce=oidc.randomNonce();
 const auth=oidc.buildAuthorizationUrl(config,{redirect_uri:redirect,scope:'openid profile email',state,nonce,code_challenge:challenge,code_challenge_method:'S256',response_type:'code'});
 const query=new URLSearchParams({clientId,responseType:'code',redirectUri:redirect,scope:auth.searchParams.get('scope'),state,nonce,code_challenge:challenge,code_challenge_method:'S256'});
 const response=await api('/api/login?'+query,{...baseForm,type:'code',application:clientId});
 return {response,verifier,state,nonce};
}
function responseUrl(flow){const url=new URL(callback);url.searchParams.set('code',flow.response.data);url.searchParams.set('state',flow.state);if(config.serverMetadata().authorization_response_iss_parameter_supported)url.searchParams.set('iss',issuer);return url;}
const invalid=await getCode('https://attacker.example/callback');assert.notEqual(invalid.response.status,'ok','Invalid redirect accepted');
const wrong=await getCode();assert.equal(wrong.response.status,'ok');await assert.rejects(()=>oidc.authorizationCodeGrant(config,responseUrl(wrong),{expectedState:wrong.state,expectedNonce:wrong.nonce,pkceCodeVerifier:oidc.randomPKCECodeVerifier()}),'Wrong PKCE verifier accepted');
const valid=await getCode();assert.equal(valid.response.status,'ok');const tokens=await oidc.authorizationCodeGrant(config,responseUrl(valid),{expectedState:valid.state,expectedNonce:valid.nonce,pkceCodeVerifier:valid.verifier});
assert(tokens.id_token);assert(tokens.access_token);const claims=tokens.claims();assert.equal(claims.iss,issuer);assert(claims.sub);assert.equal(claims.nonce,valid.nonce);
const user=await oidc.fetchUserInfo(config,tokens.access_token,claims.sub);assert.equal(user.sub,claims.sub);assert.equal(user.email,'admin@example.com');
await assert.rejects(()=>oidc.authorizationCodeGrant(config,responseUrl(valid),{expectedState:valid.state,expectedNonce:valid.nonce,pkceCodeVerifier:valid.verifier}),'Code replay accepted');
const nonceFlow=await getCode();assert.equal(nonceFlow.response.status,'ok');await assert.rejects(()=>oidc.authorizationCodeGrant(config,responseUrl(nonceFlow),{expectedState:nonceFlow.state,expectedNonce:'incorrect-nonce',pkceCodeVerifier:nonceFlow.verifier}),'Nonce mismatch accepted');
const stateFlow=await getCode();assert.equal(stateFlow.response.status,'ok');await assert.rejects(()=>oidc.authorizationCodeGrant(config,responseUrl(stateFlow),{expectedState:'incorrect-state',expectedNonce:stateFlow.nonce,pkceCodeVerifier:stateFlow.verifier}),'State mismatch accepted');
// Add a replacement key without deleting the old verification key during overlap.
const keyName='rotation-'+randomBytes(8).toString('hex');
assert.equal((await api('/api/add-cert',{owner:'admin',name:keyName,displayName:'Isolated rotation key',scope:'JWT',type:'x509',cryptoAlgorithm:'RS256',bitSize:4096,expireInYears:10})).status,'ok','Signing key creation');
assert.equal((await api('/api/update-application?id=admin/'+clientId+'&columns=cert',{...app,cert:keyName})).status,'ok','Signing key rotation');app.cert=keyName;
const keyFlow=await getCode();assert.equal(keyFlow.response.status,'ok');
// A fresh relying-party discovery models a client after its JWKS refresh window.
// Existing clients may retain keys briefly: publish before activating a new key.
const refreshed=await oidc.discovery(new URL(issuer),clientId,secret,oidc.ClientSecretPost(secret),{execute:[oidc.allowInsecureRequests,oidc.enableNonRepudiationChecks]});
const keyTokens=await oidc.authorizationCodeGrant(refreshed,responseUrl(keyFlow),{expectedState:keyFlow.state,expectedNonce:keyFlow.nonce,pkceCodeVerifier:keyFlow.verifier});
assert.equal(keyTokens.claims().sub,claims.sub,'Key rotation changed identity');
const header=token=>JSON.parse(Buffer.from(token.split('.')[0],'base64url'));
assert.notEqual(header(keyTokens.id_token).kid,header(tokens.id_token).kid,'Signing key did not change');
const overlap=await (await fetch(config.serverMetadata().jwks_uri)).json();
assert(overlap.keys.some(k=>k.kid===header(tokens.id_token).kid),'Old verification key disappeared before tokens expired');
assert(overlap.keys.some(k=>k.kid===header(keyTokens.id_token).kid),'New verification key missing from JWKS');
assert.equal((await oidc.fetchUserInfo(config,tokens.access_token,claims.sub)).sub,claims.sub,'Old access token failed during key overlap');
const rotatedSecret=randomBytes(32).toString('hex');assert.equal((await api('/api/update-application?id=admin/'+clientId+'&columns=clientSecret',{...app,clientSecret:rotatedSecret})).status,'ok','Secret rotation');
const afterRotation=await getCode();assert.equal(afterRotation.response.status,'ok');await assert.rejects(()=>oidc.authorizationCodeGrant(config,responseUrl(afterRotation),{expectedState:afterRotation.state,expectedNonce:afterRotation.nonce,pkceCodeVerifier:afterRotation.verifier}),'Old client secret accepted');
console.log('PASS: independent openid-client discovery, code + PKCE, JWKS signature, nonce/state, UserInfo, redirect/replay refusal, client-secret and overlapping signing-key rotation');
