import {useEffect,useMemo,useState,type CSSProperties,type ChangeEvent,type FormEvent} from 'react';
import {api} from '../api';
import {formatT,t,type Language} from '../i18n';
import type {CardPackage,CardTemplate,CardTemplateElement,HotSpotProfile,HotSpotServer,PointOfSale,Voucher,VoucherBatch,VoucherBatchResult} from '../types';

type RouterConnection={address:string;port:number;username:string;password:string;useTls:boolean};
type FlashKind='success'|'danger';
type CharsetMode='alphanumeric'|'letters'|'numbers';
type PasswordMode='none'|'same'|'random';
type Props={
 language:Language;
 router:RouterConnection;
 profiles:HotSpotProfile[];
 servers:HotSpotServer[];
 businessName:string;
 onRefresh:()=>Promise<void>;
 onMessage:(kind:FlashKind,message:string)=>void;
};
type LayoutKey='4x15'|'3x20'|'3x15';
const layouts:Record<LayoutKey,{label:string;columns:number;rows:number;capacity:number}>={
 '4x15':{label:'60 بطاقة — 4 أعمدة × 15 صفًا',columns:4,rows:15,capacity:60},
 '3x20':{label:'60 بطاقة — 3 أعمدة × 20 صفًا',columns:3,rows:20,capacity:60},
 '3x15':{label:'45 بطاقة — 3 أعمدة × 15 صفًا',columns:3,rows:15,capacity:45},
};
function chunk<T>(items:T[],size:number):T[][]{
 const result:T[][]=[];
 for(let i=0;i<items.length;i+=size)result.push(items.slice(i,i+size));
 return result;
}
function VoucherCard({language,voucher,title,note,accent,background}:{language:Language;voucher:Voucher;title:string;note:string;accent:string;background:string}){
 const tx=(text:string)=>t(language,text);
 const style={
  '--voucher-accent':accent,
  backgroundImage:background?'linear-gradient(125deg,rgba(255,255,255,.90),rgba(240,249,251,.84)),url('+background+')':undefined,
 } as CSSProperties;
 return <article className="voucher-card" style={style}>
  <div className="voucher-card-head"><strong>{title||'HotSpot'}</strong><span>INTERNET ACCESS</span></div>
  <div className="voucher-code-label">{tx('اسم المستخدم / رمز الدخول')}</div>
  <div className="voucher-code" dir="ltr">{voucher.username}</div>
  {voucher.password&&<><div className="voucher-code-label">{tx('كلمة المرور')}</div><div className="voucher-code" dir="ltr">{voucher.password}</div></>}
  <div className="voucher-card-meta"><span>{voucher.profileName}</span>{voucher.limitUptime&&<span>{tx('الوقت: ')}{voucher.limitUptime}</span>}</div>
  {note&&<div className="voucher-card-note">{note}</div>}
 </article>;
}

export default function VoucherStudio({language,router,profiles,servers,businessName,onRefresh,onMessage}:Props){
 const tx=(text:string)=>t(language,text);
 const [prefix,setPrefix]=useState('');
 const [count,setCount]=useState(60);
 const [usernameLength,setUsernameLength]=useState(8);
 const [usernameMode,setUsernameMode]=useState<CharsetMode>('alphanumeric');
 const [passwordMode,setPasswordMode]=useState<PasswordMode>('none');
 const [passwordLength,setPasswordLength]=useState(4);
 const [passwordCharset,setPasswordCharset]=useState<CharsetMode>('alphanumeric');
 const [profile,setProfile]=useState('');
 const [cardPackages,setCardPackages]=useState<CardPackage[]>([]);
 const [pointsOfSale,setPointsOfSale]=useState<PointOfSale[]>([]);
 const [packageID,setPackageID]=useState('');
 const [pointOfSaleID,setPointOfSaleID]=useState('');
 const [allowWithoutTemplate,setAllowWithoutTemplate]=useState(false);
 const [server,setServer]=useState('all');
 const [limitUptime,setLimitUptime]=useState('');
 const [generating,setGenerating]=useState(false);
 const [loadingBatch,setLoadingBatch]=useState(false);
 const [batches,setBatches]=useState<VoucherBatch[]>([]);
 const [selected,setSelected]=useState<VoucherBatchResult|null>(null);
 const [layoutKey,setLayoutKey]=useState<LayoutKey>('4x15');
 const [cardTitle,setCardTitle]=useState(businessName||'HotSpot');
 const [note,setNote]=useState('');
 const [accent,setAccent]=useState('#147f9f');
 const [background,setBackground]=useState('');
 const [backgroundName,setBackgroundName]=useState('');
 const [backgroundAssetId,setBackgroundAssetId]=useState(0);
 const [designElements,setDesignElements]=useState<CardTemplateElement[]>([]);
 const [selectedElementID,setSelectedElementID]=useState('username');
 const [cardWidthMM,setCardWidthMM]=useState(50);
 const [cardHeightMM,setCardHeightMM]=useState(19.1);
 const [templateName,setTemplateName]=useState('القالب الافتراضي');
 const [templateDefault,setTemplateDefault]=useState(true);
 const [templates,setTemplates]=useState<CardTemplate[]>([]);
 const [selectedTemplateID,setSelectedTemplateID]=useState('');
 const [savingTemplate,setSavingTemplate]=useState(false);
 const [templateLoading,setTemplateLoading]=useState(false);
 const [pageMarginMM,setPageMarginMM]=useState(5);
 const [pageGapMM,setPageGapMM]=useState(1);

 useEffect(()=>{
  let active=true;
  void api.voucherBatches().then(result=>{if(active)setBatches(result.batches)}).catch(error=>{if(active)onMessage('danger',(error as Error).message)});
  void Promise.all([api.cardPackages(false,'hotspot'),api.pointsOfSale(false)]).then(([packageResult,posResult])=>{
   if(!active)return;
   const address=router.address.trim().toLowerCase();
   setCardPackages(packageResult.packages.filter(pkg=>
    pkg.serviceProfilePresent&&pkg.routerAddress.trim().toLowerCase()===address&&
    pkg.routerPort===router.port&&pkg.routerUseTls===router.useTls
   ));
   setPointsOfSale(posResult.pointsOfSale.filter(point=>point.active));
  }).catch(error=>{if(active)onMessage('danger',tx((error as Error).message))});
  return ()=>{active=false};
 },[router.address,router.port,router.useTls]);
 useEffect(()=>{if(!profile&&profiles.length)setProfile(profiles[0].name)},[profiles,profile]);

 const layout=layouts[layoutKey];
 const selectedPackage=cardPackages.find(item=>item.id===Number(packageID))??null;
 const vouchers=selected?.vouchers??[];
 const sheets=useMemo(()=>chunk(vouchers,layout.capacity),[vouchers,layout.capacity]);
 const screenPreview=vouchers.slice(0,6);

 async function generate(event:FormEvent){
  event.preventDefault();
  if(packageID&&!selectedPackage){
   onMessage('danger',tx('باقة الكروت المحددة غير متاحة لهذا الراوتر.'));
   return;
  }
  if(!selectedPackage&&!profiles.some(item=>item.name===profile)){
   onMessage('danger',tx('اختر ملفًا تعريفيًا موجودًا على الراوتر قبل إنشاء القسائم.'));
   return;
  }
  if(selectedPackage&&!selectedPackage.active){
   onMessage('danger',tx('باقة الكروت المحددة موقوفة؛ فعّلها قبل إنشاء القسائم.'));
   return;
  }
  setGenerating(true);
  try{
   const generationOptions={prefix:prefix.trim(),count,server,limitUptime,usernameLength,usernameMode,passwordMode,passwordLength,passwordCharset};
   const result=selectedPackage
    ?await api.generatePackagedVouchers({
      router,packageId:selectedPackage.id,
      ...(pointOfSaleID?{pointOfSaleId:Number(pointOfSaleID)}:{}),
      allowWithoutTemplate,...generationOptions
     })
    :await api.generateVouchers({router,profile,...generationOptions});
   setSelected(result);
   setBatches(old=>[result.batch,...old.filter(batch=>batch.id!==result.batch.id)]);
   if(result.batch.failedCount>0||Boolean(result.untrackedUsernames?.length)){
    onMessage('danger',language==='ar'?'اكتملت الدفعة جزئيًا: حُفظت '+result.batch.createdCount+' من '+result.batch.requestedCount+' قسيمة محليًا. '+(result.errorMessage||''):formatT(language,'Batch partially completed: saved {created} of {requested} vouchers locally.',{created:result.batch.createdCount,requested:result.batch.requestedCount})+' '+(result.errorMessage||''));
   }else{
    onMessage('success',language==='ar'?'تم إنشاء '+result.batch.createdCount+' قسيمة وحفظها محليًا.':formatT(language,'Created {count} vouchers and saved them locally.',{count:result.batch.createdCount}));
   }
   await onRefresh();
  }catch(error){
   onMessage('danger',tx((error as Error).message||'تعذر إنشاء دفعة القسائم.'));
  }finally{setGenerating(false)}
 }

 async function chooseBatch(value:string){
  const id=Number(value);
  if(!id)return;
  setLoadingBatch(true);
  try{
   const result=await api.voucherBatch(id);
   setSelected(result);
   if(result.errorMessage)onMessage('danger',result.errorMessage);
  }catch(error){onMessage('danger',(error as Error).message)}
  finally{setLoadingBatch(false)}
 }

 async function handleBackground(event:ChangeEvent<HTMLInputElement>){
  const input=event.currentTarget;
  const file=input.files?.[0];
  if(!file)return;
  if(!['image/png','image/jpeg','image/webp'].includes(file.type)||file.size>8*1024*1024){
   onMessage('danger',tx('اختر صورة PNG أو JPG أو WebP بحجم لا يتجاوز 8 ميجابايت.'));
   input.value='';
   return;
  }
  try{
   const result=await api.uploadDesignAsset(file,'background');
   setBackground(result.url);
   setBackgroundName(result.asset.originalName);
   onMessage('success',tx('تم حفظ الخلفية على الجهاز ويمكن استرجاعها بعد إعادة التشغيل.'));
  }catch(error){
   onMessage('danger',(error as Error).message||tx('تعذر حفظ ملف الخلفية.'));
  }finally{
   input.value='';
  }
 }

 return <section className="voucher-studio hotspot-management-section" id="cards">
  <div className="hotspot-section-title"><div><h3>{tx("توليد كروت الدخول والطباعة")}</h3><p className="small text-secondary mb-0 mt-1">{tx("ينشئ حسابات HotSpot باسم المستخدم فقط، ثم يحفظ سجل القسائم على هذا الجهاز.")}</p></div><span>{tx("Voucher Studio")}</span></div>
  <form className="hotspot-editor" onSubmit={generate}>
   <h4>{tx("إنشاء دفعة جديدة")}</h4>
   <div className="row g-3">
    <div className="col-md-4"><label className="form-label" htmlFor="voucherCount">{tx("عدد الحسابات")}</label><input id="voucherCount" className="form-control ltr-input" type="number" min={1} max={200} step={1} value={count} onChange={event=>setCount(Math.max(1,Math.min(200,Number(event.target.value)||1)))} required/><div className="form-hint">{tx("من 1 إلى 200 حساب في الدفعة الواحدة.")}</div></div>
    <div className="col-md-4"><label className="form-label" htmlFor="voucherPackage">{tx("الباقة التجارية (اختياري)")}</label><select id="voucherPackage" className="form-select form-control" value={packageID} onChange={event=>{setPackageID(event.target.value);setPointOfSaleID('');setAllowWithoutTemplate(false);setPrefix('');const next=cardPackages.find(item=>item.id===Number(event.target.value));if(next)setProfile(next.serviceProfileName)}}><option value="">{tx("توليد تقليدي — دون باقة")}</option>{cardPackages.map(item=><option key={item.id} value={item.id}>{item.name} — {item.serviceProfileName}</option>)}</select><div className="form-hint">{tx("اختيار باقة يربط الدفعة بسعرها وملفها وسجل مخزونها.")}</div></div>
    <div className="col-md-4"><label className="form-label" htmlFor="voucherPrefix">{tx("بادئة اسم المستخدم (اختيارية)")}</label><input id="voucherPrefix" className="form-control ltr-input" value={prefix} onChange={event=>setPrefix(event.target.value)} maxLength={32} pattern="[A-Za-z0-9_-]*" placeholder={selectedPackage?.defaultPrefix||"BT-"}/><div className="form-hint">{tx("تُضاف البادئة قبل الرمز العشوائي.")}</div></div>
    {selectedPackage?<div className="col-md-4"><label className="form-label" htmlFor="voucherProfileFromPackage">{tx("الملف التعريفي من الباقة")}</label><input id="voucherProfileFromPackage" className="form-control" value={selectedPackage.serviceProfileName} readOnly/><div className="form-hint">{tx("سيستخدم النظام الملف المرتبط بالباقة ولا يمكن تغييره من هذه الدفعة.")}</div></div>:<div className="col-md-4"><label className="form-label" htmlFor="voucherProfile">{tx("الملف التعريفي")}</label><select id="voucherProfile" className="form-select form-control" value={profile} onChange={event=>setProfile(event.target.value)} required>{profiles.map(item=><option key={item.id||item.name} value={item.name}>{item.name}</option>)}</select></div>}
    {selectedPackage&&<div className="col-md-6"><label className="form-label" htmlFor="voucherPointOfSale">{tx("نقطة البيع / الموزع (اختياري)")}</label><select id="voucherPointOfSale" className="form-select form-control" value={pointOfSaleID} onChange={event=>setPointOfSaleID(event.target.value)}><option value="">{tx("مخزون عام — دون تخصيص لموزع")}</option>{pointsOfSale.map(point=><option key={point.id} value={point.id}>{point.name} ({point.code})</option>)}</select><div className="form-hint">{tx("سيُرفض التوليد إذا كانت الباقة غير مسموحة لنقطة البيع المحددة.")}</div></div>}
    {selectedPackage&&<div className="col-md-6 d-flex align-items-end"><label className="form-check mb-2"><input className="form-check-input" type="checkbox" checked={allowWithoutTemplate} onChange={event=>setAllowWithoutTemplate(event.target.checked)}/><span className="form-check-label">{tx("السماح بالتوليد دون قالب طباعة محفوظ")}</span><div className="form-hint">{tx("عند عدم وجود نسخة قالب محفوظة، يجب تفعيل هذا الخيار صراحةً؛ ستعتمد الطباعة على المعاينة الحالية.")}</div></label></div>}
    <div className="col-md-4"><label className="form-label" htmlFor="voucherUsernameLength">{tx("طول رمز الدخول")}</label><input id="voucherUsernameLength" className="form-control ltr-input" type="number" min={4} max={32} step={1} value={usernameLength} onChange={event=>setUsernameLength(Math.max(4,Math.min(32,Number(event.target.value)||4)))} required/><div className="form-hint">{tx("عدد المحارف، من 4 إلى 32.")}</div></div>
    <div className="col-md-4"><label className="form-label" htmlFor="voucherUsernameMode">{tx("نوع رمز الدخول")}</label><select id="voucherUsernameMode" className="form-select form-control" value={usernameMode} onChange={event=>setUsernameMode(event.target.value as CharsetMode)}><option value="alphanumeric">{tx("حروف وأرقام")}</option><option value="letters">{tx("حروف فقط")}</option><option value="numbers">{tx("أرقام فقط")}</option></select></div>
    <div className="col-md-4"><label className="form-label" htmlFor="voucherServer">{tx("خادم HotSpot")}</label><select id="voucherServer" className="form-select form-control" value={server} onChange={event=>setServer(event.target.value)}><option value="all">{tx("كل الخوادم (all)")}</option>{servers.filter(item=>!item.disabled).map(item=><option key={item.name} value={item.name}>{item.name}</option>)}</select></div>
    <div className="col-md-6"><label className="form-label" htmlFor="voucherLimit">{tx("حد الاستخدام الزمني (اختياري)")}</label><input id="voucherLimit" className="form-control ltr-input" value={limitUptime} onChange={event=>setLimitUptime(event.target.value)} maxLength={64} placeholder={tx("مثال: 1d00:00:00")}/></div>
    <div className="col-md-6"><label className="form-label" htmlFor="voucherPasswordMode">{tx("كلمة المرور")}</label><select id="voucherPasswordMode" className="form-select form-control" value={passwordMode} onChange={event=>setPasswordMode(event.target.value as PasswordMode)}><option value="none">{tx("بدون كلمة مرور (اسم المستخدم فقط)")}</option><option value="same">{tx("كلمة المرور مطابقة لاسم المستخدم")}</option><option value="random">{tx("كلمة مرور عشوائية منفصلة")}</option></select></div>
    {passwordMode==='random'&&<><div className="col-md-6"><label className="form-label" htmlFor="voucherPasswordLength">{tx("طول كلمة المرور")}</label><input id="voucherPasswordLength" className="form-control ltr-input" type="number" min={4} max={32} value={passwordLength} onChange={event=>setPasswordLength(Math.max(4,Math.min(32,Number(event.target.value)||4)))}/></div><div className="col-md-6"><label className="form-label" htmlFor="voucherPasswordCharset">{tx("نوع محارف كلمة المرور")}</label><select id="voucherPasswordCharset" className="form-select form-control" value={passwordCharset} onChange={event=>setPasswordCharset(event.target.value as CharsetMode)}><option value="alphanumeric">{tx("حروف وأرقام")}</option><option value="letters">{tx("حروف فقط")}</option><option value="numbers">{tx("أرقام فقط")}</option></select></div></>}
   </div>
   <div className="helper-box mt-3"><i className="bi bi-shield-check"/><span>{passwordMode==='none'?tx('سيُنشأ كل حساب دون كلمة مرور، بما يوافق طريقة تسجيل الدخول باسم المستخدم فقط.'):passwordMode==='same'?tx('سيُستخدم اسم المستخدم نفسه ككلمة مرور لكل حساب.'):tx('سيُنشئ التطبيق كلمة مرور عشوائية منفصلة، ويحتفظ بها محليًا لطباعتها ضمن الكرت.')} {tx('تُولّد الرموز بمصدر عشوائي آمن؛ نمطا الحروف والمختلط يستبعدان المحارف الملتبسة، أما نمط الأرقام فيستخدم 0–9 كاملة. قد تتوقف الدفعة عند أول رفض من الراوتر؛ وستُحفظ الحسابات التي أُنشئت بالفعل فقط.')}</span></div>
   <button type="submit" className="btn btn-primary mt-2" disabled={generating||(!selectedPackage&&!profiles.length)||(Boolean(packageID)&&!selectedPackage)}>{generating?tx('جارٍ إنشاء القسائم على الراوتر...'):tx('إنشاء دفعة القسائم')}</button>
  </form>

  {batches.length>0&&<div className="voucher-batch-history mt-3">
   <label className="form-label" htmlFor="voucherBatchHistory">{tx("الدفعات السابقة")}</label>
   <select id="voucherBatchHistory" className="form-select form-control" value={selected?.batch.id??''} onChange={event=>void chooseBatch(event.target.value)} disabled={loadingBatch}>
    <option value="">{tx("اختر دفعة لعرضها وطباعتها")}</option>
    {batches.map(batch=><option key={batch.id} value={batch.id}>{batch.reference} — {batch.packageNameSnapshot||batch.profileName}{batch.pointOfSaleId?' · '+tx('نقطة بيع')+' #'+batch.pointOfSaleId:''} — {batch.createdCount}/{batch.requestedCount} — {batch.usernameLength} {tx('محارف')} — {batch.status}</option>)}
   </select>
  </div>}

  {selected&&<div className="voucher-print-settings">
   <div className="hotspot-section-title mt-4"><h3>{tx("استوديو تصميم الكروت")}</h3><span>{vouchers.length} {tx('كرت متاح')}</span></div>
   <div className="row g-3">
    <div className="col-md-6"><label className="form-label" htmlFor="cardTitle">{tx("اسم الشبكة على الكرت")}</label><input id="cardTitle" className="form-control" value={cardTitle} onChange={event=>setCardTitle(event.target.value)} maxLength={80}/></div>
    <div className="col-md-6"><label className="form-label" htmlFor="cardNote">{tx("عبارة إضافية (اختيارية)")}</label><input id="cardNote" className="form-control" value={note} onChange={event=>setNote(event.target.value)} maxLength={80} placeholder={tx("للدعم أو التواصل...")}/></div>
    <div className="col-md-6"><label className="form-label" htmlFor="cardLayout">{tx("تخطيط ورقة A4")}</label><select id="cardLayout" className="form-select form-control" value={layoutKey} onChange={event=>setLayoutKey(event.target.value as LayoutKey)}>{Object.entries(layouts).map(([key,value])=><option key={key} value={key}>{tx(value.label)}</option>)}</select></div>
    <div className="col-md-3"><label className="form-label" htmlFor="cardAccent">{tx("لون الهوية")}</label><input id="cardAccent" className="form-control form-control-color" type="color" value={accent} onChange={event=>setAccent(event.target.value)} title={tx("لون الهوية")}/></div>
    <div className="col-md-3"><label className="form-label" htmlFor="cardBackground">{tx("خلفية مخصصة")}</label><input id="cardBackground" className="form-control" type="file" accept="image/png,image/jpeg,image/webp" onChange={handleBackground}/>{backgroundName&&<div className="form-hint">{backgroundName} <button type="button" className="btn btn-sm btn-link p-0" onClick={()=>{setBackground('');setBackgroundName('')}}>{tx("إزالة")}</button></div>}</div>
   </div>
   <div className="d-flex flex-wrap align-items-center gap-2 mt-3 no-print"><button type="button" className="btn btn-primary" disabled={!vouchers.length} onClick={()=>window.print()}><i className="bi bi-printer ms-1"/> {tx("طباعة / حفظ PDF")}</button><span className="small text-secondary">{tx("اختر «Save as PDF / حفظ كملف PDF» من نافذة الطباعة.")}</span></div>
   {(selected.batch.failedCount>0||Boolean(selected.untrackedUsernames?.length))&&<div className="alert alert-warning mt-3">{tx('لم تكتمل الدفعة بالكامل.')} {formatT(language,'حُفظت محليًا {created} من أصل {requested} قسيمة.',{created:selected.batch.createdCount,requested:selected.batch.requestedCount})} {selected.errorMessage}{selected.untrackedUsernames&&selected.untrackedUsernames.length>0&&<div className="mt-2"><b>{tx("أسماء أنشئت على الراوتر ولم تُحفظ محليًا:")}</b><div dir="ltr">{selected.untrackedUsernames.join(", ")}</div><small>{tx("لا تطبع هذه الرموز من الاستوديو قبل تسجيلها محليًا؛ فهي غير موجودة ضمن القسائم المحفوظة.")}</small></div>}</div>}
   {!!vouchers.length&&<div className="voucher-preview-wrap mt-3"><div className="small text-secondary mb-2">{formatT(language,'معاينة لأول {count} كروت — المعاينة للطباعة كاملة على ورق A4.',{count:screenPreview.length})}</div><div className="voucher-preview-grid">{screenPreview.map(voucher=><VoucherCard key={voucher.id} language={language} voucher={voucher} title={cardTitle} note={note} accent={accent} background={background}/>)}</div></div>}
   {!vouchers.length&&<div className="small text-secondary mt-3">{tx("لا توجد كروت ناجحة في هذه الدفعة للطباعة.")}</div>}
  </div>}

  {vouchers.length>0&&<div className="voucher-print-root" id="voucherPrintRoot">
   {sheets.map((sheet,index)=><div className={"voucher-sheet voucher-sheet-"+layoutKey} key={index} style={{'--voucher-columns':layout.columns,'--voucher-rows':layout.rows,'--voucher-accent':accent} as CSSProperties}>
    {sheet.map(voucher=><VoucherCard key={voucher.id} language={language} voucher={voucher} title={cardTitle} note={note} accent={accent} background={background}/>)}
   </div>)}
  </div>}
 </section>;
}
