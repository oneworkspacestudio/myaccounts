// Capture documentation using a disposable identity and synthetic accounts/files.
import assert from 'node:assert/strict';
import {randomBytes} from 'node:crypto';
import {createServer} from 'node:http';
import {spawn} from 'node:child_process';
import {mkdir,readFile,writeFile} from 'node:fs/promises';
import {join,resolve} from 'node:path';
import {chromium} from '@playwright/test';
const origin=process.env.MYACCOUNTS_TEST_ISSUER,password=process.env.MYACCOUNTS_TEST_PASSWORD,fixture=process.env.MYACCOUNTS_FIXTURE;
assert(origin?.startsWith('http://localhost:')&&password&&fixture,'Run through the disposable smoke harness');
const root=resolve(import.meta.dirname,'..'),output=join(root,'docs/screenshots');await mkdir(output,{recursive:true});
const context=await chromium.launchPersistentContext(join(fixture,'docs-browser'),{channel:'chrome',headless:true,viewport:{width:1365,height:980}});
const anon=await chromium.launch({channel:'chrome',headless:true});let child,setup;
const fb='http://localhost:19998';
async function api(route,body){const r=await context.request.fetch(origin+route,{method:body?'POST':'GET',headers:{Origin:origin},data:body});const data=await r.json();assert.equal(data.status,'ok',data.msg||route);return data.data;}
async function capture(page,name){await page.evaluate(()=>document.fonts.ready);await page.screenshot({path:join(output,name+'.png'),fullPage:true});}
try{
 await api('/api/login',{organization:'built-in',username:'admin@example.com',password,application:'app-built-in',type:'login',signinMethod:'Password'});
 const admin=await api('/api/get-account');const originalDisplayName=admin.displayName;admin.displayName='Demo Administrator';await api('/api/update-user?id=built-in/admin&columns=displayName',admin);
 const memberPassword=randomBytes(32).toString('base64url');
 for(const [name,displayName] of [['alex','Alex Example'],['sam','Sam Example']])await api('/api/add-user',{owner:'myaccounts',name,displayName,email:name+'@example.com',password:memberPassword,passwordType:'plain',isAdmin:false,type:'normal-user',signupApplication:'myaccounts',createdTime:new Date().toISOString(),properties:{}});
 const page=context.pages()[0];await page.goto(origin+'/operations');await page.getByRole('link',{name:'Manage users',exact:true}).waitFor();await page.getByText('Email is not connected.',{exact:false}).waitFor();await capture(page,'administration');
 await page.getByRole('link',{name:'Manage users',exact:true}).click();await page.locator('a[href="/users/myaccounts/alex"]').waitFor();await capture(page,'users');
 const publicPage=await anon.newPage({viewport:{width:1200,height:850}});await publicPage.goto(origin+'/login/myaccounts');await publicPage.locator('#username').waitFor();await capture(publicPage,'sign-in');
 const wizard=await readFile(join(root,'internal/portable/setup.html'));setup=createServer((req,res)=>{if(req.url==='/setup-info'){res.setHeader('Content-Type','application/json');res.end(JSON.stringify({dataPath:'F:/MyAccounts/data',sharedEmail:true}));return}res.setHeader('Content-Type','text/html; charset=utf-8');res.end(wizard)});await new Promise(r=>setup.listen(0,'127.0.0.1',r));
 const setupPage=await anon.newPage({viewport:{width:1365,height:980}});await setupPage.goto(`http://127.0.0.1:${setup.address().port}/#documentation-fixture`);await setupPage.locator('#prerequisites').filter({hasText:'Your account data'}).waitFor().catch(()=>{});
 await capture(setupPage,'setup-desktop');await setupPage.setViewportSize({width:390,height:844});await capture(setupPage,'setup-mobile');await setupPage.setViewportSize({width:1365,height:980});await setupPage.locator('#name').fill('Demo Administrator');await setupPage.locator('#email').fill('owner@example.com');await setupPage.locator('#password').fill(memberPassword);await setupPage.locator('#confirm').fill(memberPassword);await setupPage.locator('#next').click();await setupPage.locator('#next').click();assert.equal(await setupPage.locator('#emailMode').inputValue(),'none');await capture(setupPage,'optional-email');
 if(process.env.MYACCOUNTS_FILEBROWSER_EXE){
  const secret=randomBytes(32).toString('base64url'),clientId='filebrowser-docs';const template=await api('/api/get-application?id=admin/myaccounts');await api('/api/add-application',{...template,name:clientId,displayName:'FileBrowser Drive',clientId,clientSecret:secret,redirectUris:[fb+'/api/auth/oidc/callback'],grantTypes:['authorization_code'],enableAutoSignin:false,enableSignUp:false,providers:[],scopes:[]});
  const runtime=join(fixture,'filebrowser-docs'),source=join(runtime,'Demo files');await mkdir(join(source,'Documents'),{recursive:true});await mkdir(join(source,'Photos'),{recursive:true});await writeFile(join(source,'Welcome.txt'),'Sample files for the MyAccounts and FileBrowser documentation.\n');
  const config={server:{database:{path:join(runtime,'settings.sqlite')},sources:[{path:source,name:'Demo files',config:{defaultEnabled:true}}],disableUpdateCheck:true,disablePreviews:true,cacheDir:join(runtime,'cache')},http:{listen:'127.0.0.1',port:19998,baseURL:'/'},frontend:{oidcLoginButtonText:'Sign in with MyAccounts'},auth:{methods:{noauth:false,password:{enabled:true,signup:false,adminUsername:'demo-recovery-admin'},oidc:{enabled:true,clientId,issuerUrl:origin,scopes:'openid profile email',userIdentifier:'sub',disableVerifyTLS:false}}},userDefaults:{account:{permissions:{admin:false,api:false}}}};
  const file=join(runtime,'config.yaml');await writeFile(file,JSON.stringify(config,null,2));child=spawn(process.env.MYACCOUNTS_FILEBROWSER_EXE,['-c',file],{cwd:runtime,windowsHide:true,env:{...process.env,FILEBROWSER_OIDC_CLIENT_SECRET:secret,FILEBROWSER_ADMIN_PASSWORD:randomBytes(32).toString('hex')},stdio:'ignore'});
  let ready=false;for(let n=0;n<120;n++){assert(child.exitCode===null,'Documentation FileBrowser stopped');try{if((await fetch(fb)).ok){ready=true;break}}catch{}await new Promise(r=>setTimeout(r,250));}assert(ready);
  const browserPage=await anon.newPage({viewport:{width:1365,height:900}});await browserPage.goto(fb+'/login');await browserPage.getByRole('link',{name:'Sign in with MyAccounts'}).waitFor();await capture(browserPage,'filebrowser-login');await browserPage.getByRole('link',{name:'Sign in with MyAccounts'}).click();await browserPage.locator('#username').fill('alex@example.com');if(!await browserPage.locator('#password').isVisible())await browserPage.getByRole('button',{name:'Next',exact:true}).click();await browserPage.locator('#password').fill(memberPassword);await browserPage.getByRole('button',{name:'Sign In',exact:true}).click();await browserPage.waitForURL(u=>u.origin===fb,{timeout:60000});assert.equal((await (await browserPage.request.get(fb+'/api/users?username=self')).json()).permissions.admin,false);await browserPage.getByText('Welcome.txt',{exact:true}).first().waitFor();await capture(browserPage,'filebrowser-files');
 }
 admin.displayName=originalDisplayName;await api('/api/update-user?id=built-in/admin&columns=displayName',admin);
 console.log('PASS: documentation screenshots with synthetic accounts and real FileBrowser browser SSO; no live account or document data');
}finally{await context.close();await anon.close();if(setup)await new Promise(r=>setup.close(r));if(child){child.kill();await new Promise(r=>{if(child.exitCode!==null)r();else{child.once('exit',r);setTimeout(r,5000)}})}}
