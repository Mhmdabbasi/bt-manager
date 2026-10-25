import {useEffect,useMemo,useState,type FormEvent} from 'react';
import {api} from '../api';
import {formatT,t,type Language} from '../i18n';
import type {CardPackage,CardPackageInput,ServiceProfileRecord,ServiceSystem} from '../types';

type FlashKind='success'|'danger';
type Props={
 language:Language;
 routerId:number|null;
 currencyCode:string;
 currencySymbol:string;
 onMessage:(kind:FlashKind,message:string)=>void;
 onRefreshHotSpot:()=>void;
 onRefreshUserManager:()=>void;
};
type PackageForm={
 name:string;
 serviceSystem:ServiceSystem;
 serviceProfileId:string;
 description:string;
 userPrice:string;
 dealerPrice:string;
 defaultPrefix:string;
 active:boolean;
 sortOrder:string;
};
const emptyForm:PackageForm={
 name:'',serviceSystem:'hotspot',serviceProfileId:'',description:'',
 userPrice:'',dealerPrice:'0',defaultPrefix:'',active:true,sortOrder:'0'
};
function formatAmount(minor:number,code:string,symbol:string,language:Language){
 return (minor/100).toLocaleString(language==='ar'?'ar-YE':'en-US',{minimumFractionDigits:2,maximumFractionDigits:2})+' '+(symbol||code);
}
function priceToMinor(value:string):number{
 const amount=Number(value);
 return Number.isFinite(amount)?Math.round(amount*100):0;
}
function profileLabel(profile:ServiceProfileRecord,language:Language){
 return profile.name+(profile.present?'':' · '+t(language,'غير موجود حاليًا على الراوتر'));
}

export default function CardPackageManager({language,routerId,currencyCode,currencySymbol,onMessage,onRefreshHotSpot,onRefreshUserManager}:Props){
 const tx=(text:string)=>t(language,text);
 const [packages,setPackages]=useState<CardPackage[]>([]);
 const [profiles,setProfiles]=useState<ServiceProfileRecord[]>([]);
 const [form,setForm]=useState<PackageForm>(emptyForm);
 const [editingID,setEditingID]=useState<number|null>(null);
 const [loading,setLoading]=useState(true);
 const [saving,setSaving]=useState(false);
 const [includeInactive,setIncludeInactive]=useState(true);
 const [reloadKey,setReloadKey]=useState(0);

 useEffect(()=>{
  let active=true;
  async function load(){
   setLoading(true);
   try{
    const packageResult=await api.cardPackages(true);
    let imported:ServiceProfileRecord[]=[];
    if(routerId){
     const [hotspotResult,userManagerResult]=await Promise.all([
      api.serviceProfiles(routerId,'hotspot',true),
      api.serviceProfiles(routerId,'usermanager',true),
     ]);
     imported=[...hotspotResult.profiles,...userManagerResult.profiles];
    }
    if(active){setPackages(packageResult.packages);setProfiles(imported)}
   }catch(error){
    if(active)onMessage('danger',(error as Error).message);
   }finally{
    if(active)setLoading(false);
   }
  }
  void load();
  return()=>{active=false};
 },[routerId,reloadKey]);

 const profilesForSystem=useMemo(()=>profiles.filter(profile=>profile.sourceSystem===form.serviceSystem),[profiles,form.serviceSystem]);
 const visiblePackages=useMemo(()=>packages.filter(pkg=>includeInactive||pkg.active),[packages,includeInactive]);
 const activeCount=packages.filter(pkg=>pkg.active).length;
 const missingProfileCount=packages.filter(pkg=>!pkg.serviceProfilePresent).length;
 const parsedUserPrice=priceToMinor(form.userPrice);
 const parsedDealerPrice=priceToMinor(form.dealerPrice);
 const validPrice=Number.isFinite(Number(form.userPrice))&&form.userPrice.trim()!==''&&parsedUserPrice>0&&parsedUserPrice<=1_000_000_000_000&&
  Number.isFinite(Number(form.dealerPrice))&&form.dealerPrice.trim()!==''&&parsedDealerPrice>=0&&parsedDealerPrice<=parsedUserPrice;
 const selectedProfile=profiles.find(profile=>profile.id===Number(form.serviceProfileId));
 const selectedProfileCanBeSaved=Boolean(selectedProfile&&(selectedProfile.present||(editingID!==null&&!form.active)));

 function resetForm(){
  setEditingID(null);
  setForm({...emptyForm,serviceProfileId:String(profiles.find(profile=>profile.sourceSystem==='hotspot'&&profile.present)?.id??'')});
 }
 function startEdit(pkg:CardPackage){
  setEditingID(pkg.id);
  setForm({
   name:pkg.name,serviceSystem:pkg.serviceSystem,serviceProfileId:String(pkg.serviceProfileId),
   description:pkg.description,userPrice:(pkg.userPriceMinor/100).toFixed(2),
   dealerPrice:(pkg.dealerPriceMinor/100).toFixed(2),defaultPrefix:pkg.defaultPrefix,
   active:pkg.active,sortOrder:String(pkg.sortOrder),
  });
 }
 function changeSystem(serviceSystem:ServiceSystem){
  const first=profiles.find(profile=>profile.sourceSystem===serviceSystem&&profile.present)||
   profiles.find(profile=>profile.sourceSystem===serviceSystem);
  setForm(old=>({...old,serviceSystem,serviceProfileId:first?String(first.id):''}));
 }
 async function submit(event:FormEvent){
  event.preventDefault();
  if(!selectedProfile||selectedProfile.sourceSystem!==form.serviceSystem){
   onMessage('danger',tx('اختر ملف خدمة مستوردًا من النوع المحدد.'));
   return;
  }
  if(!selectedProfile.present&&(editingID===null||form.active)){
   onMessage('danger',tx('الملف التعريفي غير موجود حاليًا على الراوتر؛ لا يمكن إنشاء باقة جديدة أو تفعيلها حتى يعود الملف التعريفي.'));
   return;
  }
  if(!validPrice){
   onMessage('danger',tx('تحقق من سعر المستخدم وسعر الموزع؛ يجب ألا يتجاوز سعر الموزع سعر المستخدم.'));
   return;
  }
  const order=Number(form.sortOrder);
  if(!Number.isInteger(order)||order<0||order>1_000_000){
   onMessage('danger',tx('ترتيب العرض يجب أن يكون عددًا صحيحًا من صفر إلى مليون.'));
   return;
  }
  const payload:CardPackageInput={
   name:form.name.trim(),serviceSystem:form.serviceSystem,serviceProfileId:Number(form.serviceProfileId),
   description:form.description.trim(),userPriceMinor:parsedUserPrice,dealerPriceMinor:parsedDealerPrice,
   defaultPrefix:form.defaultPrefix.trim(),active:form.active,sortOrder:order,
  };
  setSaving(true);
  try{
   if(editingID===null){
    const created=await api.createCardPackage(payload);
    setPackages(old=>[created,...old.filter(pkg=>pkg.id!==created.id)]);
    onMessage('success',formatT(language,'تم إنشاء باقة الكروت «{name}».',{name:created.name}));
   }else{
    const updated=await api.updateCardPackage(editingID,payload);
    setPackages(old=>old.map(pkg=>pkg.id===updated.id?updated:pkg));
    onMessage('success',formatT(language,'تم تحديث باقة الكروت «{name}».',{name:updated.name}));
   }
   resetForm();
   setReloadKey(key=>key+1);
  }catch(error){
   onMessage('danger',(error as Error).message);
  }finally{
   setSaving(false);
  }
 }
 function toggleInactive(){
  setIncludeInactive(value=>!value);
 }
 async function togglePackage(pkg:CardPackage){
  const payload:CardPackageInput={
   name:pkg.name,serviceSystem:pkg.serviceSystem,serviceProfileId:pkg.serviceProfileId,
   description:pkg.description,userPriceMinor:pkg.userPriceMinor,dealerPriceMinor:pkg.dealerPriceMinor,
   defaultPrefix:pkg.defaultPrefix,active:!pkg.active,sortOrder:pkg.sortOrder,
  };
  setSaving(true);
  try{
   const updated=pkg.active?await api.archiveCardPackage(pkg.id):await api.updateCardPackage(pkg.id,payload);
   setPackages(old=>old.map(item=>item.id===updated.id?updated:item));
   onMessage('success',formatT(language,'تم تحديث باقة الكروت «{name}».',{name:updated.name}));
   setReloadKey(key=>key+1);
  }catch(error){onMessage('danger',(error as Error).message)}
  finally{setSaving(false)}
 }

 return <section className="hotspot-management-section card-package-manager" id="card-packages">
  <div className="hotspot-section-title">
   <div><h3>{tx('دليل باقات الكروت')}</h3><p className="small text-secondary mb-0 mt-1">{tx('اربط الباقة التجارية بملف HotSpot أو User Manager المستورد، وحدد أسعار المستخدم والموزع.')}</p></div>
   <span>{tx('كتالوج الباقات')}</span>
  </div>

  <div className="row g-3 pos-stat-row mb-3">
   <div className="col-sm-4"><div className="pos-stat"><small>{tx('إجمالي الباقات')}</small><strong>{packages.length}</strong></div></div>
   <div className="col-sm-4"><div className="pos-stat"><small>{tx('باقات نشطة')}</small><strong>{activeCount}</strong></div></div>
   <div className="col-sm-4"><div className="pos-stat"><small>{tx('ملفات غير موجودة حاليًا')}</small><strong>{missingProfileCount}</strong></div></div>
  </div>

  {!routerId&&<div className="helper-box mb-3"><i className="bi bi-router"/><span>{tx('استورد ملفات التعريف من الراوتر أولًا حتى تتمكن من ربط الباقات بها. يمكنك استيراد HotSpot من الإعدادات أو مزامنة User Manager من صفحته.')}</span></div>}
  {routerId&&profiles.length===0&&<div className="helper-box mb-3"><i className="bi bi-info-circle"/><span>{tx('لم تُستورد ملفات تعريف لهذا الراوتر بعد. حدّث HotSpot أو زامن User Manager قبل إضافة باقة.')}</span></div>}
  <div className="d-flex flex-wrap gap-2 mb-3">
   <button type="button" className="btn btn-sm btn-outline-primary" onClick={onRefreshHotSpot}><i className="bi bi-arrow-repeat ms-1"/>{tx('استيراد ملفات HotSpot')}</button>
   <button type="button" className="btn btn-sm btn-outline-primary" onClick={onRefreshUserManager}><i className="bi bi-arrow-repeat ms-1"/>{tx('مزامنة User Manager')}</button>
   <button type="button" className="btn btn-sm btn-outline-secondary" onClick={()=>setReloadKey(key=>key+1)} disabled={loading}><i className="bi bi-arrow-clockwise ms-1"/>{tx('تحديث الكتالوج المحلي')}</button>
  </div>

  <form className="hotspot-editor" onSubmit={submit}>
   <h4>{editingID!==null?tx('تعديل باقة كروت'):tx('إضافة باقة كروت')}</h4>
   <div className="row g-3">
    <div className="col-md-6"><label className="form-label" htmlFor="cardPackageName">{tx('اسم الباقة')}</label><input id="cardPackageName" className="form-control" value={form.name} onChange={event=>setForm(old=>({...old,name:event.target.value}))} maxLength={80} required/></div>
    <div className="col-md-3"><label className="form-label" htmlFor="cardPackageSystem">{tx('نوع الخدمة')}</label><select id="cardPackageSystem" className="form-select form-control" value={form.serviceSystem} onChange={event=>changeSystem(event.target.value as ServiceSystem)}><option value="hotspot">HotSpot</option><option value="usermanager">User Manager</option></select></div>
    <div className="col-md-3"><label className="form-label" htmlFor="cardPackageProfile">{tx('الملف التعريفي المستورد')}</label><select id="cardPackageProfile" className="form-select form-control" value={form.serviceProfileId} onChange={event=>setForm(old=>({...old,serviceProfileId:event.target.value}))} required><option value="">{tx('اختر ملفًا تعريفيًا')}</option>{profilesForSystem.map(profile=><option key={profile.id} value={profile.id}>{profileLabel(profile,language)}</option>)}</select></div>
    <div className="col-md-6"><label className="form-label" htmlFor="cardPackageDescription">{tx('الوصف (اختياري)')}</label><input id="cardPackageDescription" className="form-control" value={form.description} onChange={event=>setForm(old=>({...old,description:event.target.value}))} maxLength={1000}/></div>
    <div className="col-md-3"><label className="form-label" htmlFor="cardPackageUserPrice">{tx('سعر المستخدم')} ({currencySymbol||currencyCode})</label><input id="cardPackageUserPrice" type="number" step="0.01" min="0.01" max="10000000000" className="form-control ltr-input" value={form.userPrice} onChange={event=>setForm(old=>({...old,userPrice:event.target.value}))} required/></div>
    <div className="col-md-3"><label className="form-label" htmlFor="cardPackageDealerPrice">{tx('سعر الموزع')} ({currencySymbol||currencyCode})</label><input id="cardPackageDealerPrice" type="number" step="0.01" min="0" max="10000000000" className="form-control ltr-input" value={form.dealerPrice} onChange={event=>setForm(old=>({...old,dealerPrice:event.target.value}))} required/><div className="form-hint">{tx('يمكن جعل سعر الموزع صفرًا أو أقل من سعر المستخدم.')}</div></div>
    <div className="col-md-6"><label className="form-label" htmlFor="cardPackagePrefix">{tx('بادئة رمز الدخول الافتراضية')}</label><input id="cardPackagePrefix" className="form-control ltr-input" value={form.defaultPrefix} onChange={event=>setForm(old=>({...old,defaultPrefix:event.target.value}))} maxLength={32} pattern="[A-Za-z0-9_-]*" placeholder="P200-"/><div className="form-hint">{tx('حروف إنجليزية وأرقام وشرطة وشرطة سفلية فقط.')}</div></div>
    <div className="col-md-3"><label className="form-label" htmlFor="cardPackageOrder">{tx('ترتيب العرض')}</label><input id="cardPackageOrder" type="number" min="0" max="1000000" step="1" className="form-control ltr-input" value={form.sortOrder} onChange={event=>setForm(old=>({...old,sortOrder:event.target.value}))} required/></div>
    <div className="col-md-3 d-flex align-items-end"><label className="form-check mb-2"><input className="form-check-input" type="checkbox" checked={form.active} onChange={event=>setForm(old=>({...old,active:event.target.checked}))}/><span className="form-check-label">{tx('الباقة نشطة')}</span></label></div>
   </div>
   {selectedProfile&&!selectedProfile.present&&<div className="alert alert-warning mt-3 mb-0">{tx('الملف التعريفي المحدد غير موجود حاليًا على الراوتر. يمكنك حفظ الباقة غير نشطة، لكن لا يمكن تفعيلها حتى يعود الملف التعريفي.')}</div>}
   <div className="helper-box mt-3"><i className="bi bi-shield-check"/><span>{tx('لا يُحذف سجل الباقة نهائيًا؛ أوقفها بدلًا من حذفها حتى تبقى المراجع والتاريخ المالي المستقبلي متسقة. الأسعار تُحفظ بوحدات العملة الصغرى بدقة منزلتين عشريتين.')}</span></div>
   <div className="d-flex flex-wrap gap-2 mt-3"><button className="btn btn-primary" type="submit" disabled={saving||!validPrice||!selectedProfileCanBeSaved}>{saving?tx('جارٍ الحفظ...'):editingID!==null?tx('حفظ التغييرات'):tx('إنشاء الباقة')}</button>{editingID!==null&&<button type="button" className="btn btn-light" onClick={resetForm} disabled={saving}>{tx('إلغاء')}</button>}</div>
  </form>

  <div className="d-flex align-items-center justify-content-between flex-wrap gap-2 mt-4 mb-2">
   <h4 className="mb-0">{tx('الباقات المسجلة')}</h4>
   <label className="form-check mb-0"><input className="form-check-input" type="checkbox" checked={includeInactive} onChange={toggleInactive}/><span className="form-check-label">{tx('إظهار الباقات الموقوفة')}</span></label>
  </div>
  <div className="table-responsive"><table className="table table-sm align-middle hotspot-data-table">
   <thead><tr><th>{tx('الباقة')}</th><th>{tx('المصدر')}</th><th>{tx('الملف التعريفي')}</th><th>{tx('سعر المستخدم')}</th><th>{tx('سعر الموزع')}</th><th>{tx('البادئة')}</th><th>{tx('الحالة')}</th><th>{tx('إجراءات')}</th></tr></thead>
   <tbody>
    {visiblePackages.map(pkg=><tr key={pkg.id}>
     <td><strong>{pkg.name}</strong>{pkg.description&&<div className="small text-secondary">{pkg.description}</div>}</td>
     <td>{pkg.serviceSystem==='hotspot'?'HotSpot':'User Manager'}</td>
     <td>{pkg.serviceProfileName}<div className={pkg.serviceProfilePresent?'small text-success':'small text-warning'}>{tx(pkg.serviceProfilePresent?'موجود على الراوتر':'غير موجود حاليًا على الراوتر')}</div></td>
     <td>{formatAmount(pkg.userPriceMinor,currencyCode,currencySymbol,language)}</td><td>{formatAmount(pkg.dealerPriceMinor,currencyCode,currencySymbol,language)}</td>
     <td dir="ltr">{pkg.defaultPrefix||'—'}</td>
     <td><span className={pkg.active?'text-success':'text-secondary'}>{tx(pkg.active?'نشطة':'موقوفة')}</span></td>
     <td><div className="d-flex flex-wrap gap-1"><button type="button" className="btn btn-sm btn-outline-secondary" onClick={()=>startEdit(pkg)} disabled={saving}>{tx('تعديل')}</button><button type="button" className="btn btn-sm btn-outline-primary" onClick={()=>togglePackage(pkg)} disabled={saving}>{tx(pkg.active?'إيقاف':'تفعيل')}</button></div></td>
    </tr>)}
    {!visiblePackages.length&&<tr><td colSpan={8} className="text-center text-secondary py-3">{loading?tx('جارٍ تحميل الكتالوج...'):tx('لا توجد باقات بعد. استورد ملفًا تعريفيًا ثم أنشئ الباقة الأولى.')}</td></tr>}
   </tbody>
  </table></div>
 </section>;
}
