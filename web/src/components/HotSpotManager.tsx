import {useState,type FormEvent} from 'react';
import {api} from '../api';
import VoucherStudio from './VoucherStudio';
import POSPanel from './POSPanel';
import type {HotSpotOverview,HotSpotUser} from '../types';
import {formatT,t,type Language} from '../i18n';

type RouterConnection={address:string;port:number;username:string;password:string;useTls:boolean};
type FlashKind='success'|'danger';
type View='all'|'users'|'profiles'|'sessions'|'servers'|'vouchers'|'pos';
type Props={
 language:Language;
 router:RouterConnection;
 overview:HotSpotOverview;
 businessName:string;
 currencyCode:string;
 currencySymbol:string;
 view?:View;
 onRefresh:()=>Promise<void>;
 onMessage:(kind:FlashKind,message:string)=>void;
};

type UserForm={name:string;password:string;profile:string;server:string;limitUptime:string;comment:string};
type ProfileForm={name:string;rateLimit:string;sharedUsers:string;sessionTimeout:string;idleTimeout:string};
const emptyUser:UserForm={name:'',password:'',profile:'default',server:'all',limitUptime:'',comment:''};
const emptyProfile:ProfileForm={name:'',rateLimit:'',sharedUsers:'1',sessionTimeout:'',idleTimeout:''};

export default function HotSpotManager({language,router,overview,businessName,currencyCode,currencySymbol,view='all',onRefresh,onMessage}:Props){
 const tx=(text:string)=>t(language,text);
 const [userForm,setUserForm]=useState<UserForm>(emptyUser);
 const [editingUserId,setEditingUserId]=useState('');
 const [profileForm,setProfileForm]=useState<ProfileForm>(emptyProfile);
 const [editingProfileId,setEditingProfileId]=useState('');
 const [busy,setBusy]=useState(false);

 async function perform(label:string,operation:()=>Promise<unknown>){
  if(busy)return;
  setBusy(true);
  try{
   await operation();
   onMessage('success',language==='ar'?label+' بنجاح.':tx(label)+' successfully.');
   await onRefresh();
  }catch(error){
   onMessage('danger',tx((error as Error).message||'تعذرت العملية.'));
  }finally{setBusy(false)}
 }
 function resetUser(){setUserForm(emptyUser);setEditingUserId('')}
 function resetProfile(){setProfileForm(emptyProfile);setEditingProfileId('')}

 async function submitUser(event:FormEvent){
  event.preventDefault();
  const user={...userForm,name:userForm.name.trim()};
  if(editingUserId){
   await perform('تم تحديث حساب HotSpot',async()=>{
    await api.updateHotSpotUser(router,{...user,id:editingUserId});
    resetUser();
   });
  }else{
   await perform('تم إنشاء حساب HotSpot',async()=>{
    await api.createHotSpotUser(router,user);
    resetUser();
   });
  }
 }
 function editUser(user:HotSpotUser){
  setEditingUserId(user.id);
  setUserForm({name:user.name,password:'',profile:user.profile||'default',server:user.server||'all',limitUptime:user.limitUptime||'',comment:user.comment||''});
  onMessage('success',tx('تم تحميل بيانات الحساب للتعديل. اترك كلمة المرور فارغة للإبقاء عليها دون تغيير.'));
 }
 async function changeUserState(user:HotSpotUser){
  await perform(user.disabled?'تم تفعيل الحساب':'تم تعطيل الحساب',()=>api.setHotSpotUserDisabled(router,user.id,!user.disabled));
 }
 async function removeUser(user:HotSpotUser){
  if(!window.confirm(formatT(language,'هل تريد حذف حساب HotSpot «{name}» من الراوتر؟ لا يمكن التراجع عن هذا الإجراء.',{name:user.name})))return;
  await perform('تم حذف حساب HotSpot',async()=>{
   await api.deleteHotSpotUser(router,user.id);
   if(editingUserId===user.id)resetUser();
  });
 }
 async function submitProfile(event:FormEvent){
  event.preventDefault();
  const profile={...profileForm,name:profileForm.name.trim()};
  if(editingProfileId){
   await perform('تم تحديث الملف التعريفي',async()=>{
    await api.updateHotSpotProfile(router,{...profile,id:editingProfileId});
    resetProfile();
   });
  }else{
   await perform('تم إنشاء الملف التعريفي',async()=>{
    await api.createHotSpotProfile(router,profile);
    resetProfile();
   });
  }
 }
 async function removeProfile(id:string,name:string){
  if(!window.confirm(formatT(language,'هل تريد حذف الملف التعريفي «{name}»؟ قد يرفض الراوتر الحذف إذا كان الملف مستخدمًا.',{name})))return;
  await perform('تم حذف الملف التعريفي',async()=>{
   await api.deleteHotSpotProfile(router,id);
   if(editingProfileId===id)resetProfile();
  });
 }

 return <div className="hotspot-manager">
  <div className="router-result">
   <div className="result-title"><i className="bi bi-wifi"/> {tx("نظرة عامة على HotSpot")}</div>
   <p className="small text-secondary mb-3">{tx("يعرض هذا القسم البيانات ويتيح إدارتها عند تنفيذ عملية صريحة. لا تُعرض كلمات مرور المستخدمين.")}</p>
   <div className="row g-2 hotspot-counts">
    {[['الخوادم',overview.counts.servers],['الملفات التعريفية',overview.counts.profiles],['المستخدمون',overview.counts.users],['الجلسات النشطة',overview.counts.activeSessions]].map(([label,count])=><div className="col-6 col-xl-3" key={label}><div className="hotspot-count"><small>{tx(String(label))}</small><strong>{count}</strong></div></div>)}
   </div>
   {overview.truncated&&<p className="small text-secondary mt-3 mb-0">{formatT(language,"تظهر معاينة لأول {limit} عنصر من كل قائمة؛ الأعداد أعلاه تمثل العدد الكامل على الراوتر.",{limit:overview.previewLimit})}</p>}
  </div>

  {(view==='all'||view==='vouchers')&&<VoucherStudio language={language} router={router} profiles={overview.profiles} servers={overview.servers} businessName={businessName} onRefresh={onRefresh} onMessage={onMessage}/>}
  {(view==='all'||view==='pos')&&<POSPanel language={language} profiles={overview.profiles} servers={overview.servers} currencyCode={currencyCode} currencySymbol={currencySymbol} onMessage={onMessage}/>}

  {(view==='all'||view==='users')&&(<section className="hotspot-management-section">
   <div className="hotspot-section-title"><h3>{tx("مستخدمو HotSpot")}</h3><span>{overview.counts.users} {tx("حساب")}</span></div>
   <form className="hotspot-editor" onSubmit={submitUser}>
    <h4>{editingUserId?tx('تعديل حساب'):tx('إنشاء حساب جديد')}</h4>
    <div className="row g-3">
     <div className="col-md-6"><label className="form-label" htmlFor="hsUserName">{tx("اسم المستخدم")}</label><input id="hsUserName" className="form-control" maxLength={64} value={userForm.name} onChange={e=>setUserForm(s=>({...s,name:e.target.value}))} required/></div>
     <div className="col-md-6"><label className="form-label" htmlFor="hsUserPassword">{tx("كلمة المرور (اختيارية)")}</label><input id="hsUserPassword" className="form-control" type="password" maxLength={128} value={userForm.password} onChange={e=>setUserForm(s=>({...s,password:e.target.value}))} autoComplete="new-password" placeholder={editingUserId?tx('اتركها فارغة للإبقاء على الحالية'):tx('اتركها فارغة للدخول باسم المستخدم فقط')}/><div className="form-hint">{editingUserId?tx('لن تتغير كلمة المرور إذا تركت الحقل فارغًا.'):tx('يمكن إنشاء الحساب باسم المستخدم فقط دون كلمة مرور.')}</div></div>
     <div className="col-md-6"><label className="form-label" htmlFor="hsUserProfile">{tx("الملف التعريفي")}</label><input id="hsUserProfile" className="form-control" list="hotspot-profile-options" value={userForm.profile} onChange={e=>setUserForm(s=>({...s,profile:e.target.value}))} maxLength={64}/><datalist id="hotspot-profile-options">{overview.profiles.map(p=><option key={p.id||p.name} value={p.name}/>)}</datalist></div>
     <div className="col-md-6"><label className="form-label" htmlFor="hsUserServer">{tx("خادم HotSpot")}</label><input id="hsUserServer" className="form-control" value={userForm.server} onChange={e=>setUserForm(s=>({...s,server:e.target.value}))} maxLength={64} placeholder="all"/></div>
     <div className="col-md-6"><label className="form-label" htmlFor="hsUserLimit">{tx("حد الاستخدام الزمني")}</label><input id="hsUserLimit" className="form-control ltr-input" value={userForm.limitUptime} onChange={e=>setUserForm(s=>({...s,limitUptime:e.target.value}))} maxLength={64} placeholder={tx("مثال: 1d00:00:00")}/></div>
     <div className="col-md-6"><label className="form-label" htmlFor="hsUserComment">{tx("ملاحظات")}</label><input id="hsUserComment" className="form-control" value={userForm.comment} onChange={e=>setUserForm(s=>({...s,comment:e.target.value}))} maxLength={255}/></div>
    </div>
    <div className="d-flex flex-wrap gap-2 mt-3"><button className="btn btn-primary" type="submit" disabled={busy}>{busy?tx('جارٍ التنفيذ...'):editingUserId?tx('حفظ التعديلات'):tx('إنشاء الحساب')}</button>{editingUserId&&<button className="btn btn-light" type="button" disabled={busy} onClick={resetUser}>{tx("إلغاء التعديل")}</button>}</div>
   </form>
   <div className="table-responsive mt-3"><table className="table table-sm align-middle hotspot-data-table"><thead><tr><th>{tx("المستخدم")}</th><th>{tx("الملف")}</th><th>{tx("الخادم")}</th><th>{tx("حد الوقت")}</th><th>{tx("الحالة")}</th><th>{tx("إجراءات")}</th></tr></thead><tbody>
    {overview.users.map(user=><tr key={user.id||user.name}><td>{user.name||'—'}</td><td>{user.profile||'default'}</td><td>{user.server||'all'}</td><td>{user.limitUptime||'—'}</td><td><span className={user.disabled?'text-danger':'text-success'}>{tx(user.disabled?'معطّل':'مفعّل')}</span></td><td><div className="d-flex flex-wrap gap-1"><button className="btn btn-sm btn-outline-secondary" disabled={busy||!user.id} onClick={()=>editUser(user)}>{tx("تعديل")}</button><button className="btn btn-sm btn-outline-primary" disabled={busy||!user.id} onClick={()=>void changeUserState(user)}>{tx(user.disabled?'تفعيل':'تعطيل')}</button><button className="btn btn-sm btn-outline-danger" disabled={busy||!user.id} onClick={()=>void removeUser(user)}>{tx("حذف")}</button></div></td></tr>)}
    {!overview.users.length&&<tr><td colSpan={6} className="text-center text-secondary py-3">{tx("لا توجد حسابات مستخدمين في القائمة.")}</td></tr>}
   </tbody></table></div>
  </section>)}

  {(view==='all'||view==='profiles')&&(<section className="hotspot-management-section">
   <div className="hotspot-section-title"><h3>{tx("الملفات التعريفية")}</h3><span>{overview.counts.profiles} {tx("ملف")}</span></div>
   <form className="hotspot-editor" onSubmit={submitProfile}>
    <h4>{editingProfileId?tx('تعديل ملف تعريفي'):tx('إنشاء ملف تعريفي')}</h4>
    <div className="row g-3">
     <div className="col-md-6"><label className="form-label" htmlFor="hsProfileName">{tx("اسم الملف")}</label><input id="hsProfileName" className="form-control" maxLength={64} value={profileForm.name} onChange={e=>setProfileForm(s=>({...s,name:e.target.value}))} required/></div>
     <div className="col-md-6"><label className="form-label" htmlFor="hsProfileRate">{tx("حد السرعة (رفع/تنزيل)")}</label><input id="hsProfileRate" className="form-control ltr-input" maxLength={64} value={profileForm.rateLimit} onChange={e=>setProfileForm(s=>({...s,rateLimit:e.target.value}))} placeholder={tx('مثال: 4M/10M')}/></div>
     <div className="col-md-4"><label className="form-label" htmlFor="hsProfileShared">{tx("عدد الأجهزة المتزامنة")}</label><input id="hsProfileShared" className="form-control ltr-input" type="number" min={1} max={1000000} value={profileForm.sharedUsers} onChange={e=>setProfileForm(s=>({...s,sharedUsers:e.target.value}))} required/></div>
     <div className="col-md-4"><label className="form-label" htmlFor="hsProfileSession">{tx("مدة الجلسة القصوى")}</label><input id="hsProfileSession" className="form-control ltr-input" maxLength={64} value={profileForm.sessionTimeout} onChange={e=>setProfileForm(s=>({...s,sessionTimeout:e.target.value}))} placeholder={tx("اتركه فارغًا بلا حد")}/></div>
     <div className="col-md-4"><label className="form-label" htmlFor="hsProfileIdle">{tx("مدة الخمول")}</label><input id="hsProfileIdle" className="form-control ltr-input" maxLength={64} value={profileForm.idleTimeout} onChange={e=>setProfileForm(s=>({...s,idleTimeout:e.target.value}))} placeholder={tx("اتركه فارغًا بلا حد")}/></div>
    </div>
    <div className="d-flex flex-wrap gap-2 mt-3"><button className="btn btn-primary" type="submit" disabled={busy}>{busy?tx('جارٍ التنفيذ...'):editingProfileId?tx('حفظ التعديلات'):tx('إنشاء الملف')}</button>{editingProfileId&&<button className="btn btn-light" type="button" disabled={busy} onClick={resetProfile}>{tx("إلغاء التعديل")}</button>}</div>
   </form>
   <div className="table-responsive mt-3"><table className="table table-sm align-middle hotspot-data-table"><thead><tr><th>{tx("الاسم")}</th><th>{tx("حد السرعة")}</th><th>{tx("الأجهزة")}</th><th>{tx("مدة الجلسة")}</th><th>{tx("مدة الخمول")}</th><th>{tx("إجراءات")}</th></tr></thead><tbody>
    {overview.profiles.map(profile=><tr key={profile.id||profile.name}><td>{profile.name||'—'}</td><td>{profile.rateLimit||'—'}</td><td>{profile.sharedUsers||'—'}</td><td>{profile.sessionTimeout||'—'}</td><td>{profile.idleTimeout||'—'}</td><td><div className="d-flex flex-wrap gap-1"><button className="btn btn-sm btn-outline-secondary" disabled={busy||!profile.id} onClick={()=>{setEditingProfileId(profile.id);setProfileForm({name:profile.name,rateLimit:profile.rateLimit,sharedUsers:profile.sharedUsers||'1',sessionTimeout:profile.sessionTimeout,idleTimeout:profile.idleTimeout});}}>{tx("تعديل")}</button><button className="btn btn-sm btn-outline-danger" disabled={busy||!profile.id} onClick={()=>void removeProfile(profile.id,profile.name)}>{tx("حذف")}</button></div></td></tr>)}
    {!overview.profiles.length&&<tr><td colSpan={6} className="text-center text-secondary py-3">{tx("لا توجد ملفات تعريفية في القائمة.")}</td></tr>}
   </tbody></table></div>
  </section>)}

  {(view==='all'||view==='sessions')&&(<section className="hotspot-management-section">
   <div className="hotspot-section-title"><h3>{tx("الجلسات النشطة")}</h3><span>{overview.counts.activeSessions} {tx("جلسة")}</span></div>
   <div className="table-responsive"><table className="table table-sm align-middle hotspot-data-table"><thead><tr><th>{tx("المستخدم")}</th><th>{tx("عنوان IP")}</th><th>{tx("عنوان MAC")}</th><th>{tx("مدة الاتصال")}</th><th>{tx("الخادم")}</th></tr></thead><tbody>
    {overview.activeSessions.map(row=><tr key={row.id||row.user+row.address}><td>{row.user||'—'}</td><td dir="ltr">{row.address||'—'}</td><td dir="ltr">{row.macAddress||'—'}</td><td>{row.uptime||'—'}</td><td>{row.server||'—'}</td></tr>)}
    {!overview.activeSessions.length&&<tr><td colSpan={5} className="text-center text-secondary py-3">{tx("لا توجد جلسات نشطة حاليًا.")}</td></tr>}
   </tbody></table></div>
  </section>)}

  {(view==='all'||view==='servers')&&(<section className="hotspot-management-section">
   <div className="hotspot-section-title"><h3>{tx("خوادم HotSpot")}</h3><span>{overview.counts.servers} {tx("خادم")}</span></div>
   <div className="table-responsive"><table className="table table-sm align-middle hotspot-data-table"><thead><tr><th>{tx("الاسم")}</th><th>{tx("الواجهة")}</th><th>{tx("مجمع العناوين")}</th><th>{tx("الملف")}</th><th>{tx("الحالة")}</th></tr></thead><tbody>
    {overview.servers.map(row=><tr key={row.name}><td>{row.name||'—'}</td><td>{row.interface||'—'}</td><td>{row.addressPool||'—'}</td><td>{row.profile||'—'}</td><td><span className={row.disabled?'text-danger':'text-success'}>{tx(row.disabled?'معطّل':'مفعّل')}</span></td></tr>)}
    {!overview.servers.length&&<tr><td colSpan={5} className="text-center text-secondary py-3">{tx("لم يتم إعداد خادم HotSpot على هذا الراوتر.")}</td></tr>}
   </tbody></table></div>
  </section>)}
 </div>;
}
