import {useEffect,useState,type FormEvent} from 'react';
import {api} from '../api';
import {formatT,t,type Language} from '../i18n';
import type {PackageSelectionMode,PointOfSale,PointOfSaleInput,PointOfSalePackageRule} from '../types';

type FlashKind='success'|'danger';
type Props={language:Language;currencyCode:string;currencySymbol:string;onMessage:(kind:FlashKind,message:string)=>void};
type POSForm={code:string;name:string;phone:string;address:string;notes:string;active:boolean};
const emptyForm:POSForm={code:'',name:'',phone:'',address:'',notes:'',active:true};
function formatAmount(minor:number,code:string,symbol:string,language:Language){
 return (minor/100).toLocaleString(language==='ar'?'ar-YE':'en-US',{minimumFractionDigits:2,maximumFractionDigits:2})+' '+(symbol||code);
}

export default function PointOfSaleManager({language,currencyCode,currencySymbol,onMessage}:Props){
 const tx=(text:string)=>t(language,text);
 const [points,setPoints]=useState<PointOfSale[]>([]);
 const [selectedID,setSelectedID]=useState<number|null>(null);
 const [form,setForm]=useState<POSForm>(emptyForm);
 const [editingID,setEditingID]=useState<number|null>(null);
 const [rules,setRules]=useState<PointOfSalePackageRule[]>([]);
 const [selectionMode,setSelectionMode]=useState<PackageSelectionMode>('all');
 const [loading,setLoading]=useState(true);
 const [loadingRules,setLoadingRules]=useState(false);
 const [saving,setSaving]=useState(false);
 const [savingRules,setSavingRules]=useState(false);
 const [includeInactive,setIncludeInactive]=useState(true);
 const [reloadKey,setReloadKey]=useState(0);
 const selected=points.find(point=>point.id===selectedID)??null;
 const activeCount=points.filter(point=>point.active).length;
 const enabledPackageCount=rules.filter(rule=>rule.isEnabled&&rule.packageActive).length;

 useEffect(()=>{
  let alive=true;
  async function load(){
   setLoading(true);
   try{
    const result=await api.pointsOfSale(true);
    if(!alive)return;
    setPoints(result.pointsOfSale);
    const next=result.pointsOfSale.find(item=>item.id===selectedID)??result.pointsOfSale.find(item=>item.active)??result.pointsOfSale[0]??null;
    setSelectedID(next?.id??null);
    if(!editingID&&next){setEditingID(next.id);setForm({code:next.code,name:next.name,phone:next.phone,address:next.address,notes:next.notes,active:next.active});}
   }catch(error){if(alive)onMessage('danger',tx((error as Error).message))}
   finally{if(alive)setLoading(false)}
  }
  void load();
  return()=>{alive=false};
 },[reloadKey]);

 useEffect(()=>{
  if(!selectedID){setRules([]);setSelectionMode('all');return}
  let alive=true;
  async function loadRules(){
   setLoadingRules(true);
   try{
    const result=await api.pointOfSalePackages(selectedID,true);
    if(alive){setRules(result.packages);setSelectionMode(result.pointOfSale.packageSelectionMode)}
   }catch(error){if(alive)onMessage('danger',tx((error as Error).message))}
   finally{if(alive)setLoadingRules(false)}
  }
  void loadRules();
  return()=>{alive=false};
 },[selectedID,reloadKey]);

 function resetForm(){
  setEditingID(null);
  setForm({...emptyForm});
 }
 function selectPoint(point:PointOfSale){
  setSelectedID(point.id);
  setEditingID(point.id);
  setForm({code:point.code,name:point.name,phone:point.phone,address:point.address,notes:point.notes,active:point.active});
 }
 function startEdit(point:PointOfSale){
  selectPoint(point);
  setEditingID(point.id);
 }
 async function savePoint(event:FormEvent){
  event.preventDefault();
  const current=points.find(point=>point.id===editingID);
  const payload:PointOfSaleInput={
   ...form,code:form.code.trim().toUpperCase(),name:form.name.trim(),phone:form.phone.trim(),
   address:form.address.trim(),notes:form.notes.trim(),
   packageSelectionMode:current?.packageSelectionMode??'all',
  };
  setSaving(true);
  try{
   if(editingID===null){
    const created=await api.createPointOfSale({...payload,packageSelectionMode:'all'});
    setPoints(old=>[created,...old.filter(item=>item.id!==created.id)]);
    setSelectedID(created.id);
    setForm({code:created.code,name:created.name,phone:created.phone,address:created.address,notes:created.notes,active:created.active});
    setEditingID(created.id);
    onMessage('success',formatT(language,'تم إنشاء نقطة البيع «{name}».',{name:created.name}));
   }else{
    const updated=await api.updatePointOfSale(editingID,payload);
    setPoints(old=>old.map(item=>item.id===updated.id?updated:item));
    setSelectedID(updated.id);
    onMessage('success',formatT(language,'تم تحديث نقطة البيع «{name}».',{name:updated.name}));
   }
   setReloadKey(value=>value+1);
  }catch(error){onMessage('danger',tx((error as Error).message))}
  finally{setSaving(false)}
 }
 async function togglePoint(point:PointOfSale){
  setSaving(true);
  try{
   const updated=point.active?await api.archivePointOfSale(point.id):await api.updatePointOfSale(point.id,{
    code:point.code,name:point.name,phone:point.phone,address:point.address,notes:point.notes,
    packageSelectionMode:point.packageSelectionMode,active:true,
   });
   setPoints(old=>old.map(item=>item.id===updated.id?updated:item));
   if(editingID===updated.id)setForm({code:updated.code,name:updated.name,phone:updated.phone,address:updated.address,notes:updated.notes,active:updated.active});
   onMessage('success',tx(updated.active?'تم تفعيل نقطة البيع.':'تم إيقاف نقطة البيع مع الاحتفاظ بسجلاتها.'));
   setReloadKey(value=>value+1);
  }catch(error){onMessage('danger',tx((error as Error).message))}
  finally{setSaving(false)}
 }
 function changeSelectionMode(next:PackageSelectionMode){
  const previousDefault=selectionMode==='all';
  const nextDefault=next==='all';
  setSelectionMode(next);
  setRules(current=>current.map(rule=>{
   const hasExplicitChoice=rule.hasRule||rule.isEnabled!==previousDefault||Boolean(rule.prefixOverride?.trim());
   return {...rule,isEnabled:hasExplicitChoice?rule.isEnabled:nextDefault};
  }));
 }
 async function savePackageRules(event:FormEvent){
  event.preventDefault();
  if(!selected){onMessage('danger',tx('اختر نقطة بيع أولًا.'));return}
  const payload=rules.map(rule=>({
   packageId:rule.packageId,isEnabled:rule.isEnabled,
   prefixOverride:rule.prefixOverride?.trim()?rule.prefixOverride.trim():null,
   templateIdOverride:rule.templateIdOverride,
  }));
  setSavingRules(true);
  try{
   const result=await api.replacePointOfSalePackages(selected.id,selectionMode,payload);
   setRules(result.packages);
   setSelectionMode(result.pointOfSale.packageSelectionMode);
   setPoints(old=>old.map(item=>item.id===result.pointOfSale.id?result.pointOfSale:item));
   onMessage('success',formatT(language,'تم حفظ قواعد الباقات لنقطة البيع «{name}».',{name:result.pointOfSale.name}));
   setReloadKey(value=>value+1);
  }catch(error){onMessage('danger',tx((error as Error).message))}
  finally{setSavingRules(false)}
 }
 function toggleRule(packageId:number,isEnabled:boolean){
  setRules(current=>current.map(rule=>rule.packageId===packageId?{...rule,isEnabled}:rule));
 }
 function changePrefix(packageId:number,value:string){
  setRules(current=>current.map(rule=>rule.packageId===packageId?{...rule,prefixOverride:value||null}:rule));
 }

 return <section className="hotspot-management-section point-of-sale-manager" id="points-of-sale">
  <div className="hotspot-section-title"><div><h3>{tx('نقاط البيع والموزعون')}</h3><p className="small text-secondary mb-0 mt-1">{tx('سجل نقاط البيع، وأدر بيانات كل موزع، وحدد الباقات المسموح له باستلامها وبيعها.')}</p></div><span>{tx('التوزيع')}</span></div>

  <div className="row g-3 pos-stat-row mb-3">
   <div className="col-sm-4"><div className="pos-stat"><small>{tx('إجمالي نقاط البيع')}</small><strong>{points.length}</strong></div></div>
   <div className="col-sm-4"><div className="pos-stat"><small>{tx('نقاط بيع نشطة')}</small><strong>{activeCount}</strong></div></div>
   <div className="col-sm-4"><div className="pos-stat"><small>{tx('باقات نشطة ومسموحة')}</small><strong>{enabledPackageCount}</strong></div></div>
  </div>

  <div className="row g-4">
   <div className="col-xl-5">
    <section className="pos-subsection">
     <div className="hotspot-section-title"><h3>{tx('سجل نقاط البيع')}</h3><span>{points.length}</span></div>
     <div className="d-flex flex-wrap align-items-center justify-content-between gap-2 mb-2">
      <button type="button" className="btn btn-sm btn-primary" onClick={resetForm}><i className="bi bi-plus-lg ms-1"/>{tx('إضافة نقطة بيع')}</button>
      <label className="form-check mb-0"><input className="form-check-input" type="checkbox" checked={includeInactive} onChange={event=>setIncludeInactive(event.target.checked)}/><span className="form-check-label">{tx('إظهار الموقوفة')}</span></label>
     </div>
     <div className="list-group pos-list">
      {points.filter(point=>includeInactive||point.active).map(point=><button key={point.id} type="button" className={'list-group-item list-group-item-action '+(selectedID===point.id?'active':'')} onClick={()=>selectPoint(point)}>
       <div className="d-flex align-items-center justify-content-between gap-2"><strong>{point.name}</strong><span className={'badge '+(point.active?'text-bg-success':'text-bg-secondary')}>{tx(point.active?'نشطة':'موقوفة')}</span></div>
       <div className="small mt-1">{tx('الرمز')}: <b dir="ltr">{point.code}</b> · {formatT(language,'{count} قاعدة باقات',{count:point.packageRuleCount})}</div>
       {point.phone&&<div className="small mt-1" dir="ltr">{point.phone}</div>}
      </button>)}
      {!points.length&&<div className="text-secondary small py-3">{loading?tx('جارٍ تحميل نقاط البيع...'):tx('لا توجد نقاط بيع مسجلة. أضف أول موزع للبدء.')}</div>}
     </div>
    </section>
   </div>
   <div className="col-xl-7">
    <section className="pos-subsection">
     <div className="hotspot-section-title"><h3>{tx(editingID!==null?'بيانات نقطة البيع':'إضافة نقطة بيع جديدة')}</h3>{selected&&<span>{selected.code}</span>}</div>
     <form className="hotspot-editor" onSubmit={savePoint}>
      <div className="row g-3">
       <div className="col-md-5"><label className="form-label" htmlFor="posCode">{tx('رمز نقطة البيع')}</label><input id="posCode" className="form-control ltr-input" value={form.code} onChange={event=>setForm(old=>({...old,code:event.target.value.toUpperCase()}))} maxLength={40} pattern="[A-Za-z0-9_-]+" placeholder="POS-001" required/><div className="form-hint">{tx('أحرف إنجليزية وأرقام وشرطة وشرطة سفلية فقط.')}</div></div>
       <div className="col-md-7"><label className="form-label" htmlFor="posName">{tx('اسم نقطة البيع أو الموزع')}</label><input id="posName" className="form-control" value={form.name} onChange={event=>setForm(old=>({...old,name:event.target.value}))} maxLength={120} required/></div>
       <div className="col-md-6"><label className="form-label" htmlFor="posPhone">{tx('رقم الهاتف')}</label><input id="posPhone" className="form-control ltr-input" value={form.phone} onChange={event=>setForm(old=>({...old,phone:event.target.value}))} maxLength={40}/></div>
       <div className="col-md-6"><label className="form-label" htmlFor="posAddress">{tx('العنوان')}</label><input id="posAddress" className="form-control" value={form.address} onChange={event=>setForm(old=>({...old,address:event.target.value}))} maxLength={240}/></div>
       <div className="col-12"><label className="form-label" htmlFor="posNotes">{tx('ملاحظات')}</label><textarea id="posNotes" className="form-control" rows={2} value={form.notes} onChange={event=>setForm(old=>({...old,notes:event.target.value}))} maxLength={1000}/></div>
       <div className="col-12"><label className="form-check"><input className="form-check-input" type="checkbox" checked={form.active} onChange={event=>setForm(old=>({...old,active:event.target.checked}))}/><span className="form-check-label">{tx('نقطة البيع نشطة')}</span></label></div>
      </div>
      <div className="helper-box mt-3"><i className="bi bi-shield-check"/><span>{tx('الإيقاف لا يحذف نقطة البيع ولا قواعدها. يظل السجل متاحًا للمراجعة، ويمكن إعادة تفعيله لاحقًا.')}</span></div>
      <div className="d-flex flex-wrap gap-2 mt-3">
       <button type="submit" className="btn btn-primary" disabled={saving}>{saving?tx('جارٍ الحفظ...'):editingID===null?tx('إنشاء نقطة البيع'):tx('حفظ التغييرات')}</button>
       {editingID!==null&&<button type="button" className="btn btn-light" onClick={resetForm} disabled={saving}>{tx('إضافة نقطة جديدة')}</button>}
       {selected&&<button type="button" className="btn btn-outline-secondary" onClick={()=>void togglePoint(selected)} disabled={saving}>{tx(selected.active?'إيقاف نقطة البيع':'إعادة تفعيل نقطة البيع')}</button>}
      </div>
     </form>
    </section>
   </div>
  </div>

  {selected&&<section className="pos-subsection mt-4">
   <div className="hotspot-section-title"><div><h3>{tx('تخصيص الباقات')}</h3><p className="small text-secondary mb-0 mt-1">{formatT(language,'نقطة البيع الحالية: {name} ({code})',{name:selected.name,code:selected.code})}</p></div><span>{tx('قواعد التوزيع')}</span></div>
   <form className="hotspot-editor" onSubmit={savePackageRules}>
    <div className="row g-3 align-items-end">
     <div className="col-lg-5"><label className="form-label" htmlFor="posPackageMode">{tx('طريقة إتاحة الباقات')}</label><select id="posPackageMode" className="form-select form-control" value={selectionMode} onChange={event=>changeSelectionMode(event.target.value as PackageSelectionMode)}><option value="all">{tx('كل الباقات النشطة مسموحة افتراضيًا')}</option><option value="selected">{tx('الباقات المحددة فقط مسموحة')}</option></select></div>
     <div className="col-lg-7"><div className="helper-box mb-0"><i className="bi bi-info-circle"/><span>{selectionMode==='all'?tx('في هذا الوضع، أي باقة جديدة تكون متاحة تلقائيًا ما لم تُستثنَ صراحةً.'):tx('في هذا الوضع، لن تكون أي باقة متاحة حتى يتم تحديدها وتمكينها لهذه النقطة.')}</span></div></div>
    </div>
    <div className="table-responsive mt-3"><table className="table table-sm align-middle hotspot-data-table">
     <thead><tr><th>{tx('مسموح')}</th><th>{tx('الباقة')}</th><th>{tx('نوع الخدمة / الملف')}</th><th>{tx('سعر المستخدم')}</th><th>{tx('سعر الموزع')}</th><th>{tx('بادئة بديلة')}</th></tr></thead>
     <tbody>
      {rules.map(rule=><tr key={rule.packageId}>
       <td><input className="form-check-input" type="checkbox" checked={rule.isEnabled} onChange={event=>toggleRule(rule.packageId,event.target.checked)} aria-label={tx('مسموح')+' '+rule.packageName}/></td>
       <td><strong>{rule.packageName}</strong><div className={'small '+(rule.packageActive?'text-success':'text-secondary')}>{tx(rule.packageActive?'باقة نشطة':'الباقة موقوفة')}</div></td>
       <td><span>{rule.serviceSystem==='hotspot'?'HotSpot':'User Manager'}</span><div className="small text-secondary">{rule.serviceProfileName}</div></td>
       <td>{formatAmount(rule.userPriceMinor,currencyCode,currencySymbol,language)}</td><td>{formatAmount(rule.dealerPriceMinor,currencyCode,currencySymbol,language)}</td>
       <td><input className="form-control form-control-sm ltr-input" value={rule.prefixOverride??''} onChange={event=>changePrefix(rule.packageId,event.target.value)} maxLength={32} pattern="[A-Za-z0-9_-]*" placeholder={tx('استخدم بادئة الباقة')}/></td>
      </tr>)}
      {!rules.length&&<tr><td colSpan={6} className="text-center text-secondary py-3">{loadingRules?tx('جارٍ تحميل الباقات...'):tx('لا توجد باقات في الكتالوج. أنشئ باقة أولًا من دليل باقات الكروت.')}</td></tr>}
     </tbody>
    </table></div>
    <div className="helper-box mt-3"><i className="bi bi-shield-check"/><span>{tx('تُحفظ القواعد في معاملة واحدة. البادئة البديلة اختيارية، والقيمة الفارغة تعني استخدام بادئة الباقة الأساسية. لا يؤدي الإسناد أو الحفظ إلى إصدار كروت أو تسجيل بيع.')}</span></div>
    <button type="submit" className="btn btn-primary mt-3" disabled={savingRules||loadingRules||!selected.active}>{savingRules?tx('جارٍ حفظ قواعد الباقات...'):tx('حفظ قواعد الباقات')}</button>
    {!selected.active&&<p className="small text-secondary mt-2 mb-0">{tx('نقطة البيع موقوفة؛ أعد تفعيلها قبل حفظ قواعد جديدة.')}</p>}
   </form>
  </section>}
 </section>;
}
