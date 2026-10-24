import type {AppSettings,HealthResponse,RouterInfo} from './types';
async function request<T>(url:string,init?:RequestInit):Promise<T>{
 const headers=new Headers(init?.headers);if(!headers.has('Content-Type'))headers.set('Content-Type','application/json');
 const response=await fetch(url,{...init,headers});const payload=await response.json().catch(()=>({}));
 if(!response.ok)throw new Error(payload?.error?.message??`فشل الطلب (${response.status})`);
 return payload as T;
}
export const api={
 health:()=>request<HealthResponse>('/api/health'),
 settings:()=>request<AppSettings>('/api/settings'),
 saveSettings:(s:AppSettings)=>request<AppSettings>('/api/settings',{method:'PUT',body:JSON.stringify(s)}),
 testRouter:(v:{address:string;port:number;username:string;password:string;useTls:boolean})=>request<{status:string;router:RouterInfo}>('/api/router/test',{method:'POST',body:JSON.stringify(v)}),
};
