import {createElement as h,useEffect,useState} from 'react';
import {useAccount} from '@/hooks/use-account';
import * as Setting from '@/lib/setting';
import {OneAccountSecurityScreen} from './OneAccountSecurityScreen';

export function OneAccountSecurity(){
 const {account,reload}=useAccount();const [status,setStatus]=useState<any>();const [pending,setPending]=useState<any>();const [recovery,setRecovery]=useState('');const [error,setError]=useState('');const [message,setMessage]=useState('');const [busy,setBusy]=useState(false);
 const input=(name:string)=>document.querySelector<HTMLInputElement>(`input[name="${name}"]`)?.value||'';
 const clear=()=>document.querySelectorAll<HTMLInputElement>('input').forEach(i=>i.value='');
 async function api(path:string,values:Record<string,string>={}){const body=new FormData();for(const [k,v] of Object.entries({owner:account.owner,name:account.name,mfaType:'app',...values}))body.append(k,v as string);const response=await fetch(Setting.ServerUrl+path,{method:'POST',credentials:'include',body,cache:'no-store'});const result=await response.json();if(result.status!=='ok')throw Error(result.msg||'Security action failed.');return result.data;}
 async function refresh(){setStatus(await api('/api/mfa/setup/initiate',{action:'status'}));}
 useEffect(()=>{if(account)void refresh().catch(e=>setError(e.message));},[account?.id]);
 async function reauthenticate(){const password=input('password');try{const response=await fetch(Setting.ServerUrl+'/api/check-user-password',{method:'POST',credentials:'include',headers:{'Content-Type':'application/json'},body:JSON.stringify({owner:account.owner,name:account.name,password})});const result=await response.json();if(result.status!=='ok')throw Error(result.msg||'Password verification failed.');}finally{const field=document.querySelector<HTMLInputElement>('input[name="password"]');if(field)field.value='';}}
 async function action(name:string){if(busy)return;setError('');setMessage('');setBusy(true);try{
  if(name==='download'){const blob=new Blob(['OneAccount one-time recovery codes\nStore separately from your phone.\n\n'+recovery],{type:'text/plain'});const url=URL.createObjectURL(blob);const link=document.createElement('a');link.href=url;link.download='OneAccount-recovery-codes.txt';link.click();setTimeout(()=>URL.revokeObjectURL(url),1000);return;}
  if(name==='print'){window.print();return;}
  if(name==='acknowledge'){setRecovery('');await refresh();await reload();return;}
  if(name==='cancel'){await api('/api/mfa/setup/initiate',{action:'cancel'});clear();setPending(undefined);await refresh();return;}
  if(name==='begin'){await reauthenticate();setPending(await api('/api/mfa/setup/initiate',{deviceLabel:input('deviceLabel')}));clear();return;}
  if(name==='confirm'){await api('/api/mfa/setup/verify',{passcode:input('code')});const result=await api('/api/mfa/setup/enable');setPending(undefined);clear();setRecovery(result.recoveryCodes.join('\n'));return;}
  if(name==='disable'||name==='regenerate'){
   if(name==='disable'&&input('disableConfirmation')!=='DISABLE')throw Error('Type DISABLE to confirm removal.');
   const proof=input('proof').trim();await reauthenticate();const values=/^\d{6}$/.test(proof)?{passcode:proof}:{recoveryCode:proof};
   if(name==='disable'){await api('/api/delete-mfa',values);clear();window.location.assign('/login');return;}
   else{const result=await api('/api/mfa/setup/initiate',{action:'regenerate',...values});setRecovery(result.recoveryCodes.join('\n'));}
   clear();await refresh();await reload();return;
  }
 }catch(e){setError((e as Error).message);}finally{setBusy(false);}}
 if(!account)return h('p',{},'Sign in to manage your OneAccount security.');
 const device=status?.devices?.[0];const data={error,message:busy?'Verifying securely…':message,enabled:!!status?.enabled,disabled:status&&!status.enabled,pending:!!pending,recovery,showStatus:!!status&&!pending&&!recovery,statusTitle:status?.enabled?'Two-factor authentication is ON':'Protect your OneAccount',metadata:status?.enabled?`${device?.label||'One Authenticator'} · Enrolled ${device?.enrolled?new Date(Number(device.enrolled)*1000).toLocaleDateString():'previously'} · ${status.recoveryCodesRemaining} recovery codes remaining`:'MFA stays off until your first code is verified.',qr:pending?.url,secret:pending?.secret,notifications:status?.securityNotifications?.map((e:any)=>`${new Date(Number(e.created)*1000).toLocaleString()} — ${e.event.replaceAll('_',' ')}`).join('\n')||'No recent security changes.'};
 return h(OneAccountSecurityScreen,{data,onAction:(name:string)=>void action(name)});
}
