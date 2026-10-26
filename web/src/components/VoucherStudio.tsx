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
type LayoutKey='4x15'|'3x20'|'3x15'|'custom';
const layouts:Record<Exclude<LayoutKey,'custom'>,{label:string;columns:number;rows:number;capacity:number}>={
 '4x15':{label:'60 بطاقة — 4 أعمدة × 15 صفًا',columns:4,rows:15,capacity:60},
 '3x20':{label:'60 بطاقة — 3 أعمدة × 20 صفًا',columns:3,rows:20,capacity:60},
 '3x15':{label:'45 بطاقة — 3 أعمدة × 15 صفًا',columns:3,rows:15,capacity:45},
};
const defaultDesign:CardTemplateElement[]=[{id:'title',field:'title',x:4,y:5,width:92,fontSize:11,color:'#147f9f',align:'right',bold:true},{id:'usernameLabel',field:'usernameLabel',x:4,y:29,width:92,fontSize:7,color:'#7e8fa1',align:'right',bold:false},{id:'username',field:'username',x:4,y:39,width:92,fontSize:17,color:'#172f44',align:'center',bold:true},{id:'passwordLabel',field:'passwordLabel',x:4,y:58,width:92,fontSize:7,color:'#7e8fa1',align:'right',bold:false},{id:'password',field:'password',x:4,y:66,width:92,fontSize:11,color:'#172f44',align:'center',bold:true},{id:'profile',field:'profile',x:4,y:84,width:45,fontSize:7,color:'#718298',align:'right',bold:false},{id:'uptime',field:'uptime',x:50,y:84,width:46,fontSize:7,color:'#718298',align:'left',bold:false},{id:'note',field:'note',x:4,y:92,width:92,fontSize:7,color:'#687b90',align:'center',bold:false}];
const elementLabels:Record<CardTemplateElement['field'],string>={title:'اسم الشبكة',usernameLabel:'عنوان رمز الدخول',username:'اسم المستخدم',passwordLabel:'عنوان كلمة المرور',password:'كلمة المرور',profile:'الملف التعريفي',uptime:'الوقت',note:'العبارة الإضافية'};
function chunk<T>(items:T[],size:number):T[][]{
 const result:T[][]=[];
 for(let i=0;i<items.length;i+=size)result.push(items.slice(i,i+size));
 return result;
}
function VoucherCard({language,voucher,title,note,accent,background,elements,cardWidthMM,cardHeightMM}:{language:Language;voucher:Voucher;title:string;note:string;accent:string;background:string;elements:CardTemplateElement[];cardWidthMM:number;cardHeightMM:number}){
 const tx=(text:string)=>t(language,text);
 const values:Record<CardTemplateElement['field'],string>={title:title||'HotSpot',usernameLabel:tx('اسم المستخدم / رمز الدخول'),username:voucher.username,passwordLabel:tx('كلمة المرور'),password:voucher.password||'',profile:voucher.profileName,uptime:voucher.limitUptime?tx('الوقت: ')+voucher.limitUptime:'',note};
 const style={'--voucher-accent':accent,aspectRatio:cardWidthMM+'/'+cardHeightMM,backgroundImage:background?'url('+background+')':undefined} as CSSProperties;
 return <article className="voucher-card voucher-card-canvas" style={style}>{elements.filter(element=>element.visible!==false&&values[element.field]!=='').map(element=><span key={element.id} className={'voucher-canvas-element voucher-field-'+element.field} dir={element.field==='username'||element.field==='password'?'ltr':undefined} style={{left:element.x+'%',top:element.y+'%',width:element.width+'%',fontSize:element.fontSize+'pt',color:element.color,textAlign:element.align,fontWeight:element.bold?800:400}}>{values[element.field]}</span>)}</article>;
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
 const [templateSnapshotMissing,setTemplateSnapshotMissing]=useState(false);
 const [batches,setBatches]=useState<VoucherBatch[]>([]);
 const [selected,setSelected]=useState<VoucherBatchResult|null>(null);
 const [layoutKey,setLayoutKey]=useState<LayoutKey>('4x15');
 const [customColumns,setCustomColumns]=useState(4);
 const [customRows,setCustomRows]=useState(15);
 const [cardTitle,setCardTitle]=useState(businessName||'HotSpot');
 const [note,setNote]=useState('');
 const [accent,setAccent]=useState('#147f9f');
 const [background,setBackground]=useState('');
 const [backgroundName,setBackgroundName]=useState('');
 const [backgroundAssetId,setBackgroundAssetId]=useState(0);
 const [designElements,setDesignElements]=useState<CardTemplateElement[]>(defaultDesign);
 const [selectedElementID,setSelectedElementID]=useState('username');
 const [cardWidthMM,setCardWidthMM]=useState(50);
 const [cardHeightMM,setCardHeightMM]=useState(19.1);
 const [templateName,setTemplateName]=useState('القالب الافتراضي');
 const [templateDefault,setTemplateDefault]=useState(true);
 const [templatePointOfSaleID,setTemplatePointOfSaleID]=useState('');
 const [templates,setTemplates]=useState<CardTemplate[]>([]);
 const [selectedTemplateID,setSelectedTemplateID]=useState('');
 const [savingTemplate,setSavingTemplate]=useState(false);
 const [templateLoading,setTemplateLoading]=useState(false);
 const [pageMarginMM,setPageMarginMM]=useState(5);
 const [pageGapMM,setPageGapMM]=useState(0);

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

 const layout=layoutKey==='custom'?{label:`${customColumns} × ${customRows}`,columns:customColumns,rows:customRows,capacity:customColumns*customRows}:layouts[layoutKey];
 const selectedPackage=cardPackages.find(item=>item.id===Number(packageID))??null;
 useEffect(()=>{
  let active=true;
  if(!selectedPackage){setTemplates([]);setSelectedTemplateID('');return ()=>{active=false}};
  setTemplateLoading(true);
  void Promise.all([api.cardTemplates(selectedPackage.id),api.designAssets(),pointOfSaleID?api.pointOfSalePackages(Number(pointOfSaleID),false):Promise.resolve(null)]).then(([result,assetsResult,posPackagesResult])=>{
   if(!active)return;
   setTemplates(result.templates);
   const pointPackageRule=posPackagesResult?.packages.find(rule=>rule.packageId===selectedPackage.id&&rule.isEnabled);
   const overriddenTemplate=pointPackageRule?.templateIdOverride?result.templates.find(template=>template.id===pointPackageRule.templateIdOverride&&template.active&&template.scopeKind==='point_of_sale'&&template.pointOfSaleId===Number(pointOfSaleID)):undefined;
   const pointTemplate=pointOfSaleID?result.templates.find(template=>template.active&&template.scopeKind==='point_of_sale'&&template.pointOfSaleId===Number(pointOfSaleID)):undefined;
   const item=overriddenTemplate||pointTemplate||result.templates.find(template=>template.active&&template.isDefault&&template.scopeKind==='global')||result.templates.find(template=>template.active&&template.scopeKind==='global');
   if(!item){setSelectedTemplateID('');setTemplateName('القالب الافتراضي');setTemplateDefault(true);setTemplatePointOfSaleID('');setDesignElements(defaultDesign);setCardWidthMM(50);setCardHeightMM(19.1);setPageMarginMM(5);setPageGapMM(0);setBackgroundAssetId(0);setBackground('');setBackgroundName('');setLayoutKey('4x15');return}
   setSelectedTemplateID(String(item.id));setTemplateName(item.name);setTemplateDefault(item.isDefault);setTemplatePointOfSaleID(item.pointOfSaleId?String(item.pointOfSaleId):'');
   setCardWidthMM(item.cardWidthMm);setCardHeightMM(item.cardHeightMm);
   setDesignElements(Array.isArray(item.design)&&item.design.length?item.design:defaultDesign);
   const layoutKeyFound=(Object.keys(layouts) as Exclude<LayoutKey,'custom'>[]).find(key=>layouts[key].columns===item.pageLayout?.columns&&layouts[key].rows===item.pageLayout?.rows);
   const cols=Math.max(1,Math.min(10,item.pageLayout?.columns??4)),rws=Math.max(1,Math.min(40,item.pageLayout?.rows??15));
   setCustomColumns(cols);setCustomRows(rws);setLayoutKey(layoutKeyFound||'custom');setPageMarginMM(item.pageLayout?.marginMm??5);setPageGapMM(item.pageLayout?.gapMm??0);
   setBackgroundAssetId(item.backgroundAssetId||0);
   const asset=assetsResult.assets.find(assetItem=>assetItem.id===item.backgroundAssetId);
   setBackground(asset?'/api/design-assets/'+asset.assetKey:'');setBackgroundName(asset?.originalName||'');
  }).catch(error=>{if(active)onMessage('danger',(error as Error).message)}).finally(()=>{if(active)setTemplateLoading(false)});
  return ()=>{active=false};
 },[selectedPackage?.id,pointOfSaleID]);
 const vouchers=selected?.vouchers??[];
 const sheets=useMemo(()=>chunk(vouchers,layout.capacity),[vouchers,layout.capacity]);
 const sampleVoucher:Voucher={id:0,batchId:0,username:'BT-12345678',password:passwordMode==='none'?'':'1234',profileName:selectedPackage?.serviceProfileName||profile||'Profile',serverName:'all',limitUptime:limitUptime,status:'available',createdAt:''};
 const screenPreview=vouchers.length?vouchers.slice(0,6):[sampleVoucher];
 const layoutFits=layout.columns*cardWidthMM+(layout.columns-1)*pageGapMM<=210-2*pageMarginMM+0.01&&layout.rows*cardHeightMM+(layout.rows-1)*pageGapMM<=297-2*pageMarginMM+0.01;

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
      ...(selectedTemplateID?{templateId:Number(selectedTemplateID)}:{}),
      allowWithoutTemplate,...generationOptions
     })
    :await api.generateVouchers({router,profile,...generationOptions});
   if(result.templateSnapshot)applyTemplateSnapshot(result.templateSnapshot);
   setTemplateSnapshotMissing(Boolean(result.batch.templateVersionId&&!result.templateSnapshot));
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

 function applyTemplateSnapshot(item:CardTemplate){
  setSelectedTemplateID(String(item.id));
  setTemplateName(item.name);
  setTemplateDefault(item.isDefault);
  setTemplatePointOfSaleID(item.pointOfSaleId?String(item.pointOfSaleId):'');
  setCardWidthMM(item.cardWidthMm);
  setCardHeightMM(item.cardHeightMm);
  setDesignElements(Array.isArray(item.design)&&item.design.length?item.design:defaultDesign);
  const match=(Object.keys(layouts) as Exclude<LayoutKey,'custom'>[]).find(key=>layouts[key].columns===item.pageLayout?.columns&&layouts[key].rows===item.pageLayout?.rows);
  const cols=Math.max(1,Math.min(10,item.pageLayout?.columns??4));
  const rows=Math.max(1,Math.min(40,item.pageLayout?.rows??15));
  setCustomColumns(cols);setCustomRows(rows);setLayoutKey(match||'custom');
  setPageMarginMM(item.pageLayout?.marginMm??5);setPageGapMM(item.pageLayout?.gapMm??0);
  setBackgroundAssetId(item.backgroundAssetId||0);
  void api.designAssets().then(result=>{const asset=result.assets.find(entry=>entry.id===item.backgroundAssetId);setBackground(asset?'/api/design-assets/'+asset.assetKey:'');setBackgroundName(asset?.originalName||'')}).catch(error=>onMessage('danger',(error as Error).message));
 }

async function chooseBatch(value:string){
  const id=Number(value);
  if(!id)return;
  setLoadingBatch(true);
  try{
   const result=await api.voucherBatch(id);
   if(result.templateSnapshot)applyTemplateSnapshot(result.templateSnapshot);
   const snapshotMissing=Boolean(result.batch.templateVersionId&&!result.templateSnapshot);
   setTemplateSnapshotMissing(snapshotMissing);
   if(snapshotMissing)onMessage('danger',tx('تعذر استعادة إصدار قالب الطباعة التاريخي لهذه الدفعة؛ تم تعطيل الطباعة لتجنب استخدام تصميم مختلف.'));
   setSelected(result);
   if(result.errorMessage)onMessage('danger',result.errorMessage);
  }catch(error){onMessage('danger',(error as Error).message)}
  finally{setLoadingBatch(false)}
 }

 async function saveTemplate(){
  if(!selectedPackage){onMessage('danger',tx('اختر باقة كروت أولًا لحفظ القالب.'));return}
  setSavingTemplate(true);
  try{
   const result=await api.saveCardTemplate({...(selectedTemplateID?{id:Number(selectedTemplateID)}:{}),packageId:selectedPackage.id,...(templatePointOfSaleID?{pointOfSaleId:Number(templatePointOfSaleID)}:{}),name:templateName.trim()||'قالب الكروت',isDefault:templateDefault,cardWidthMm:cardWidthMM,cardHeightMm:cardHeightMM,design:designElements,pageLayout:{columns:layout.columns,rows:layout.rows,marginMm:pageMarginMM,gapMm:pageGapMM},...(backgroundAssetId?{backgroundAssetId}:{})});
   setTemplates(old=>[result.template,...old.filter(item=>item.id!==result.template.id)]);setSelectedTemplateID(String(result.template.id));
   onMessage('success',tx('تم حفظ إصدار جديد من قالب التصميم.'));
  }catch(error){onMessage('danger',(error as Error).message||tx('تعذر حفظ القالب.'))}
  finally{setSavingTemplate(false)}
 }
 function updateElement(id:string,patch:Partial<CardTemplateElement>){setDesignElements(old=>old.map(element=>element.id===id?{...element,...patch}:element))}
 const selectedElement=designElements.find(element=>element.id===selectedElementID)||designElements[0];
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
   setBackgroundAssetId(result.asset.id);
   onMessage('success',tx('تم حفظ ملف الخلفية؛ احفظ القالب للاحتفاظ باختياره فيه.'));
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

  {(selected||selectedPackage)&&<div className="voucher-print-settings">
   <div className="hotspot-section-title mt-4"><h3>{tx("استوديو تصميم الكروت")}</h3><span>{vouchers.length} {tx('كرت متاح')}</span></div>
   <div className="row g-3">
    <div className="col-md-6"><label className="form-label" htmlFor="cardTitle">{tx("اسم الشبكة على الكرت")}</label><input id="cardTitle" className="form-control" value={cardTitle} onChange={event=>setCardTitle(event.target.value)} maxLength={80}/></div>
    <div className="col-md-6"><label className="form-label" htmlFor="cardNote">{tx("عبارة إضافية (اختيارية)")}</label><input id="cardNote" className="form-control" value={note} onChange={event=>setNote(event.target.value)} maxLength={80} placeholder={tx("للدعم أو التواصل...")}/></div>
    <div className="col-md-6"><label className="form-label" htmlFor="cardLayout">{tx("تخطيط ورقة A4")}</label><select id="cardLayout" className="form-select form-control" value={layoutKey} onChange={event=>setLayoutKey(event.target.value as LayoutKey)}>{Object.entries(layouts).map(([key,value])=><option key={key} value={key}>{tx(value.label)}</option>)}<option value="custom">{tx('تخطيط مخصص','Custom layout')} — {customColumns} × {customRows}</option></select></div>
    <div className="col-md-3"><label className="form-label" htmlFor="cardAccent">{tx("لون الهوية")}</label><input id="cardAccent" className="form-control form-control-color" type="color" value={accent} onChange={event=>setAccent(event.target.value)} title={tx("لون الهوية")}/></div>
    <div className="col-md-3"><label className="form-label" htmlFor="cardBackground">{tx("خلفية مخصصة")}</label><input id="cardBackground" className="form-control" type="file" accept="image/png,image/jpeg,image/webp" onChange={handleBackground}/>{backgroundName&&<div className="form-hint">{backgroundName} <button type="button" className="btn btn-sm btn-link p-0" onClick={()=>{setBackground('');setBackgroundName('');setBackgroundAssetId(0)}}>{tx("إزالة")}</button></div>}</div>
   </div>
   <div className="voucher-design-workbench mt-4 no-print">
    <div className="hotspot-section-title"><h4>{tx('محرر عناصر الكرت')}</h4><span>{tx('إعدادات القالب')}</span></div>
    <div className="row g-3 mt-1">
     <div className="col-md-6"><label className="form-label" htmlFor="templateLoad">{tx('القوالب المحفوظة')}</label><select id="templateLoad" className="form-select" value={selectedTemplateID} disabled={templateLoading} onChange={event=>{const item=templates.find(template=>template.id===Number(event.target.value));if(!item){setSelectedTemplateID('');setTemplateName('قالب جديد');setDesignElements(defaultDesign);return}setSelectedTemplateID(String(item.id));setTemplateName(item.name);setTemplateDefault(item.isDefault);setTemplatePointOfSaleID(item.pointOfSaleId?String(item.pointOfSaleId):'');setCardWidthMM(item.cardWidthMm);setCardHeightMM(item.cardHeightMm);setDesignElements(Array.isArray(item.design)&&item.design.length?item.design:defaultDesign);const matchingLayout=(Object.keys(layouts) as Exclude<LayoutKey,'custom'>[]).find(key=>layouts[key].columns===item.pageLayout?.columns&&layouts[key].rows===item.pageLayout?.rows);const cols=Math.max(1,Math.min(10,item.pageLayout?.columns??4));const rows=Math.max(1,Math.min(40,item.pageLayout?.rows??15));setCustomColumns(cols);setCustomRows(rows);setLayoutKey(matchingLayout||'custom');setPageMarginMM(item.pageLayout?.marginMm??5);setPageGapMM(item.pageLayout?.gapMm??0);setBackgroundAssetId(item.backgroundAssetId||0);void api.designAssets().then(result=>{const asset=result.assets.find(a=>a.id===item.backgroundAssetId);setBackground(asset?'/api/design-assets/'+asset.assetKey:'');setBackgroundName(asset?.originalName||'')}).catch(error=>onMessage('danger',(error as Error).message))}}><option value="">{tx('قالب جديد')}</option>{templates.map(item=><option key={item.id} value={item.id}>{item.name}{item.isDefault?' — '+tx('افتراضي'):''} · v{item.versionNumber}</option>)}</select></div>
     <div className="col-md-6"><label className="form-label" htmlFor="templateScope">{tx('نطاق القالب')}</label><select id="templateScope" className="form-select" value={templatePointOfSaleID} onChange={event=>{setTemplatePointOfSaleID(event.target.value);setSelectedTemplateID('');if(event.target.value)setTemplateDefault(false)}}><option value="">{tx('قالب عام للباقة')}</option>{pointsOfSale.map(point=><option key={point.id} value={point.id}>{tx('قالب خاص بنقطة البيع')}: {point.name}</option>)}</select></div>
     <div className="col-md-6"><label className="form-label" htmlFor="templateName">{tx('اسم القالب')}</label><input id="templateName" className="form-control" value={templateName} onChange={event=>setTemplateName(event.target.value)} maxLength={100}/></div>
     <div className="col-md-3"><label className="form-label" htmlFor="cardWidthMm">{tx('عرض الكرت (مم)')}</label><input id="cardWidthMm" type="number" className="form-control" min={10} max={300} step=".1" value={cardWidthMM} onChange={event=>setCardWidthMM(Math.max(10,Math.min(300,Number(event.target.value)||50)))}/></div>
     <div className="col-md-3"><label className="form-label" htmlFor="cardHeightMm">{tx('ارتفاع الكرت (مم)')}</label><input id="cardHeightMm" type="number" className="form-control" min={10} max={300} step=".1" value={cardHeightMM} onChange={event=>setCardHeightMM(Math.max(10,Math.min(300,Number(event.target.value)||19.1)))}/></div>
     <div className="col-md-3"><label className="form-label" htmlFor="pageMarginMm">{tx('هامش الورقة (مم)')}</label><input id="pageMarginMm" type="number" className="form-control" min={0} max={20} step=".5" value={pageMarginMM} onChange={event=>setPageMarginMM(Math.max(0,Math.min(20,Number(event.target.value)||0)))}/></div>
     <div className="col-md-3"><label className="form-label" htmlFor="pageGapMm">{tx('المسافة بين الكروت (مم)')}</label><input id="pageGapMm" type="number" className="form-control" min={0} max={10} step=".5" value={pageGapMM} onChange={event=>setPageGapMM(Math.max(0,Math.min(10,Number(event.target.value)||0)))}/></div>
     <div className="col-md-4"><label className="form-label" htmlFor="designElementSelect">{tx('العنصر المراد تعديله')}</label><select id="designElementSelect" className="form-select" value={selectedElement?.id||''} onChange={event=>setSelectedElementID(event.target.value)}>{designElements.map(element=><option key={element.id} value={element.id}>{tx(elementLabels[element.field])}</option>)}</select></div>
     {selectedElement&&<><div className="col-md-2"><label className="form-label" htmlFor="elementX">{tx('الموضع الأفقي %')}</label><input id="elementX" type="number" min={0} max={100} className="form-control" value={selectedElement.x} onChange={event=>updateElement(selectedElement.id,{x:Math.max(0,Math.min(100,Number(event.target.value)||0))})}/></div><div className="col-md-2"><label className="form-label" htmlFor="elementY">{tx('الموضع الرأسي %')}</label><input id="elementY" type="number" min={0} max={100} className="form-control" value={selectedElement.y} onChange={event=>updateElement(selectedElement.id,{y:Math.max(0,Math.min(100,Number(event.target.value)||0))})}/></div><div className="col-md-2"><label className="form-label" htmlFor="elementWidth">{tx('العرض %')}</label><input id="elementWidth" type="number" min={5} max={100} className="form-control" value={selectedElement.width} onChange={event=>updateElement(selectedElement.id,{width:Math.max(5,Math.min(100,Number(event.target.value)||5))})}/></div><div className="col-md-2"><label className="form-label" htmlFor="elementSize">{tx('حجم الخط')}</label><input id="elementSize" type="number" min={4} max={36} className="form-control" value={selectedElement.fontSize} onChange={event=>updateElement(selectedElement.id,{fontSize:Math.max(4,Math.min(36,Number(event.target.value)||4))})}/></div><div className="col-md-3"><label className="form-label" htmlFor="elementColor">{tx('لون النص')}</label><input id="elementColor" type="color" className="form-control form-control-color" value={selectedElement.color} onChange={event=>updateElement(selectedElement.id,{color:event.target.value})}/></div><div className="col-md-3"><label className="form-label" htmlFor="elementAlign">{tx('محاذاة النص')}</label><select id="elementAlign" className="form-select" value={selectedElement.align} onChange={event=>updateElement(selectedElement.id,{align:event.target.value as CardTemplateElement['align']})}><option value="right">{tx('يمين')}</option><option value="center">{tx('وسط')}</option><option value="left">{tx('يسار')}</option></select></div><div className="col-md-3 d-flex align-items-end"><label className="form-check mb-2"><input type="checkbox" className="form-check-input" checked={selectedElement.bold} onChange={event=>updateElement(selectedElement.id,{bold:event.target.checked})}/><span className="form-check-label">{tx('خط عريض')}</span></label></div><div className="col-md-3 d-flex align-items-end"><label className="form-check mb-2"><input type="checkbox" className="form-check-input" checked={selectedElement.visible!==false} onChange={event=>updateElement(selectedElement.id,{visible:event.target.checked})}/><span className="form-check-label">{tx('إظهار العنصر')}</span></label></div></>}
     <div className="col-md-3 d-flex align-items-end"><label className="form-check mb-2"><input type="checkbox" className="form-check-input" checked={templateDefault} onChange={event=>setTemplateDefault(event.target.checked)}/><span className="form-check-label">{tx('جعله القالب الافتراضي للباقة')}</span></label></div>
    </div>
    <div className="d-flex flex-wrap gap-2 mt-3"><button type="button" className="btn btn-primary" disabled={!selectedPackage||savingTemplate||templateLoading} onClick={()=>void saveTemplate()}>{savingTemplate?tx('جارٍ الحفظ...'):tx('حفظ إصدار القالب')}</button><button type="button" className="btn btn-outline-secondary" onClick={()=>{setSelectedTemplateID('');setTemplateName('قالب جديد');setTemplateDefault(false);setTemplatePointOfSaleID('');setDesignElements(defaultDesign);setBackground('');setBackgroundName('');setBackgroundAssetId(0)}}>{tx('بدء قالب جديد')}</button><span className="form-hint align-self-center">{tx('كل عملية حفظ تنشئ إصدارًا جديدًا، وتبقى الإصدارات السابقة محفوظة.')}</span></div>
   </div>
   {!layoutFits&&<div className="alert alert-warning mt-3 no-print">{tx('مقاسات الكرت والهوامش والمسافات تتجاوز المساحة القابلة للطباعة على ورقة A4. عدّل المقاسات أو تخطيط الصفحات قبل الطباعة.')}</div>}
   <div className="d-flex flex-wrap align-items-center gap-2 mt-3 no-print"><button type="button" className="btn btn-primary" disabled={!vouchers.length||!layoutFits||templateSnapshotMissing} onClick={()=>window.print()}><i className="bi bi-printer ms-1"/> {tx("طباعة / حفظ PDF")}</button><span className="small text-secondary">{tx("اختر «Save as PDF / حفظ كملف PDF» من نافذة الطباعة.")}</span></div>
   {selected&&(selected.batch.failedCount>0||Boolean(selected.untrackedUsernames?.length))&&<div className="alert alert-warning mt-3">{tx('لم تكتمل الدفعة بالكامل.')} {formatT(language,'حُفظت محليًا {created} من أصل {requested} قسيمة.',{created:selected.batch.createdCount,requested:selected.batch.requestedCount})} {selected.errorMessage}{selected.untrackedUsernames&&selected.untrackedUsernames.length>0&&<div className="mt-2"><b>{tx("أسماء أنشئت على الراوتر ولم تُحفظ محليًا:")}</b><div dir="ltr">{selected.untrackedUsernames.join(", ")}</div><small>{tx("لا تطبع هذه الرموز من الاستوديو قبل تسجيلها محليًا؛ فهي غير موجودة ضمن القسائم المحفوظة.")}</small></div>}</div>}
   <div className="voucher-preview-wrap mt-3"><div className="small text-secondary mb-2">{vouchers.length?formatT(language,'معاينة لأول {count} كروت — المعاينة للطباعة كاملة على ورق A4.',{count:screenPreview.length}):tx('معاينة تجريبية للقالب؛ أنشئ دفعة لعرض بيانات الدخول الفعلية.')}</div><div className="voucher-preview-grid">{screenPreview.map(voucher=><VoucherCard key={voucher.id} language={language} voucher={voucher} title={cardTitle} note={note} accent={accent} background={background} elements={designElements} cardWidthMM={cardWidthMM} cardHeightMM={cardHeightMM}/>)}</div></div>
   {!vouchers.length&&<div className="small text-secondary mt-3">{tx("لا توجد كروت ناجحة في هذه الدفعة للطباعة؛ المعاينة أعلاه تجريبية.")}</div>}
  </div>}

  {vouchers.length>0&&<div className="voucher-print-root" id="voucherPrintRoot">
   {sheets.map((sheet,index)=><div className={"voucher-sheet voucher-sheet-"+layoutKey} key={index} style={{'--voucher-columns':layout.columns,'--voucher-rows':layout.rows,'--voucher-accent':accent,'--voucher-sheet-width':(210-pageMarginMM*2)+'mm','--voucher-sheet-height':(297-pageMarginMM*2)+'mm','--voucher-gap':pageGapMM+'mm','--voucher-card-width':cardWidthMM+'mm','--voucher-card-height':cardHeightMM+'mm'} as CSSProperties}>
    {sheet.map(voucher=><VoucherCard key={voucher.id} language={language} voucher={voucher} title={cardTitle} note={note} accent={accent} background={background} elements={designElements} cardWidthMM={cardWidthMM} cardHeightMM={cardHeightMM}/>)}
   </div>)}
  </div>}
 </section>;
}
