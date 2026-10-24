import type {AppSettings,HealthResponse,HotSpotOverview,RouterInfo,VoucherBatch,VoucherBatchResult} from './types';
async function request<T>(url:string,init?:RequestInit):Promise<T>{
 const headers=new Headers(init?.headers);if(!headers.has('Content-Type'))headers.set('Content-Type','application/json');
 let response:Response;
 try {
  response=await fetch(url,{...init,headers});
 } catch(error) {
  if(error instanceof DOMException && error.name==='TimeoutError') {
   throw new Error('انتهت مهلة الطلب. تحقق من اتصال الراوتر وتفعيل خدمة RouterOS API ثم أعد المحاولة.');
  }
  throw new Error('تعذر الوصول إلى الخادم المحلي. تأكد من أن bt-manager يعمل ثم أعد المحاولة.');
 }
 const payload=await response.json().catch(()=>({}));
 if(!response.ok)throw new Error(payload?.error?.message??`فشل الطلب (${response.status})`);
 return payload as T;
}
export const api={
 health:()=>request<HealthResponse>('/api/health'),
 settings:()=>request<AppSettings>('/api/settings'),
 saveSettings:(s:AppSettings)=>request<AppSettings>('/api/settings',{method:'PUT',body:JSON.stringify(s)}),
 testRouter:(v:{address:string;port:number;username:string;password:string;useTls:boolean})=>request<{status:string;router:RouterInfo}>('/api/router/test',{method:'POST',body:JSON.stringify(v),signal:AbortSignal.timeout(15000)}),
 hotspotOverview:(v:{address:string;port:number;username:string;password:string;useTls:boolean})=>request<HotSpotOverview>('/api/router/hotspot/overview',{method:'POST',body:JSON.stringify(v),signal:AbortSignal.timeout(30000)}),
 createHotSpotUser:(router:{address:string;port:number;username:string;password:string;useTls:boolean},user:{name:string;password:string;profile:string;server:string;limitUptime:string;comment:string})=>request<{status:string}>('/api/router/hotspot/users/create',{method:'POST',body:JSON.stringify({router,user}),signal:AbortSignal.timeout(30000)}),
 updateHotSpotUser:(router:{address:string;port:number;username:string;password:string;useTls:boolean},user:{id:string;name:string;password:string;profile:string;server:string;limitUptime:string;comment:string})=>request<{status:string}>('/api/router/hotspot/users/update',{method:'POST',body:JSON.stringify({router,user}),signal:AbortSignal.timeout(30000)}),
 setHotSpotUserDisabled:(router:{address:string;port:number;username:string;password:string;useTls:boolean},id:string,disabled:boolean)=>request<{status:string}>('/api/router/hotspot/users/state',{method:'POST',body:JSON.stringify({router,id,disabled}),signal:AbortSignal.timeout(30000)}),
 deleteHotSpotUser:(router:{address:string;port:number;username:string;password:string;useTls:boolean},id:string)=>request<{status:string}>('/api/router/hotspot/users/delete',{method:'POST',body:JSON.stringify({router,id}),signal:AbortSignal.timeout(30000)}),
 createHotSpotProfile:(router:{address:string;port:number;username:string;password:string;useTls:boolean},profile:{name:string;rateLimit:string;sharedUsers:string;sessionTimeout:string;idleTimeout:string})=>request<{status:string}>('/api/router/hotspot/profiles/create',{method:'POST',body:JSON.stringify({router,profile}),signal:AbortSignal.timeout(30000)}),
 updateHotSpotProfile:(router:{address:string;port:number;username:string;password:string;useTls:boolean},profile:{id:string;name:string;rateLimit:string;sharedUsers:string;sessionTimeout:string;idleTimeout:string})=>request<{status:string}>('/api/router/hotspot/profiles/update',{method:'POST',body:JSON.stringify({router,profile}),signal:AbortSignal.timeout(30000)}),
 deleteHotSpotProfile:(router:{address:string;port:number;username:string;password:string;useTls:boolean},id:string)=>request<{status:string}>('/api/router/hotspot/profiles/delete',{method:'POST',body:JSON.stringify({router,id}),signal:AbortSignal.timeout(30000)}),
 generateVouchers:(payload:{router:{address:string;port:number;username:string;password:string;useTls:boolean};prefix:string;count:number;profile:string;server:string;limitUptime:string})=>request<VoucherBatchResult>('/api/router/hotspot/vouchers/generate',{method:'POST',body:JSON.stringify(payload),signal:AbortSignal.timeout(120000)}),
 voucherBatches:()=>request<{batches:VoucherBatch[]}>('/api/vouchers/batches'),
 voucherBatch:(id:number)=>request<VoucherBatchResult>('/api/vouchers/batches/'+id),
};
