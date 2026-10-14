import assert from 'node:assert/strict';
import {randomBytes} from 'node:crypto';
import {pathToFileURL} from 'node:url';
const origin=process.env.MYACCOUNTS_TEST_ISSUER,password=process.env.MYACCOUNTS_TEST_PASSWORD;
assert(origin&&password&&process.env.MYACCOUNTS_PLAYWRIGHT);
const {chromium}=await import(pathToFileURL(process.env.MYACCOUNTS_PLAYWRIGHT).href);
const browser=await chromium.launch({executablePath:process.env.OFFICE_BROWSER||'C:/Program Files/Google/Chrome/Application/chrome.exe',headless:true});
const context=await browser.newContext();
try{
 const page=await context.newPage();await page.goto(origin);
 const cdp=await context.newCDPSession(page);await cdp.send('WebAuthn.enable');
 await cdp.send('WebAuthn.addVirtualAuthenticator',{options:{protocol:'ctap2',transport:'internal',hasResidentKey:true,hasUserVerification:true,isUserVerified:true,automaticPresenceSimulation:true}});
 async function api(route,body){const r=await context.request.fetch(origin+route,{method:body?'POST':'GET',headers:{Origin:origin},data:body});return r.json();}
 assert.equal((await api('/api/login',{organization:'built-in',username:'admin@example.com',password,application:'app-built-in',type:'login',signinMethod:'Password'})).status,'ok');
 const template=(await api('/api/get-account')).data,name='passkey-'+randomBytes(6).toString('hex'),credential=randomBytes(32).toString('base64url');
 const user={...template,owner:'myaccounts',name,id:'',email:name+'@example.com',displayName:'Passkey Test',emailVerified:true,isAdmin:false,type:'normal-user',passwordType:'plain',passwordSalt:'',password:credential,groups:[],roles:[],permissions:[],webauthnCredentials:[],totpSecret:'',mfaItems:[],recoveryCodes:[],preferredMfaType:'',needUpdatePassword:false};
 assert.equal((await api('/api/add-user',user)).status,'ok');await context.clearCookies();
 assert.equal((await api('/api/login',{organization:'myaccounts',username:user.email,password:credential,application:'myaccounts',type:'login',signinMethod:'Password'})).status,'ok');
 async function ceremony(options,registration){return page.evaluate(async({options,registration})=>{
  const decode=s=>Uint8Array.from(atob(s.replace(/-/g,'+').replace(/_/g,'/')),c=>c.charCodeAt(0));
  const encode=b=>btoa(String.fromCharCode(...new Uint8Array(b))).replace(/\+/g,'-').replace(/\//g,'_').replace(/=+$/,'');
  const p=options.publicKey;p.challenge=decode(p.challenge);if(p.user)p.user.id=decode(p.user.id);
  for(const key of ['excludeCredentials','allowCredentials'])for(const item of p[key]||[])item.id=decode(item.id);
  const c=registration?await navigator.credentials.create({publicKey:p}):await navigator.credentials.get({publicKey:p});
  const response={clientDataJSON:encode(c.response.clientDataJSON)};
  if(registration){response.attestationObject=encode(c.response.attestationObject);response.transports=c.response.getTransports();}
  else{response.authenticatorData=encode(c.response.authenticatorData);response.signature=encode(c.response.signature);response.userHandle=c.response.userHandle?encode(c.response.userHandle):null;}
  return {id:c.id,rawId:encode(c.rawId),type:c.type,response,clientExtensionResults:c.getClientExtensionResults()};
 },{options,registration});}
 const registration=await ceremony(await api('/api/webauthn/signup/begin'),true);
 assert.equal((await api('/api/webauthn/signup/finish',registration)).status,'ok','Passkey registration failed');
 assert.equal((await api('/api/webauthn/signup/finish',registration)).status,'error','Registration challenge replay accepted');
 await context.clearCookies();const begin='/api/webauthn/signin/begin?owner=myaccounts&name='+name,finish='/api/webauthn/signin/finish?responseType=login&application=myaccounts';
 const invalid=await ceremony(await api(begin),false);const client=JSON.parse(Buffer.from(invalid.response.clientDataJSON,'base64url'));client.origin='https://invalid.example';invalid.response.clientDataJSON=Buffer.from(JSON.stringify(client)).toString('base64url');
 assert.equal((await api(finish,invalid)).status,'error','Wrong-origin assertion accepted');
 const assertion=await ceremony(await api(begin),false);assert.equal((await api(finish,assertion)).status,'ok','Passkey sign-in failed');
 const account=(await api('/api/get-account')).data;assert.equal(account.name,name);assert.equal(account.isAdmin,false);
 assert.equal((await api(finish,assertion)).status,'error','Authentication challenge replay accepted');
 await context.clearCookies();await page.goto(origin+'/login/myaccounts');
 await page.locator('#username').fill(user.email);
 await page.getByRole('tab',{name:'WebAuthn',exact:true}).click();
 await page.getByRole('button',{name:'Sign in with WebAuthn',exact:true}).click();
 await page.waitForURL(u=>!u.pathname.startsWith('/login'));
 assert.equal((await api('/api/get-account')).data.name,name,'Passkey UI did not establish the normal-user session');
 console.log('PASS: native normal-user WebAuthn registration/sign-in with Chrome CTAP2 authenticator, wrong-origin and challenge replay refusal');
}finally{await context.close();await browser.close();}
