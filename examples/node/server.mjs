import http from 'node:http';
import {randomBytes} from 'node:crypto';
import * as oidc from 'openid-client';
const issuer=new URL(process.env.OIDC_ISSUER),origin=new URL(process.env.RP_ORIGIN||'http://localhost:18994');
if(origin.protocol!=='https:'&&!['localhost','127.0.0.1'].includes(origin.hostname))throw Error('Use HTTPS for a public relying party');
if(!process.env.OIDC_CLIENT_ID||!process.env.OIDC_CLIENT_SECRET)throw Error('Configure OIDC client credentials');
const insecure=issuer.protocol==='http:'&&issuer.hostname==='localhost';
const config=await oidc.discovery(issuer,process.env.OIDC_CLIENT_ID,process.env.OIDC_CLIENT_SECRET,undefined,insecure?{execute:[oidc.allowInsecureRequests]}:undefined);
oidc.enableNonRepudiationChecks(config);
const sessions=new Map(),cookieName='example_session';
function cookie(res,id){res.setHeader('Set-Cookie',`${cookieName}=${id}; Path=/; HttpOnly; SameSite=Lax; Max-Age=1800${origin.protocol==='https:'?'; Secure':''}`)}
function fresh(res){if(sessions.size>=1000)sessions.delete(sessions.keys().next().value);const id=randomBytes(32).toString('base64url'),state={expires:Date.now()+1800000};sessions.set(id,state);cookie(res,id);return [id,state];}
function escape(value){return String(value).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));}
http.createServer(async(req,res)=>{
 try{
  if(req.headers.host!==origin.host||!req.url.startsWith('/')){res.writeHead(400).end();return}
  const url=new URL(req.url,origin);let id=(req.headers.cookie||'').split('; ').find(c=>c.startsWith(cookieName+'='))?.slice(cookieName.length+1),state=sessions.get(id);
  if(!state||state.expires<Date.now()){if(id)sessions.delete(id);[id,state]=fresh(res)}
  res.setHeader('Cache-Control','no-store');res.setHeader('X-Content-Type-Options','nosniff');
  if(url.pathname==='/login'){
   const verifier=oidc.randomPKCECodeVerifier();state.oauth={verifier,state:oidc.randomState(),nonce:oidc.randomNonce(),expires:Date.now()+300000};
   const target=oidc.buildAuthorizationUrl(config,{redirect_uri:new URL('/callback',origin).href,scope:'openid profile email',state:state.oauth.state,nonce:state.oauth.nonce,code_challenge:await oidc.calculatePKCECodeChallenge(verifier),code_challenge_method:'S256'});res.writeHead(302,{Location:target.href}).end();return
  }
  if(url.pathname==='/callback'){
   const flow=state.oauth;delete state.oauth;if(!flow||flow.expires<Date.now())throw Error('Expired authorization request');
   const tokens=await oidc.authorizationCodeGrant(config,url,{pkceCodeVerifier:flow.verifier,expectedState:flow.state,expectedNonce:flow.nonce});const claims=tokens.claims();
   const user=await oidc.fetchUserInfo(config,tokens.access_token,claims.sub);sessions.delete(id);[id,state]=fresh(res);state.user={sub:user.sub,name:user.name,email:user.email};res.writeHead(303,{Location:'/'}).end();return
  }
  if(url.pathname==='/api/me'){if(!state.user){res.writeHead(401).end();return}res.setHeader('Content-Type','application/json');res.end(JSON.stringify(state.user));return}
  if(url.pathname==='/logout'&&req.method==='POST'){if(req.headers.origin!==origin.origin){res.writeHead(403).end();return}sessions.delete(id);cookie(res,'');res.writeHead(303,{Location:'/'}).end();return}
  if(url.pathname!=='/'){res.writeHead(404).end();return}
  res.setHeader('Content-Type','text/html; charset=utf-8');res.end(`<h1>OIDC example</h1>${state.user?`<p>Signed in as ${escape(state.user.sub)}</p><form method="post" action="/logout"><button>Sign out</button></form>`:'<a href="/login">Sign in with MyAccounts</a>'}`);
 }catch{res.writeHead(400,{'Content-Type':'text/plain'}).end('Authorization failed. Start a new sign-in.');}
}).listen(Number(origin.port||18994),'127.0.0.1',()=>console.log('Example relying party listening on loopback'));
