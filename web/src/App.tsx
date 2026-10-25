import {useEffect,useState,type FormEvent} from 'react';
import {api} from './api';
import HotSpotManager from './components/HotSpotManager';
import UserManagerManager from './components/UserManagerManager';
import CardPackageManager from './components/CardPackageManager';
import {formatT,t,type Language} from './i18n';
import {setBootstrapDirection} from './bootstrapStyles';
import type {AppSettings,HealthResponse,HotSpotOverview,RouterInfo,UserManagerOverview} from './types';

const defaults:AppSettings={businessName:'bt-manager',currencyCode:'YER',currencySymbol:'ر.ي',setupComplete:false};
type Flash={kind:'success'|'danger';message:string}|null;
type Page='dashboard'|'settings'|'hotspot-users'|'hotspot-profiles'|'hotspot-sessions'|'hotspot-servers'|'vouchers'|'pos'|'user-manager'|'card-packages';
type HotSpotView='users'|'profiles'|'sessions'|'servers'|'vouchers'|'pos';
const pageInfo:Record<Page,{title:string;eyebrow:string;description:string}>={
 dashboard:{title:'نظرة عامة',eyebrow:'لوحة التحكم',description:'ملخص حالة التطبيق والشبكة وأهم وحدات العمل.'},
 settings:{title:'الإعدادات والاتصال',eyebrow:'تهيئة النظام',description:'إعداد بيانات النشاط والعملة والاتصال براوتر MikroTik.'},
 'hotspot-users':{title:'مستخدمو HotSpot',eyebrow:'إدارة HotSpot',description:'إنشاء حسابات الدخول وتعديلها وتفعيلها أو تعطيلها.'},
 'hotspot-profiles':{title:'الملفات التعريفية',eyebrow:'إدارة HotSpot',description:'إدارة ملفات السرعة وحدود الجلسة والأجهزة المتزامنة.'},
 'hotspot-sessions':{title:'الجلسات النشطة',eyebrow:'مراقبة الشبكة',description:'عرض المستخدمين المتصلين وعناوينهم ومدد اتصالهم.'},
 'hotspot-servers':{title:'خوادم HotSpot',eyebrow:'إدارة HotSpot',description:'عرض خوادم HotSpot وواجهاتها وملفاتها التعريفية.'},
 vouchers:{title:'دفعات الكروت والطباعة',eyebrow:'قسائم HotSpot',description:'إنشاء دفعات الدخول وضبط الرموز وتصميم الكروت وطباعتها.'},
 pos:{title:'نقطة البيع',eyebrow:'المبيعات',description:'تعريف الباقات وبيع القسائم المتوفرة ومراجعة الإيصالات.'},
 'user-manager':{title:'User Manager',eyebrow:'إدارة RADIUS',description:'استيراد Profiles وLimitations وعرض القيود المرتبطة بها حسب إصدار RouterOS.'},
 'card-packages':{title:'دليل باقات الكروت',eyebrow:'كتالوج الباقات',description:'ربط الباقات التجارية بملفات HotSpot أو User Manager، وتحديد أسعار المستخدم والموزع.'}
};
const navGroups:{label:string;items:{page:Page;label:string;icon:string;soon?:boolean}[]}[]=[
 {label:'عام',items:[{page:'dashboard',label:'نظرة عامة',icon:'bi-grid-1x2-fill'},{page:'settings',label:'الإعدادات والاتصال',icon:'bi-router'}]},
 {label:'إدارة HotSpot',items:[
  {page:'hotspot-users',label:'المستخدمون',icon:'bi-people'},
  {page:'hotspot-profiles',label:'الملفات التعريفية',icon:'bi-speedometer2'},
  {page:'hotspot-sessions',label:'الجلسات النشطة',icon:'bi-activity'},
  {page:'hotspot-servers',label:'خوادم HotSpot',icon:'bi-wifi'},
  {page:'vouchers',label:'دفعات الكروت والطباعة',icon:'bi-ticket-perforated'}
 ]},
 {label:'البيع وإدارة المستخدمين',items:[
  {page:'pos',label:'نقطة البيع',icon:'bi-shop'},
  {page:'card-packages',label:'دليل باقات الكروت',icon:'bi-collection'},
  {page:'user-manager',label:'User Manager',icon:'bi-person-gear'}
 ]}
];
const hotSpotViewByPage:Partial<Record<Page,HotSpotView>>={
 'hotspot-users':'users','hotspot-profiles':'profiles','hotspot-sessions':'sessions',
 'hotspot-servers':'servers',vouchers:'vouchers',pos:'pos'
};
const isHotSpotPage=(page:Page)=>page in hotSpotViewByPage;

export default function App(){
 const [health,setHealth]=useState<HealthResponse|null>(null);
 const [settings,setSettings]=useState<AppSettings>(defaults);
 const [router,setRouter]=useState({address:'192.168.88.1',port:8728,username:'admin',password:'',useTls:false});
 const [info,setInfo]=useState<RouterInfo|null>(null);
 const [hotspot,setHotspot]=useState<HotSpotOverview|null>(null);
 const [userManagerOverview,setUserManagerOverview]=useState<UserManagerOverview|null>(null);
 const [routerCatalogId,setRouterCatalogId]=useState<number|null>(null);
 const [page,setPage]=useState<Page>('dashboard');
 const [language,setLanguage]=useState<Language>(()=>{try{return localStorage.getItem('bt-manager-language')==='en'?'en':'ar'}catch{return 'ar'}});
 const [theme,setTheme]=useState<'light'|'dark'>(()=>{try{return localStorage.getItem('bt-manager-theme')==='dark'?'dark':'light'}catch{return 'light'}});
 const [fullscreen,setFullscreen]=useState(Boolean(document.fullscreenElement));
 const [testing,setTesting]=useState(false);
 const [loadingHotspot,setLoadingHotspot]=useState(false);
 const [loadingUserManager,setLoadingUserManager]=useState(false);
 const [saving,setSaving]=useState(false);
 const [flash,setFlash]=useState<Flash>(null);

 useEffect(()=>{
  document.documentElement.lang=language;
  document.title=language==='ar'?'bt-manager | إدارة الشبكة':'bt-manager | Network Manager';
  document.documentElement.dir=language==='ar'?'rtl':'ltr';
  document.documentElement.dataset.theme=theme;
  document.body.dataset.theme=theme;
  setBootstrapDirection(language==='ar'?'rtl':'ltr');
  try{localStorage.setItem('bt-manager-language',language);localStorage.setItem('bt-manager-theme',theme)}catch{/* localStorage can be disabled */}
 },[language,theme]);

 useEffect(()=>{
  const updateFullscreen=()=>setFullscreen(Boolean(document.fullscreenElement));
  document.addEventListener('fullscreenchange',updateFullscreen);
  return()=>document.removeEventListener('fullscreenchange',updateFullscreen);
 },[]);

 useEffect(()=>{
  void Promise.all([api.health(),api.settings()])
   .then(([h,s])=>{setHealth(h);setSettings(s)})
   .catch((e:Error)=>setFlash({kind:'danger',message:e.message}));
 },[]);

 async function toggleFullscreen(){
  try{
   if(document.fullscreenElement)await document.exitFullscreen();
   else await document.documentElement.requestFullscreen();
  }catch{
   setFlash({kind:'danger',message:t(language,'تعذر تفعيل وضع ملء الشاشة.')});
  }
 }
 async function captureScreenshot(){
  if(!navigator.mediaDevices?.getDisplayMedia){
   setFlash({kind:'danger',message:t(language,'متصفحك لا يدعم التقاط الشاشة من داخل الصفحة.')});return;
  }
  let stream:MediaStream|undefined;
  try{
   stream=await navigator.mediaDevices.getDisplayMedia({video:true,audio:false});
   const video=document.createElement('video');
   video.muted=true;video.srcObject=stream;
   await video.play();
   if(video.readyState<2)await new Promise<void>((resolve,reject)=>{
    video.onloadeddata=()=>resolve();video.onerror=()=>reject(new Error('video capture failed'));
   });
   const canvas=document.createElement('canvas');
   canvas.width=video.videoWidth;canvas.height=video.videoHeight;
   const context=canvas.getContext('2d');
   if(!context)throw new Error('canvas unavailable');
   context.drawImage(video,0,0,canvas.width,canvas.height);
   const blob=await new Promise<Blob>((resolve,reject)=>canvas.toBlob(value=>value?resolve(value):reject(new Error('PNG encoding failed')),'image/png'));
   const url=URL.createObjectURL(blob);
   const anchor=document.createElement('a');
   anchor.href=url;anchor.download='bt-manager-'+new Date().toISOString().replace(/[:.]/g,'-')+'.png';
   document.body.appendChild(anchor);anchor.click();anchor.remove();
   window.setTimeout(()=>URL.revokeObjectURL(url),1000);
   setFlash({kind:'success',message:t(language,'تم حفظ لقطة الشاشة.')});
  }catch(error){
   const name=(error as Error)?.name;
   if(name!=='NotAllowedError'&&name!=='AbortError')setFlash({kind:'danger',message:t(language,'تعذر التقاط لقطة الشاشة. تحقق من إذن مشاركة الشاشة ثم أعد المحاولة.')});
  }finally{
   stream?.getTracks().forEach(track=>track.stop());
  }
 }
 async function save(event:FormEvent){
  event.preventDefault();setSaving(true);setFlash(null);
  try{setSettings(await api.saveSettings(settings));setFlash({kind:'success',message:t(language,'تم حفظ إعدادات النشاط والعملة.')})}
  catch(error){setFlash({kind:'danger',message:t(language,(error as Error).message)})}
  finally{setSaving(false)}
 }
 async function testRouter(event:FormEvent){
  event.preventDefault();setTesting(true);setFlash(null);setInfo(null);setRouterCatalogId(null);
  try{
   const result=await api.testRouter(router);setInfo(result.router);
   setFlash({kind:'success',message:formatT(language,'تم الاتصال بالراوتر {identity} بنجاح.',{identity:result.router.identity||router.address})});
  }catch(error){setFlash({kind:'danger',message:t(language,(error as Error).message)})}
  finally{setTesting(false)}
 }
 async function loadHotspot(showMessage=true){
  setLoadingHotspot(true);if(showMessage)setFlash(null);
  try{
   const result=await api.hotspotOverview(router);setHotspot(result);
   let profileCount = 0;
   try{
    const profileSync=await api.syncHotSpotProfiles(router);
    profileCount=profileSync.sync.profileCount;
    setRouterCatalogId(profileSync.sync.routerId);
   }catch(error){
    setFlash({kind:'danger',message:formatT(language,'تم تحميل بيانات HotSpot، لكن تعذرت مزامنة ملفات التعريف إلى الكتالوج المحلي: {error}',{error:(error as Error).message})});
    return;
   }
   if(showMessage)setFlash({kind:'success',message:formatT(language,'تمت قراءة HotSpot ومزامنة {profiles} ملفًا تعريفيًا؛ {users} مستخدم و{sessions} جلسة نشطة.',{profiles:profileCount,users:result.counts.users,sessions:result.counts.activeSessions})});
  }catch(error){setFlash({kind:'danger',message:t(language,(error as Error).message)})}
  finally{setLoadingHotspot(false)}
 }
 async function loadUserManager(showMessage=true){
  if(!info){
   setFlash({kind:'danger',message:t(language,'اختبر اتصال الراوتر من «الإعدادات والاتصال» قبل استيراد User Manager.')});
   return;
  }
  setLoadingUserManager(true);if(showMessage)setFlash(null);
  try{
   const result=await api.syncUserManager(router);
   setUserManagerOverview(result);
   setRouterCatalogId(result.sync.routerId);
   if(showMessage)setFlash({kind:'success',message:formatT(language,'اكتملت مزامنة User Manager: {profiles} ملفًا و{limitations} قيدًا و{relations} علاقة.',{profiles:result.sync.profileCount,limitations:result.sync.limitationCount,relations:result.sync.relationshipCount})});
  }catch(error){setFlash({kind:'danger',message:t(language,(error as Error).message)})}
  finally{setLoadingUserManager(false)}
 }
 function navigate(nextPage:Page){
  setPage(nextPage);
  if(isHotSpotPage(nextPage)&&!loadingHotspot){
   if(info){void loadHotspot(false)}
   else if(!hotspot){setFlash({kind:'danger',message:t(language,'افتح «الإعدادات والاتصال» واختبر اتصال الراوتر أولًا، ثم اقرأ بيانات HotSpot.')})}
  }
  if(nextPage==='user-manager'&&!loadingUserManager){
   if(info){void loadUserManager(false)}
   else{setFlash({kind:'danger',message:t(language,'اختبر اتصال الراوتر من «الإعدادات والاتصال» قبل استيراد User Manager.')})}
  }
  if(nextPage==='card-packages'&&!routerCatalogId&&info&&!loadingHotspot){void loadHotspot(false)}
 }
 const setting=(key:keyof AppSettings,value:string|boolean)=>setSettings(current=>({...current,[key]:value}));
 const route=(key:keyof typeof router,value:string|number|boolean)=>setRouter(current=>({...current,[key]:value}));
 const current=pageInfo[page];
 const view=hotSpotViewByPage[page];

 return <div className="app-shell">
  <aside className="sidebar">
   <div className="brand"><div className="brand-mark"><i className="bi bi-router-fill"/></div><div><strong>bt-manager</strong><small>{t(language,'إدارة شبكتك بذكاء')}</small></div></div>
   <nav className="sidebar-nav" aria-label={t(language,'التنقل الرئيسي')}>
    {navGroups.map(group=><div className="nav-group" key={group.label}>
     <div className="sidebar-label">{t(language,group.label)}</div>
     {group.items.map(item=><button key={item.page} type="button" className={`nav-entry nav-entry-button ${page===item.page?'active':''}`} onClick={()=>navigate(item.page)} aria-current={page===item.page?'page':undefined}>
      <i className={`bi ${item.icon}`}/><span>{t(language,item.label)}</span>{item.soon&&<small className="soon">{t(language,'قيد التطوير')}</small>}
     </button>)}
    </div>)}
   </nav>
   <div className="sidebar-bottom"><div className="secure-note"><i className="bi bi-shield-lock"/><div><b>{t(language,'تشغيل محلي')}</b><small>{t(language,'الوصول عبر LAN غير مفعل')}</small></div></div><small className="version">{t(language,'نسخة تطويرية · 0.1.0-dev')}</small></div>
  </aside>

  <main className="main-content">
   <header className="topbar"><div><div className="eyebrow">{t(language,current.eyebrow)}</div><h1>{t(language,current.title)}</h1><p>{t(language,current.description)}</p></div><div className="topbar-tools">
    <button type="button" className="tool-button" onClick={()=>void toggleFullscreen()} title={t(language,fullscreen?'الخروج من الشاشة الكاملة':'شاشة كاملة')} aria-label={t(language,fullscreen?'الخروج من الشاشة الكاملة':'شاشة كاملة')}><i className={`bi ${fullscreen?'bi-fullscreen-exit':'bi-arrows-fullscreen'}`}/></button>
    <button type="button" className="tool-button" onClick={()=>void captureScreenshot()} title={t(language,'التقاط لقطة شاشة')} aria-label={t(language,'التقاط لقطة شاشة')}><i className="bi bi-camera"/></button>
    <button type="button" className="tool-button" onClick={()=>setTheme(value=>value==='dark'?'light':'dark')} title={t(language,theme==='dark'?'الوضع الفاتح':'الوضع الداكن')} aria-label={t(language,theme==='dark'?'الوضع الفاتح':'الوضع الداكن')}><i className={`bi ${theme==='dark'?'bi-sun':'bi-moon-stars'}`}/></button>
    <button type="button" className="tool-button language-button" onClick={()=>setLanguage(value=>value==='ar'?'en':'ar')} title={t(language,'تغيير اللغة')} aria-label={t(language,'تغيير اللغة')}><i className="bi bi-translate"/><span>{language==='ar'?'EN':'عربي'}</span></button>
    <div className="top-status"><span className={`status-led ${health?.status==='ok'?'online':''}`}/>{health?.status==='ok'?t(language,'التطبيق يعمل'):t(language,'جارٍ فحص الحالة')}</div>
   </div></header>
   {flash&&<div className={`alert alert-${flash.kind} d-flex align-items-center gap-2`} role="alert"><i className={flash.kind==='success'?'bi bi-check-circle':'bi bi-exclamation-triangle'}/><div>{flash.message}</div><button type="button" className="btn-close me-auto" onClick={()=>setFlash(null)} aria-label={t(language,'إغلاق')}/></div>}

   {page==='dashboard'&&<div className="dashboard-page">
    <section className="welcome-panel"><div className="welcome-copy"><span className="pill">{t(language,"مساحة عمل موحدة")}</span><h2>{t(language,"إدارة الشبكة من مكان واحد")}</h2><p>{t(language,"تنقل بين مستخدمي HotSpot ودفعات الكروت ونقطة البيع عبر صفحات مستقلة، مع الإبقاء على واجهة عربية واضحة.")}</p><div className="setup-steps"><span className={settings.setupComplete?'step-done':''}><i className={`bi ${settings.setupComplete?'bi-check2-circle':'bi-circle'}`}/> {t(language,'إعداد النشاط')}</span><span className={info?'step-done':''}><i className={`bi ${info?'bi-check2-circle':'bi-circle'}`}/> {t(language,'اختبار الراوتر')}</span><span className={hotspot?'step-done':''}><i className={`bi ${hotspot?'bi-check2-circle':'bi-circle'}`}/> {t(language,'قراءة HotSpot')}</span></div></div><div className="welcome-art"><div className="orbit orbit-one"/><div className="orbit orbit-two"/><i className="bi bi-router-fill"/><span className="signal signal-a"/><span className="signal signal-b"/><span className="signal signal-c"/></div></section>
    <div className="section-heading"><div><h2>{t(language,"حالة الشبكة")}</h2><p>{t(language,"تتحدث المؤشرات بعد الاتصال وقراءة بيانات الراوتر.")}</p></div><button className="btn btn-outline-primary" type="button" onClick={()=>navigate('settings')}>{t(language,'الإعدادات والاتصال')} <i className="bi bi-arrow-left ms-1"/></button></div>
    {hotspot?<div className="row g-3 dashboard-stat-grid">
     {[{label:'مستخدمو HotSpot',value:hotspot.counts.users,icon:'bi-people-fill',page:'hotspot-users' as Page},{label:'الملفات التعريفية',value:hotspot.counts.profiles,icon:'bi-speedometer2',page:'hotspot-profiles' as Page},{label:'الجلسات النشطة',value:hotspot.counts.activeSessions,icon:'bi-activity',page:'hotspot-sessions' as Page},{label:'خوادم HotSpot',value:hotspot.counts.servers,icon:'bi-wifi',page:'hotspot-servers' as Page}].map(stat=><div className="col-6 col-xl-3" key={stat.label}><button type="button" className="dashboard-stat w-100" onClick={()=>navigate(stat.page)}><span className="dashboard-stat-icon"><i className={`bi ${stat.icon}`}/></span><small>{t(language,stat.label)}</small><strong>{stat.value}</strong><span className="dashboard-stat-link">{t(language,'افتح القسم')} <i className="bi bi-arrow-left"/></span></button></div>)}
    </div>:<section className="panel dashboard-setup-card"><div className="panel-heading"><div className="panel-icon cyan"><i className="bi bi-router"/></div><div><h3>{t(language,"ابدأ بربط الراوتر")}</h3><p>{t(language,"اختبر الاتصال ثم اقرأ بيانات HotSpot لعرض إحصاءات الشبكة.")}</p></div></div><button type="button" className="btn btn-primary" onClick={()=>navigate('settings')}>{t(language,'تهيئة الاتصال')} <i className="bi bi-arrow-left ms-1"/></button></section>}
    <div className="row g-3 mt-1">
     <div className="col-lg-6"><button type="button" className="dashboard-shortcut w-100" onClick={()=>navigate('vouchers')}><i className="bi bi-ticket-perforated"/><span><strong>{t(language,"دفعات الكروت")}</strong><small>{t(language,"أنشئ رموز الدخول وحدد طولها ونوع محارفها ثم اطبعها.")}</small></span><i className="bi bi-arrow-left"/></button></div>
     <div className="col-lg-6"><button type="button" className="dashboard-shortcut w-100" onClick={()=>navigate('pos')}><i className="bi bi-shop"/><span><strong>{t(language,'نقطة البيع')}</strong><small>{t(language,"إدارة الباقات والمبيعات والإيصالات.")}</small></span><i className="bi bi-arrow-left"/></button></div>
    </div>
   </div>}

   {page==='settings'&&<div className="row g-4 settings-grid">
    <div className="col-xl-5"><section className="panel h-100"><div className="panel-heading"><div className="panel-icon violet"><i className="bi bi-sliders2"/></div><div><h3>{t(language,"بيانات النشاط والعملة")}</h3><p>{t(language,"عملة واحدة لجميع العمليات المالية")}</p></div></div>
     <form onSubmit={save} className="panel-form"><label className="form-label" htmlFor="businessName">{t(language,"اسم الشبكة أو النشاط")}</label><input id="businessName" className="form-control" maxLength={120} value={settings.businessName} onChange={event=>setting('businessName',event.target.value)} required/>
      <div className="row g-3"><div className="col-6"><label className="form-label" htmlFor="currencyCode">{t(language,"رمز العملة (ISO)")}</label><input id="currencyCode" className="form-control text-uppercase" maxLength={3} minLength={3} pattern="[A-Za-z]{3}" value={settings.currencyCode} onChange={event=>setting('currencyCode',event.target.value.toUpperCase())} required/><div className="form-hint">{t(language,"مثل YER أو SAR")}</div></div><div className="col-6"><label className="form-label" htmlFor="currencySymbol">{t(language,"الرمز الظاهر")}</label><input id="currencySymbol" className="form-control" maxLength={12} value={settings.currencySymbol} onChange={event=>setting('currencySymbol',event.target.value)} required/><div className="form-hint">{t(language,"مثل ر.ي")}</div></div></div>
      <div className="helper-box"><i className="bi bi-info-circle"/><span>{t(language,"تأكد من العملة قبل بدء البيع؛ لن يمكن تغييرها بعد تسجيل أول معاملة مالية.")}</span></div><button className="btn btn-primary w-100" disabled={saving}>{saving?t(language,'جارٍ الحفظ...'):t(language,'حفظ الإعدادات')}</button>
     </form></section></div>
    <div className="col-xl-7"><section className="panel h-100"><div className="panel-heading"><div className="panel-icon cyan"><i className="bi bi-router"/></div><div><h3>{t(language,"الاتصال براوتر MikroTik")}</h3><p>{t(language,"اختبار قراءة معلومات النظام عبر LAN")}</p></div><span className="api-chip">RouterOS API</span></div>
     <form onSubmit={testRouter} className="panel-form"><div className="row g-3"><div className="col-md-7"><label className="form-label" htmlFor="routerAddress">{t(language,"عنوان الراوتر")}</label><input id="routerAddress" className="form-control ltr-input" value={router.address} onChange={event=>route('address',event.target.value)} placeholder="192.168.88.1" autoComplete="off" required/></div><div className="col-md-5"><label className="form-label" htmlFor="routerPort">{t(language,"المنفذ")}</label><input id="routerPort" className="form-control ltr-input" type="number" min={1} max={65535} value={router.port} onChange={event=>route('port',Number(event.target.value))} required/></div><div className="col-md-6"><label className="form-label" htmlFor="routerUser">{t(language,"اسم المستخدم")}</label><input id="routerUser" className="form-control ltr-input" value={router.username} onChange={event=>route('username',event.target.value)} autoComplete="username" required/></div><div className="col-md-6"><label className="form-label" htmlFor="routerPassword">{t(language,"كلمة المرور")}</label><input id="routerPassword" className="form-control ltr-input" type="password" value={router.password} onChange={event=>route('password',event.target.value)} autoComplete="current-password" required/></div></div>
      <label className="tls-option"><input className="form-check-input" type="checkbox" checked={router.useTls} onChange={event=>{const enabled=event.target.checked;route('useTls',enabled);route('port',enabled&&router.port===8728?8729:!enabled&&router.port===8729?8728:router.port)}}/><span><b>{t(language,"استخدام API-SSL")}</b><small>{t(language,"يتطلب إعداد شهادة موثوقة على الراوتر.")}</small></span><i className="bi bi-shield-check"/></label><button type="submit" className="btn btn-outline-primary w-100" disabled={testing||loadingHotspot}>{testing?t(language,'جارٍ اختبار الاتصال...'):t(language,'اختبار اتصال الراوتر')}</button><button type="button" className="btn btn-primary w-100" disabled={testing||loadingHotspot} onClick={()=>void loadHotspot()}>{loadingHotspot?t(language,'جارٍ قراءة بيانات HotSpot...'):t(language,'قراءة بيانات HotSpot')}</button>
     </form>
     {info&&<div className="router-result"><div className="result-title"><i className="bi bi-check-circle-fill"/> {t(language,'معلومات الراوتر')}</div><div className="result-grid">{[['الهوية',info.identity],['RouterOS',info.version],['الطراز',info.model||info.boardName],['مدة التشغيل',info.uptime],['المعالج',info.cpu],['الذاكرة الحرة',info.freeMemory]].map(([key,value])=><div key={key}><small>{t(language,String(key))}</small><b>{value||'—'}</b></div>)}</div></div>}
    </section></div>
   </div>}

   {view&&<>{hotspot?<HotSpotManager language={language} view={view} router={router} overview={hotspot} businessName={settings.businessName} currencyCode={settings.currencyCode} currencySymbol={settings.currencySymbol} onRefresh={()=>loadHotspot(false)} onMessage={(kind,message)=>setFlash({kind,message})}/>:<section className="panel empty-router-state"><div className="panel-icon cyan"><i className="bi bi-router"/></div><h2>{t(language,"بيانات HotSpot غير محمّلة")}</h2><p>{t(language,"اختبر اتصال الراوتر من الإعدادات أولًا، ثم اقرأ بيانات HotSpot لفتح هذا القسم.")}</p><button type="button" className="btn btn-primary" onClick={()=>navigate('settings')}>{t(language,"فتح الإعدادات والاتصال")}</button></section>}</>}

   {page==='user-manager'&&<UserManagerManager language={language} overview={userManagerOverview} loading={loadingUserManager} onRefresh={()=>void loadUserManager()}/>}
   {page==='card-packages'&&<CardPackageManager language={language} routerId={routerCatalogId} currencyCode={settings.currencyCode} currencySymbol={settings.currencySymbol} onMessage={(kind,message)=>setFlash({kind,message})} onRefreshHotSpot={()=>void loadHotspot(false)} onRefreshUserManager={()=>void loadUserManager(false)}/>}

   <footer className="page-footer"><div><span className="footer-mark">bt</span><span>bt-manager <small>· إدارة محلية لشبكات MikroTik</small></span></div><span><i className="bi bi-shield-check"/> {language==='ar'?'يعمل على جهازك':'Runs locally on your device'}</span></footer>
  </main>
 </div>;
}
