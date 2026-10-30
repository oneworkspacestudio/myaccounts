import {readFileSync,writeFileSync} from 'node:fs';
import {parse,compileReact} from '@codedia/parser';
const root=new URL('./src/components/auth/mfa/',import.meta.url);
writeFileSync(new URL('OneAccountSecurityScreen.tsx',root),compileReact(parse(readFileSync(new URL('OneAccountSecurity.aui',root),'utf8'))).replaceAll('@/components/ui','./oneaccount-ui'));
