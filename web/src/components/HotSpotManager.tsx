import {useState,type FormEvent} from 'react';
import {api} from '../api';
import VoucherStudio from './VoucherStudio';
import type {HotSpotOverview,HotSpotUser} from '../types';

type RouterConnection={address:string;port:number;username:string;password:string;useTls:boolean};
type FlashKind='success'|'danger';
type Props={
 router:RouterConnection;
 overview:HotSpotOverview;
 businessName:string;
 onRefresh:()=>Promise<void>;
 onMessage:(kind:FlashKind,message:string)=>void;
};

type UserForm={name:string;password:string;profile:string;server:string;limitUptime:string;comment:string};
type ProfileForm={name:string;rateLimit:string;sharedUsers:string;sessionTimeout:string;idleTimeout:string};
const emptyUser:UserForm={name:'',password:'',profile:'default',server:'all',limitUptime:'',comment:''};
const emptyProfile:ProfileForm={name:'',rateLimit:'',sharedUsers:'1',sessionTimeout:'',idleTimeout:''};

export default function HotSpotManager({router,overview,businessName,onRefresh,onMessage}:Props){
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
   onMessage('success',label+' بنجاح.');
   await onRefresh();
  }catch(error){
   onMessage('danger',(error as Error).message||'تعذرت العملية.');
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
  onMessage('success','تم تحميل بيانات الحساب للتعديل. اترك كلمة المرور فارغة للإبقاء عليها دون تغيير.');
 }
 async function changeUserState(user:HotSpotUser){
  await perform(user.disabled?'تم تفعيل الحساب':'تم تعطيل الحساب',()=>api.setHotSpotUserDisabled(router,user.id,!user.disabled));
 }
 async function removeUser(user:HotSpotUser){
  if(!window.confirm('هل تريد حذف حساب HotSpot «'+user.name+'» من الراوتر؟ لا يمكن التراجع عن هذا الإجراء.'))return;
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
  if(!window.confirm('هل تريد حذف الملف التعريفي «'+name+'»؟ قد يرفض الراوتر الحذف إذا كان الملف مستخدمًا.'))return;
  await perform('تم حذف الملف التعريفي',async()=>{
   await api.deleteHotSpotProfile(router,id);
   if(editingProfileId===id)resetProfile();
  });
 }

 return <div className="hotspot-manager">
  <div className="router-result">
   <div className="result-title"><i className="bi bi-wifi"/> نظرة عامة على HotSpot</div>
   <p className="small text-secondary mb-3">يعرض هذا القسم البيانات ويتيح إدارتها عند تنفيذ عملية صريحة. لا تُعرض كلمات مرور المستخدمين.</p>
   <div className="row g-2 hotspot-counts">
    {[['الخوادم',overview.counts.servers],['الملفات التعريفية',overview.counts.profiles],['المستخدمون',overview.counts.users],['الجلسات النشطة',overview.counts.activeSessions]].map(([label,count])=><div className="col-6 col-xl-3" key={label}><div className="hotspot-count"><small>{label}</small><strong>{count}</strong></div></div>)}
   </div>
   {overview.truncated&&<p className="small text-secondary mt-3 mb-0">تظهر معاينة لأول {overview.previewLimit} عنصر من كل قائمة؛ الأعداد أعلاه تمثل العدد الكامل على الراوتر.</p>}
  </div>

  <VoucherStudio router={router} profiles={overview.profiles} servers={overview.servers} businessName={businessName} onRefresh={onRefresh} onMessage={onMessage}/>

  <section className="hotspot-management-section">
   <div className="hotspot-section-title"><h3>مستخدمو HotSpot</h3><span>{overview.counts.users} حساب</span></div>
   <form className="hotspot-editor" onSubmit={submitUser}>
    <h4>{editingUserId?'تعديل حساب':'إنشاء حساب جديد'}</h4>
    <div className="row g-3">
     <div className="col-md-6"><label className="form-label" htmlFor="hsUserName">اسم المستخدم</label><input id="hsUserName" className="form-control" maxLength={64} value={userForm.name} onChange={e=>setUserForm(s=>({...s,name:e.target.value}))} required/></div>
     <div className="col-md-6"><label className="form-label" htmlFor="hsUserPassword">كلمة المرور (اختيارية)</label><input id="hsUserPassword" className="form-control" type="password" maxLength={128} value={userForm.password} onChange={e=>setUserForm(s=>({...s,password:e.target.value}))} autoComplete="new-password" placeholder={editingUserId?'اتركها فارغة للإبقاء على الحالية':'اتركها فارغة للدخول باسم المستخدم فقط'}/><div className="form-hint">{editingUserId?'لن تتغير كلمة المرور إذا تركت الحقل فارغًا.':'يمكن إنشاء الحساب باسم المستخدم فقط دون كلمة مرور.'}</div></div>
     <div className="col-md-6"><label className="form-label" htmlFor="hsUserProfile">الملف التعريفي</label><input id="hsUserProfile" className="form-control" list="hotspot-profile-options" value={userForm.profile} onChange={e=>setUserForm(s=>({...s,profile:e.target.value}))} maxLength={64}/><datalist id="hotspot-profile-options">{overview.profiles.map(p=><option key={p.id||p.name} value={p.name}/>)}</datalist></div>
     <div className="col-md-6"><label className="form-label" htmlFor="hsUserServer">خادم HotSpot</label><input id="hsUserServer" className="form-control" value={userForm.server} onChange={e=>setUserForm(s=>({...s,server:e.target.value}))} maxLength={64} placeholder="all"/></div>
     <div className="col-md-6"><label className="form-label" htmlFor="hsUserLimit">حد الاستخدام الزمني</label><input id="hsUserLimit" className="form-control ltr-input" value={userForm.limitUptime} onChange={e=>setUserForm(s=>({...s,limitUptime:e.target.value}))} maxLength={64} placeholder="مثال: 1d00:00:00"/></div>
     <div className="col-md-6"><label className="form-label" htmlFor="hsUserComment">ملاحظات</label><input id="hsUserComment" className="form-control" value={userForm.comment} onChange={e=>setUserForm(s=>({...s,comment:e.target.value}))} maxLength={255}/></div>
    </div>
    <div className="d-flex flex-wrap gap-2 mt-3"><button className="btn btn-primary" type="submit" disabled={busy}>{busy?'جارٍ التنفيذ...':editingUserId?'حفظ التعديلات':'إنشاء الحساب'}</button>{editingUserId&&<button className="btn btn-light" type="button" disabled={busy} onClick={resetUser}>إلغاء التعديل</button>}</div>
   </form>
   <div className="table-responsive mt-3"><table className="table table-sm align-middle hotspot-data-table"><thead><tr><th>المستخدم</th><th>الملف</th><th>الخادم</th><th>حد الوقت</th><th>الحالة</th><th>إجراءات</th></tr></thead><tbody>
    {overview.users.map(user=><tr key={user.id||user.name}><td>{user.name||'—'}</td><td>{user.profile||'default'}</td><td>{user.server||'all'}</td><td>{user.limitUptime||'—'}</td><td><span className={user.disabled?'text-danger':'text-success'}>{user.disabled?'معطّل':'مفعّل'}</span></td><td><div className="d-flex flex-wrap gap-1"><button className="btn btn-sm btn-outline-secondary" disabled={busy||!user.id} onClick={()=>editUser(user)}>تعديل</button><button className="btn btn-sm btn-outline-primary" disabled={busy||!user.id} onClick={()=>void changeUserState(user)}>{user.disabled?'تفعيل':'تعطيل'}</button><button className="btn btn-sm btn-outline-danger" disabled={busy||!user.id} onClick={()=>void removeUser(user)}>حذف</button></div></td></tr>)}
    {!overview.users.length&&<tr><td colSpan={6} className="text-center text-secondary py-3">لا توجد حسابات مستخدمين في القائمة.</td></tr>}
   </tbody></table></div>
  </section>

  <section className="hotspot-management-section">
   <div className="hotspot-section-title"><h3>الملفات التعريفية</h3><span>{overview.counts.profiles} ملف</span></div>
   <form className="hotspot-editor" onSubmit={submitProfile}>
    <h4>{editingProfileId?'تعديل ملف تعريفي':'إنشاء ملف تعريفي'}</h4>
    <div className="row g-3">
     <div className="col-md-6"><label className="form-label" htmlFor="hsProfileName">اسم الملف</label><input id="hsProfileName" className="form-control" maxLength={64} value={profileForm.name} onChange={e=>setProfileForm(s=>({...s,name:e.target.value}))} required/></div>
     <div className="col-md-6"><label className="form-label" htmlFor="hsProfileRate">حد السرعة (رفع/تنزيل)</label><input id="hsProfileRate" className="form-control ltr-input" maxLength={64} value={profileForm.rateLimit} onChange={e=>setProfileForm(s=>({...s,rateLimit:e.target.value}))} placeholder="مثال: 4M/10M"/></div>
     <div className="col-md-4"><label className="form-label" htmlFor="hsProfileShared">عدد الأجهزة المتزامنة</label><input id="hsProfileShared" className="form-control ltr-input" type="number" min={1} max={1000000} value={profileForm.sharedUsers} onChange={e=>setProfileForm(s=>({...s,sharedUsers:e.target.value}))} required/></div>
     <div className="col-md-4"><label className="form-label" htmlFor="hsProfileSession">مدة الجلسة القصوى</label><input id="hsProfileSession" className="form-control ltr-input" maxLength={64} value={profileForm.sessionTimeout} onChange={e=>setProfileForm(s=>({...s,sessionTimeout:e.target.value}))} placeholder="اتركه فارغًا بلا حد"/></div>
     <div className="col-md-4"><label className="form-label" htmlFor="hsProfileIdle">مدة الخمول</label><input id="hsProfileIdle" className="form-control ltr-input" maxLength={64} value={profileForm.idleTimeout} onChange={e=>setProfileForm(s=>({...s,idleTimeout:e.target.value}))} placeholder="اتركه فارغًا بلا حد"/></div>
    </div>
    <div className="d-flex flex-wrap gap-2 mt-3"><button className="btn btn-primary" type="submit" disabled={busy}>{busy?'جارٍ التنفيذ...':editingProfileId?'حفظ التعديلات':'إنشاء الملف'}</button>{editingProfileId&&<button className="btn btn-light" type="button" disabled={busy} onClick={resetProfile}>إلغاء التعديل</button>}</div>
   </form>
   <div className="table-responsive mt-3"><table className="table table-sm align-middle hotspot-data-table"><thead><tr><th>الاسم</th><th>حد السرعة</th><th>الأجهزة</th><th>مدة الجلسة</th><th>مدة الخمول</th><th>إجراءات</th></tr></thead><tbody>
    {overview.profiles.map(profile=><tr key={profile.id||profile.name}><td>{profile.name||'—'}</td><td>{profile.rateLimit||'—'}</td><td>{profile.sharedUsers||'—'}</td><td>{profile.sessionTimeout||'—'}</td><td>{profile.idleTimeout||'—'}</td><td><div className="d-flex flex-wrap gap-1"><button className="btn btn-sm btn-outline-secondary" disabled={busy||!profile.id} onClick={()=>{setEditingProfileId(profile.id);setProfileForm({name:profile.name,rateLimit:profile.rateLimit,sharedUsers:profile.sharedUsers||'1',sessionTimeout:profile.sessionTimeout,idleTimeout:profile.idleTimeout});}}>تعديل</button><button className="btn btn-sm btn-outline-danger" disabled={busy||!profile.id} onClick={()=>void removeProfile(profile.id,profile.name)}>حذف</button></div></td></tr>)}
    {!overview.profiles.length&&<tr><td colSpan={6} className="text-center text-secondary py-3">لا توجد ملفات تعريفية في القائمة.</td></tr>}
   </tbody></table></div>
  </section>

  <section className="hotspot-management-section">
   <div className="hotspot-section-title"><h3>الجلسات النشطة</h3><span>{overview.counts.activeSessions} جلسة</span></div>
   <div className="table-responsive"><table className="table table-sm align-middle hotspot-data-table"><thead><tr><th>المستخدم</th><th>عنوان IP</th><th>عنوان MAC</th><th>مدة الاتصال</th><th>الخادم</th></tr></thead><tbody>
    {overview.activeSessions.map(row=><tr key={row.id||row.user+row.address}><td>{row.user||'—'}</td><td dir="ltr">{row.address||'—'}</td><td dir="ltr">{row.macAddress||'—'}</td><td>{row.uptime||'—'}</td><td>{row.server||'—'}</td></tr>)}
    {!overview.activeSessions.length&&<tr><td colSpan={5} className="text-center text-secondary py-3">لا توجد جلسات نشطة حاليًا.</td></tr>}
   </tbody></table></div>
  </section>

  <section className="hotspot-management-section">
   <div className="hotspot-section-title"><h3>خوادم HotSpot</h3><span>{overview.counts.servers} خادم</span></div>
   <div className="table-responsive"><table className="table table-sm align-middle hotspot-data-table"><thead><tr><th>الاسم</th><th>الواجهة</th><th>مجمع العناوين</th><th>الملف</th><th>الحالة</th></tr></thead><tbody>
    {overview.servers.map(row=><tr key={row.name}><td>{row.name||'—'}</td><td>{row.interface||'—'}</td><td>{row.addressPool||'—'}</td><td>{row.profile||'—'}</td><td><span className={row.disabled?'text-danger':'text-success'}>{row.disabled?'معطّل':'مفعّل'}</span></td></tr>)}
    {!overview.servers.length&&<tr><td colSpan={5} className="text-center text-secondary py-3">لم يتم إعداد خادم HotSpot على هذا الراوتر.</td></tr>}
   </tbody></table></div>
  </section>
 </div>;
}
