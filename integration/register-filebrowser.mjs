// Installation helper: credentials are supplied only through the process environment.
import assert from 'node:assert/strict';
import {randomBytes} from 'node:crypto';
import {writeFile} from 'node:fs/promises';
const base=process.env.MYACCOUNTS_ADMIN_API;
const issuer=process.env.MYACCOUNTS_ISSUER;
const email=process.env.MYACCOUNTS_ADMIN_EMAIL;
const password=process.env.MYACCOUNTS_ADMIN_PASSWORD;
const target=new URL(process.env.FILEBROWSER_ORIGIN);
const name=process.env.FILEBROWSER_CLIENT_NAME;
assert(new URL(base).hostname==='localhost');assert(new URL(issuer).protocol==='https:');assert(email&&password);assert(/^[a-z0-9-]+$/.test(name));
assert(target.protocol==='https:' || target.protocol==='http:'&&target.hostname==='localhost');
const jar=new Map();
async function request(route,body){
 const r=await fetch(base+route,{redirect:'manual',method:body?'POST':'GET',headers:{Origin:base,Cookie:[...jar].map(([k,v])=>`${k}=${v}`).join('; '),'Content-Type':'application/json'},body:body?JSON.stringify(body):undefined});
 for(const value of r.headers.getSetCookie()){const first=value.split(';')[0];const i=first.indexOf('=');jar.set(first.slice(0,i),first.slice(i+1));}
 const result=await r.json();assert.equal(result.status,'ok','MyAccounts installation API rejected the operation');return result;
}
await request('/api/login',{organization:'built-in',username:email,password,application:'app-built-in',type:'login',signinMethod:'Password'});
const existing=await request('/api/get-applications?owner=admin');assert(!existing.data.some(a=>a.name===name),'Client already exists; refuse implicit replacement or secret rotation');
const template=(await request('/api/get-application?id=admin/app-built-in')).data;
const clientSecret=randomBytes(48).toString('base64url');
const callback=new URL(process.env.MYACCOUNTS_REDIRECT_PATH||'api/auth/oidc/callback',target.href.endsWith('/')?target.href:target.href+'/').href;
assert.equal(new URL(callback).origin,target.origin,'Callback must stay in the selected relying-party origin');
const application={...template,name,displayName:process.env.MYACCOUNTS_CLIENT_DISPLAY||(name.includes('remote')?'FileBrowser Drive (remote)':'FileBrowser Drive (local)'),clientId:name,clientSecret,redirectUris:[callback],homepageUrl:target.href,grantTypes:['authorization_code'],enableAutoSignin:false,enableSigninSession:true,enableSignUp:false,providers:[],scopes:[]};
await request('/api/add-application',application);
await writeFile(process.env.MYACCOUNTS_CLIENT_OUTPUT,JSON.stringify({issuer,clientId:name,clientSecret,callback}),{mode:0o600,flag:'wx'});
console.log('Registered exact FileBrowser OIDC callback; protected installer handoff required.');
