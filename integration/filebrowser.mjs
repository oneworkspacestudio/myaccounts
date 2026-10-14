import assert from 'node:assert/strict';
import {randomBytes} from 'node:crypto';
import {spawn} from 'node:child_process';
import {mkdir,writeFile} from 'node:fs/promises';
import path from 'node:path';
const issuer=process.env.MYACCOUNTS_TEST_ISSUER;const password=process.env.MYACCOUNTS_TEST_PASSWORD;
const exe=process.env.MYACCOUNTS_FILEBROWSER_EXE;const runtime=process.env.MYACCOUNTS_FILEBROWSER_DATA;
assert(issuer?.startsWith('http://localhost:'));assert(password);assert(exe);assert(runtime);
const origin='http://localhost:19998';const callback=origin+'/api/auth/oidc/callback';
function browser(base){const jar=new Map();return async(relative,body)=>{const r=await fetch(base+relative,{redirect:'manual',method:body?'POST':'GET',headers:{Origin:base,Cookie:[...jar].map(([k,v])=>`${k}=${v}`).join('; '),'Content-Type':'application/json'},body:body?JSON.stringify(body):undefined});for(const value of r.headers.getSetCookie()){const p=value.split(';')[0];const i=p.indexOf('=');const k=p.slice(0,i);if(/max-age=(?:0|-1)(?:;|$)/i.test(value))jar.delete(k);else jar.set(k,p.slice(i+1));}return r;};}
const provider=browser(issuer);const primary={organization:'built-in',username:'admin@example.com',password,application:'app-built-in',type:'login',signinMethod:'Password'};
assert.equal((await (await provider('/api/login',primary)).json()).status,'ok');
const account=await (await provider('/api/get-account')).json();assert.equal(account.status,'ok');const subject=account.data.id;assert(subject);
const template=await (await provider('/api/get-application?id=admin/app-built-in')).json();assert.equal(template.status,'ok');
const id='filebrowser-'+randomBytes(8).toString('hex');const secret=randomBytes(32).toString('hex');
const app={...template.data,name:id,displayName:'FileBrowser test',clientId:id,clientSecret:secret,redirectUris:[callback],grantTypes:['authorization_code'],enableAutoSignin:false,enableSigninSession:true,enableSignUp:false,providers:[],scopes:[]};
assert.equal((await (await provider('/api/add-application',app)).json()).status,'ok');
const source=path.join(runtime,'files');await mkdir(source,{recursive:true});await writeFile(path.join(source,'hello.txt'),'MyAccounts isolated SSO fixture\n');
const config={server:{database:{path:path.join(runtime,'filebrowser.sqlite')},sources:[{path:source,name:'SSO',config:{defaultEnabled:true}}],disableUpdateCheck:true,disablePreviews:true,cacheDir:path.join(runtime,'cache')},http:{listen:'127.0.0.1',port:19998,baseURL:'/'},auth:{methods:{password:{enabled:true,adminUsername:'bootstrap-admin'},oidc:{enabled:true,clientId:id,issuerUrl:issuer,scopes:'openid profile email',userIdentifier:'sub',disableVerifyTLS:false}}}};
const configPath=path.join(runtime,'config.yaml');await writeFile(configPath,JSON.stringify(config,null,2));
const child=spawn(exe,['-c',configPath],{cwd:runtime,windowsHide:true,env:{...process.env,FILEBROWSER_OIDC_CLIENT_SECRET:secret,FILEBROWSER_ADMIN_PASSWORD:randomBytes(32).toString('hex')}});
let diagnostics='';child.stdout.on('data',b=>{diagnostics+=b.toString();});child.stderr.on('data',b=>{diagnostics+=b.toString();});
try{
 let ready=false;for(let i=0;i<120;i++){if(child.exitCode!==null)throw Error(`Isolated FileBrowser stopped (${child.exitCode}); inspect configuration compatibility`);try{const r=await fetch(origin+'/');if(r.status===200){ready=true;break}}catch{};await new Promise(r=>setTimeout(r,500));}assert(ready,'Isolated FileBrowser startup timeout');
 const browserClient=browser(origin);const initiate=await browserClient('/api/auth/oidc/login');assert.equal(initiate.status,302);const auth=new URL(initiate.headers.get('location'));assert.equal(auth.origin,new URL(issuer).origin);assert.equal(auth.searchParams.get('redirect_uri'),callback);
 const query=new URLSearchParams({clientId:id,responseType:'code',redirectUri:callback,scope:auth.searchParams.get('scope'),state:auth.searchParams.get('state')});
 for(const name of ['nonce','code_challenge','code_challenge_method'])if(auth.searchParams.has(name))query.set(name,auth.searchParams.get(name));
 const login=await (await provider('/api/login?'+query,{...primary,type:'code',application:id})).json();assert.equal(login.status,'ok');
 const reply=new URL(callback);reply.searchParams.set('state',auth.searchParams.get('state'));reply.searchParams.set('code',login.data);reply.searchParams.set('iss',issuer);
 const completed=await browserClient(reply.pathname+reply.search);assert.equal(completed.status,302,'FileBrowser callback rejected the real code/token exchange');
 const profileResponse=await browserClient('/api/users?username=self');assert.equal(profileResponse.status,200,'No FileBrowser session');const profile=await profileResponse.json();assert.equal(profile.username,subject,'FileBrowser account was not linked by stable sub');assert.equal(profile.permissions.admin,false,'OIDC identity gained unexpected administrator rights');
 console.log('PASS: FileBrowser Quantum real OIDC redirect, callback/token verification, session, stable sub linkage, normal-user permissions');
}finally{child.kill();await new Promise(resolve=>{if(child.exitCode!==null)resolve();else{child.once('exit',resolve);setTimeout(resolve,5000);}});}
