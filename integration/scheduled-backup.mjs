import assert from 'node:assert/strict';
import {randomBytes} from 'node:crypto';
import {readFile,writeFile,unlink,stat} from 'node:fs/promises';
import {join} from 'node:path';
import {spawnSync} from 'node:child_process';
const origin=process.env.MYACCOUNTS_TEST_ISSUER,password=process.env.MYACCOUNTS_TEST_PASSWORD,root=process.env.MYACCOUNTS_FIXTURE,exe=process.env.MYACCOUNTS_EXE;
assert(origin&&password&&root&&exe);
const jar=new Map();async function api(route,body){const r=await fetch(origin+route,{method:body?'POST':'GET',headers:{Origin:origin,Cookie:[...jar].map(([k,v])=>`${k}=${v}`).join('; '),'Content-Type':'application/json'},body:body?JSON.stringify(body):undefined});for(const value of r.headers.getSetCookie()){const item=value.split(';')[0],i=item.indexOf('=');jar.set(item.slice(0,i),item.slice(i+1));}return r.json();}
assert.equal((await api('/api/login',{organization:'built-in',username:'admin@example.com',password,application:'app-built-in',type:'login',signinMethod:'Password'})).status,'ok');
const phrase=randomBytes(48).toString('base64url'),statePath=join(root,'logs','backup-state.json'),phrasePath=join(root,'secrets','test-schedule-passphrase.txt');
// Force a due interval only in this disposable fixture; never change live schedules.
await writeFile(statePath,JSON.stringify({lastSuccess:'2000-01-01T00:00:00Z',file:''}));
assert.equal((await api('/api/myaccounts/schedule',{hours:1,passphrase:phrase})).status,'ok');
try{
 let state;const deadline=Date.now()+100_000;
 while(Date.now()<deadline){state=JSON.parse(await readFile(statePath,'utf8'));if(state.file&&new Date(state.lastSuccess).getTime()>Date.now()-120_000)break;await new Promise(r=>setTimeout(r,1000));}
 assert(state?.file,'Scheduled backup did not fire');assert(/^[a-zA-Z0-9.-]+\.age$/.test(state.file));
 const archive=join(root,'backups',state.file);assert((await stat(archive)).size>0);
 await writeFile(phrasePath,phrase);
 const restored=root+'-scheduled-restored';const result=spawnSync(exe,['-data',restored,'restore',archive,phrasePath],{encoding:'utf8',windowsHide:true});assert.equal(result.status,0,'Scheduled archive failed authenticated native restore');
 assert((await stat(join(restored,'users.db'))).size>0);
 const config=JSON.parse(await readFile(join(restored,'config.json'),'utf8'));assert.equal(config.issuer,origin);
 await assert.rejects(stat(join(restored,'secrets','backup-passphrase.dpapi')),'Machine schedule credential leaked into portable archive');
 console.log('PASS: real scheduled encrypted backup firing, authenticated native restore/database integrity and portable credential exclusion');
}finally{await api('/api/myaccounts/schedule',{hours:0});await unlink(phrasePath).catch(()=>{});}
