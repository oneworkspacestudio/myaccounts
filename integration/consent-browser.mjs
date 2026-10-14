import assert from 'node:assert/strict';
import {randomBytes} from 'node:crypto';
import {pathToFileURL} from 'node:url';
import {createServer} from 'node:http';
import * as oidc from 'openid-client';
const origin=process.env.MYACCOUNTS_TEST_ISSUER,password=process.env.MYACCOUNTS_TEST_PASSWORD;
assert(origin&&password&&process.env.MYACCOUNTS_PLAYWRIGHT);
const {chromium}=await import(pathToFileURL(process.env.MYACCOUNTS_PLAYWRIGHT).href);
const browser=await chromium.launch({executablePath:process.env.OFFICE_BROWSER||'C:/Program Files/Google/Chrome/Application/chrome.exe',headless:true});
const context=await browser.newContext(),callback='http://localhost:19998/callback';
const server=createServer((req,res)=>{res.writeHead(200,{'Content-Type':'text/plain'});res.end('Authorization response received');});
await new Promise((resolve,reject)=>{server.once('error',reject);server.listen(19998,'127.0.0.1',resolve);});
try{
 async function api(route,body){return (await context.request.fetch(origin+route,{method:body?'POST':'GET',headers:{Origin:origin},data:body})).json();}
 assert.equal((await api('/api/login',{organization:'built-in',username:'admin@example.com',password,application:'app-built-in',type:'login',signinMethod:'Password'})).status,'ok');
 const template=(await api('/api/get-account')).data,name='consent-'+randomBytes(6).toString('hex'),credential=randomBytes(32).toString('base64url');
 const user={...template,owner:'myaccounts',name,id:'',email:name+'@example.com',displayName:'Consent Test',emailVerified:true,isAdmin:false,type:'normal-user',passwordType:'plain',passwordSalt:'',password:credential,groups:[],roles:[],permissions:[],applicationScopes:[],webauthnCredentials:[],totpSecret:'',mfaItems:[],recoveryCodes:[],preferredMfaType:'',needUpdatePassword:false};
 assert.equal((await api('/api/add-user',user)).status,'ok');
 const base=(await api('/api/get-application?id=admin/myaccounts')).data,secret=randomBytes(32).toString('hex');
 const app={...base,name,displayName:'Consent integration',clientId:name,clientSecret:secret,redirectUris:[callback],enableAutoSignin:false,enableSigninSession:true,customScopes:[{scope:'openid',displayName:'Your identity',description:'Identify your stable account'},{scope:'profile',displayName:'Basic profile',description:'Read your profile'},{scope:'email',displayName:'Email address',description:'Read your email address'}]};
 assert.equal((await api('/api/add-application',app)).status,'ok');await context.clearCookies();
 const config=await oidc.discovery(new URL(origin),name,secret,oidc.ClientSecretPost(secret),{execute:[oidc.allowInsecureRequests,oidc.enableNonRepudiationChecks]});
 async function authorize(allow){
  await context.clearCookies();const page=await context.newPage(),verifier=oidc.randomPKCECodeVerifier(),state=oidc.randomState(),nonce=oidc.randomNonce();
  const url=oidc.buildAuthorizationUrl(config,{redirect_uri:callback,scope:'openid profile email',state,nonce,code_challenge:await oidc.calculatePKCECodeChallenge(verifier),code_challenge_method:'S256',response_type:'code'});
  await page.goto(url.href);await page.locator('#username').fill(user.email);await page.getByRole('button',{name:'Next',exact:true}).click();await page.locator('#password').fill(credential);await page.getByRole('button',{name:'Sign In',exact:true}).click();
  await page.waitForURL(u=>u.pathname.startsWith('/consent/'));await page.getByRole('button',{name:allow?'Allow':'Deny',exact:true}).click();await page.waitForURL(u=>u.origin===new URL(callback).origin);
  const response=new URL(page.url());assert.equal(response.searchParams.get('state'),state);
  if(allow){const tokens=await oidc.authorizationCodeGrant(config,response,{expectedState:state,expectedNonce:nonce,pkceCodeVerifier:verifier});assert(tokens.claims().sub);}
  else{assert.equal(response.searchParams.get('error'),'access_denied');assert.equal(response.searchParams.has('code'),false);}
  await page.close();
 }
 await authorize(false);assert(!(await api('/api/get-account')).data.applicationScopes.some(r=>r.application==='admin/'+name));
 await authorize(true);let account=(await api('/api/get-account')).data;assert(account.applicationScopes.some(r=>r.application==='admin/'+name));
 const apps=(await api('/api/get-organization-applications?owner=admin&organization=myaccounts')).data;assert(apps.some(a=>a.name===name));assert(!apps.some(a=>a.name==='app-built-in'),'Normal launcher exposed administrator app');assert(apps.every(a=>a.clientSecret!==secret),'Launcher exposed client secret');
 assert.equal((await api('/api/revoke-consent',{application:'admin/'+name,grantedScopes:['openid','profile','email']})).status,'ok');account=(await api('/api/get-account')).data;assert(!account.applicationScopes.some(r=>r.application==='admin/'+name));
 await authorize(false);
 console.log('PASS: real normal-user browser consent allow/deny, verified code/PKCE tokens, connected-app consent removal/re-prompt and organization launcher/secret boundaries');
}finally{await context.close();await browser.close();await new Promise(resolve=>server.close(resolve));}
