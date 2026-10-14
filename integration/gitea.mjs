import assert from 'node:assert/strict';
import {randomBytes,createHash} from 'node:crypto';
import {spawn,execFile} from 'node:child_process';
import {promisify} from 'node:util';
import {mkdir,writeFile,readFile} from 'node:fs/promises';
import path from 'node:path';
const rawRun=promisify(execFile);async function run(...args){try{return await rawRun(...args)}catch{throw Error('Isolated Gitea setup command failed; inspect its isolated log (credentials redacted)')}}const issuer=process.env.MYACCOUNTS_TEST_ISSUER;const password=process.env.MYACCOUNTS_TEST_PASSWORD;const exe=process.env.MYACCOUNTS_GITEA_EXE;const runtime=process.env.MYACCOUNTS_GITEA_DATA;
assert(issuer?.startsWith('http://localhost:'));assert(password);assert(exe);assert(runtime);
const userSSH=path.join(process.env.USERPROFILE,'.ssh','authorized_keys');async function keyFingerprint(){try{return createHash('sha256').update(await readFile(userSSH)).digest('hex')}catch(e){if(e.code==='ENOENT')return 'absent';throw e}}const originalSSH=await keyFingerprint();
const origin='http://localhost:19997';const callback=origin+'/user/oauth2/myaccounts/callback';
function browser(base){const jar=new Map();return async(relative,body)=>{const r=await fetch(base+relative,{redirect:'manual',method:body?'POST':'GET',headers:{Origin:base,Cookie:[...jar].map(([k,v])=>`${k}=${v}`).join('; '),'Content-Type':'application/json'},body:body?JSON.stringify(body):undefined});for(const value of r.headers.getSetCookie()){const p=value.split(';')[0];const i=p.indexOf('=');const k=p.slice(0,i);if(/max-age=(?:0|-1)(?:;|$)/i.test(value))jar.delete(k);else jar.set(k,p.slice(i+1));}return r;};}
const provider=browser(issuer);const primary={organization:'built-in',username:'admin@example.com',password,application:'app-built-in',type:'login',signinMethod:'Password'};assert.equal((await (await provider('/api/login',primary)).json()).status,'ok');
const account=await (await provider('/api/get-account')).json();const subject=account.data.id;assert(subject);const template=await (await provider('/api/get-application?id=admin/app-built-in')).json();assert.equal(template.status,'ok');
const id='gitea-'+randomBytes(8).toString('hex');const secret=randomBytes(32).toString('hex');const app={...template.data,name:id,displayName:'Gitea test',clientId:id,clientSecret:secret,redirectUris:[callback],grantTypes:['authorization_code'],enableAutoSignin:false,enableSigninSession:true,enableSignUp:false,providers:[],scopes:[]};assert.equal((await (await provider('/api/add-application',app)).json()).status,'ok');
await mkdir(runtime,{recursive:true});const configPath=path.join(runtime,'app.ini');const disk=p=>p.replaceAll('\\','/');
const config=`APP_NAME = Isolated MyAccounts Gitea test
RUN_MODE = prod
[server]
HTTP_ADDR = 127.0.0.1
HTTP_PORT = 19997
DOMAIN = localhost
ROOT_URL = ${origin}/
DISABLE_SSH = true
SSH_ROOT_PATH = ${disk(path.join(runtime,'ssh'))}
SSH_CREATE_AUTHORIZED_KEYS_FILE = false
SSH_CREATE_AUTHORIZED_PRINCIPALS_FILE = false
APP_DATA_PATH = ${disk(path.join(runtime,'data'))}
[database]
DB_TYPE = sqlite3
PATH = ${disk(path.join(runtime,'gitea.db'))}
[security]
INSTALL_LOCK = true
SECRET_KEY = ${randomBytes(32).toString('hex')}
[repository]
ROOT = ${disk(path.join(runtime,'repos'))}
[service]
DISABLE_REGISTRATION = false
REQUIRE_SIGNIN_VIEW = true
REGISTER_EMAIL_CONFIRM = false
ENABLE_NOTIFY_MAIL = false
[oauth2_client]
ENABLE_AUTO_REGISTRATION = true
USERNAME = userid
ACCOUNT_LINKING = disabled
UPDATE_AVATAR = false
[mailer]
ENABLED = false
[log]
MODE = file
ROOT_PATH = ${disk(path.join(runtime,'logs'))}
LEVEL = Warn
`;
await writeFile(configPath,config);const flags=['--work-path',runtime,'--config',configPath];const env={...process.env,HOME:runtime,USERPROFILE:runtime,GIT_CONFIG_GLOBAL:path.join(runtime,'gitconfig'),GIT_CONFIG_SYSTEM:path.join(runtime,'git-system-config')};const options={cwd:runtime,windowsHide:true,env};
await run(exe,[...flags,'migrate'],options);
await run(exe,[...flags,'admin','user','create','--username','bootstrap-admin','--password',randomBytes(32).toString('hex'),'--email','bootstrap@example.com','--admin','--must-change-password=false'],options);
await run(exe,[...flags,'admin','auth','add-oauth','--name','myaccounts','--provider','openidConnect','--key',id,'--secret',secret,'--auto-discover-url',issuer+'/.well-known/openid-configuration','--scopes','openid','--scopes','profile','--scopes','email'],options);
const child=spawn(exe,[...flags,'web'],{...options,stdio:'ignore'});
try{
 let ready=false;for(let i=0;i<120;i++){if(child.exitCode!==null)throw Error('Isolated Gitea exited; inspect its isolated log');try{const r=await fetch(origin+'/user/login');if(r.status===200){ready=true;break}}catch{};await new Promise(r=>setTimeout(r,500));}assert(ready,'Gitea startup timeout');
 const client=browser(origin);let start=await client('/user/oauth2/myaccounts');let auth=new URL(start.headers.get('location'),origin);for(let n=0;n<3&&auth.origin===origin;n++){assert([302,303,307].includes(start.status));start=await client(auth.pathname+auth.search);auth=new URL(start.headers.get('location'),origin);}assert([302,303,307].includes(start.status));assert.equal(auth.origin,new URL(issuer).origin);assert.equal(auth.searchParams.get('redirect_uri'),callback);
 const query=new URLSearchParams({clientId:id,responseType:'code',redirectUri:callback,scope:auth.searchParams.get('scope'),state:auth.searchParams.get('state')});for(const name of ['nonce','code_challenge','code_challenge_method'])if(auth.searchParams.has(name))query.set(name,auth.searchParams.get(name));
 const login=await (await provider('/api/login?'+query,{...primary,type:'code',application:id})).json();assert.equal(login.status,'ok');const reply=new URL(callback);reply.searchParams.set('state',auth.searchParams.get('state'));reply.searchParams.set('code',login.data);reply.searchParams.set('iss',issuer);
 const completed=await client(reply.pathname+reply.search);assert.equal(completed.status,303,'Gitea callback did not complete automatic registration');const destination=new URL(completed.headers.get('location'),origin);assert.equal(destination.origin,origin);await client(destination.pathname+destination.search);// Gitea 28.1.0 API authentication deliberately excludes browser sessions.
 const settings=await client('/user/settings');assert.equal(settings.status,200,'Gitea browser session was not established');const html=await settings.text();assert(html.includes(subject),'Gitea settings did not identify the stable subject');
 const admin=await client('/admin');assert([403,404].includes(admin.status),'SSO user gained administrator access');
 const listing=await run(exe,[...flags,'admin','user','list'],options);const row=listing.stdout.split('\n').find(line=>line.includes(subject));assert(row,'Gitea did not persist the registered subject');assert(/false/i.test(row),'Registered SSO user should be a normal user');
 console.log('PASS: Gitea real OIDC redirect, callback, automatic registration, session, stable subject, normal-user permissions');
}finally{child.kill();await new Promise(resolve=>{if(child.exitCode!==null)resolve();else{child.once('exit',resolve);setTimeout(resolve,5000);}});assert.equal(await keyFingerprint(),originalSSH,'Gitea isolation altered the user SSH file');}
