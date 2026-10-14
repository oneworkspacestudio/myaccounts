import assert from 'node:assert/strict';
import {randomBytes} from 'node:crypto';
import {createServer} from 'node:net';
import {join} from 'node:path';
import {DatabaseSync} from 'node:sqlite';
import {readFile} from 'node:fs/promises';
import {request as httpsRequest} from 'node:https';
const origin=process.env.MYACCOUNTS_TEST_ISSUER,password=process.env.MYACCOUNTS_TEST_PASSWORD,root=process.env.MYACCOUNTS_FIXTURE;
assert(origin==='http://localhost:18787'&&password&&root,'Use only the disposable native smoke installation');
const relayFixture=process.env.MYACCOUNTS_RELAY_FIXTURE?JSON.parse(await readFile(process.env.MYACCOUNTS_RELAY_FIXTURE,'utf8')):null;
const relayCA=relayFixture?await readFile(relayFixture.ca):null;
async function received(){
 if(!relayFixture)return messages;
 return new Promise((resolve,reject)=>{const r=httpsRequest(relayFixture.endpoint+'/fixture/messages',{ca:relayCA,headers:{Authorization:'Bearer '+relayFixture.control}},res=>{let body='';res.on('data',chunk=>body+=chunk);res.on('end',()=>{try{assert.equal(res.statusCode,200);resolve(JSON.parse(body).map(m=>m.Text))}catch(e){reject(e)}})});r.on('error',reject);r.end()});
}
const messages=[],sockets=new Set();
// Test-only loopback mail sink. Production credentials/TLS are never changed.
const smtp=createServer(socket=>{
 sockets.add(socket);socket.on('close',()=>sockets.delete(socket));socket.write('220 localhost fixture SMTP\r\n');let buffer='',data=false,body=[];
 socket.on('data',chunk=>{buffer+=chunk;while(buffer.includes('\r\n')){const i=buffer.indexOf('\r\n'),line=buffer.slice(0,i);buffer=buffer.slice(i+2);
  if(data){if(line==='.') {messages.push(body.join('\r\n'));body=[];data=false;socket.write('250 accepted\r\n');}else body.push(line);continue;}
  const command=line.split(' ')[0].toUpperCase();
  if(command==='EHLO'||command==='HELO')socket.write('250-localhost\r\n250 8BITMIME\r\n');
  else if(command==='DATA'){data=true;socket.write('354 end with dot\r\n');}
  else if(command==='QUIT'){socket.end('221 bye\r\n');}
  else socket.write('250 ok\r\n');
 }});
});
await new Promise((resolve,reject)=>{smtp.once('error',reject);smtp.listen(19997,'127.0.0.1',resolve);});
function client(){const jar=new Map();return async(route,body,form=false)=>{const r=await fetch(origin+route,{method:body?'POST':'GET',headers:{Origin:origin,Cookie:[...jar].map(([k,v])=>`${k}=${v}`).join('; '),'Content-Type':form?'application/x-www-form-urlencoded':'application/json'},body:body?(form?new URLSearchParams(body):JSON.stringify(body)):undefined});for(const value of r.headers.getSetCookie()){const pair=value.split(';')[0],i=pair.indexOf('=');jar.set(pair.slice(0,i),pair.slice(i+1));}assert.equal(r.status,200);return r.json();};}
const admin=client(),suffix=randomBytes(6).toString('hex'),name='mail-'+suffix,email=name+'@example.com',initial=randomBytes(32).toString('base64url'),next=randomBytes(32).toString('base64url');
let db;
try{
 assert.equal((await admin('/api/login',{organization:'built-in',username:'admin@example.com',password,application:'app-built-in',type:'login',signinMethod:'Password'})).status,'ok');
 const provider=relayFixture?(await admin('/api/get-provider?id=admin/myaccounts-email')).data:{owner:'admin',name,displayName:'Isolated mail sink',category:'Email',type:'SMTP',host:'localhost',port:19997,sslMode:'Disable',clientId:'',clientSecret:'',clientId2:'fixture@example.com',clientSecret2:'Fixture',title:'Verification',content:'Your fixture code is %s'};
 if(!relayFixture)assert.equal((await admin('/api/add-provider',provider)).status,'ok');
 else assert.equal(provider.type,'MyAccounts Relay');
 const template=(await admin('/api/get-application?id=admin/myaccounts')).data;
 const app={...template,name,clientId:name,clientSecret:randomBytes(32).toString('hex'),enableSignUp:true,disableSelfSignup:false,codeResendTimeout:1,providers:[{owner:'admin',name:provider.name,canSignUp:true,canSignIn:true,canUnlink:false,rule:'All'}],signupItems:template.signupItems.map(item=>({...item,visible:['Username','Display name','Password','Confirm password','Email'].includes(item.name),required:['Username','Display name','Password','Email'].includes(item.name)})),signinItems:template.signinItems.map(item=>item.name==='Forgot password?'?{...item,visible:true}:item)};
 assert.equal((await admin('/api/add-application',app)).status,'ok');
 const anonymous=client();
 async function send(api,method){const before=(await received()).length;const result=await api('/api/send-verification-code',{dest:email,type:'email',method,captchaType:'none',applicationId:'admin/'+name},true);assert.equal(result.status,'ok','Fixture email send failed');const delivered=await received();assert.equal(delivered.length,before+1);const match=delivered.at(-1).match(/\b(\d{6})\b/);assert(match,'Fixture did not receive code');return match[1];}
 const registrationCode=await send(anonymous,'signup');
 const signup={organization:'myaccounts',application:name,username:name,name:'Mail Test',email,password:initial,emailCode:registrationCode,captchaType:'none'};
 assert.equal((await anonymous('/api/signup',{...signup,emailCode:'invalid'})).status,'error','Invalid signup code accepted');
 assert.equal((await anonymous('/api/signup',signup)).status,'ok','Verified email signup failed');
 const userClient=client();const login=(api,pass)=>api('/api/login',{organization:'myaccounts',username:email,password:pass,application:name,type:'login',signinMethod:'Password'});
 assert.equal((await login(userClient,initial)).status,'ok');const account=(await userClient('/api/get-account')).data;assert.equal(account.emailVerified,true);assert.equal(account.isAdmin,false);
 const recovery=client();const code=await send(recovery,'forget');const verify={organization:'myaccounts',username:email,name,code};
 const other=client();const concurrent=await Promise.all([recovery('/api/verify-code',verify),other('/api/verify-code',verify)]);assert.equal(concurrent.filter(r=>r.status==='ok').length,1,'Email code was not consumed atomically');
 const granted=concurrent[0].status==='ok'?recovery:other;
 assert.equal((await granted('/api/set-password',{userOwner:'myaccounts',userName:'another-account',newPassword:next,code},true)).status,'error','Recovery crossed identity');
 const reset={userOwner:'myaccounts',userName:name,newPassword:next,code};const results=await Promise.all([granted('/api/set-password',reset,true),granted('/api/set-password',reset,true)]);assert.equal(results.filter(r=>r.status==='ok').length,1,'Password-reset permission was reused concurrently');
 const previousSession=await userClient('/api/get-account');assert(!previousSession.data||previousSession.status==='error','Password reset left an old issuer session active');
 assert.equal((await login(client(),initial)).status,'error','Old password survived reset');assert.equal((await login(client(),next)).status,'ok');
 db=new DatabaseSync(join(root,'users.db'));db.exec('PRAGMA busy_timeout=5000');
 await new Promise(r=>setTimeout(r,1100));const expired=client(),expiredCode=await send(expired,'forget');
 db.prepare('UPDATE verification_record SET time=? WHERE receiver=? AND code=?').run(946684800,email,expiredCode);
 assert.equal((await expired('/api/verify-code',{...verify,code:expiredCode})).status,'error','Expired email code accepted');
 await new Promise(r=>setTimeout(r,1100));const delayed=client(),delayedCode=await send(delayed,'forget');assert.equal((await delayed('/api/verify-code',{...verify,code:delayedCode})).status,'ok');
 db.prepare('UPDATE myaccounts_reset_grant SET expires=? WHERE user_id=?').run(946684800,'myaccounts/'+name);
 assert.equal((await delayed('/api/set-password',{...reset,code:delayedCode,newPassword:initial},true)).status,'error','Verified recovery session never expired');
 console.log('PASS: '+(relayFixture?'TLS shared relay':'fixture SMTP')+' verified signup, normal account, atomic concurrent email-code/reset consumption, identity binding, old-password refusal and code/grant expiry');
}finally{db?.close();for(const socket of sockets)socket.destroy();await new Promise(resolve=>smtp.close(resolve));}
