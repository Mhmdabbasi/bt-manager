import {useEffect,useState,type FormEvent} from 'react';
import {api} from './api';
import type {AppSettings,HealthResponse,HotSpotOverview,RouterInfo} from './types';

const defaults:AppSettings={businessName:'bt-manager',currencyCode:'YER',currencySymbol:'ر.ي',setupComplete:false};
type Flash={kind:'success'|'danger';message:string}|null;

export default function App(){
 const [health,setHealth]=useState<HealthResponse|null>(null);
 const [settings,setSettings]=useState<AppSettings>(defaults);
 const [router,setRouter]=useState({address:'192.168.88.1',port:8728,username:'admin',password:'',useTls:false});
 const [info,setInfo]=useState<RouterInfo|null>(null);
 const [hotspot,setHotspot]=useState<HotSpotOverview|null>(null);
 const [testing,setTesting]=useState(false);const [loadingHotspot,setLoadingHotspot]=useState(false);const [saving,setSaving]=useState(false);const [flash,setFlash]=useState<Flash>(null);
 useEffect(()=>{void Promise.all([api.health(),api.settings()]).then(([h,s])=>{setHealth(h);setSettings(s)}).catch((e:Error)=>setFlash({kind:'danger',message:e.message}))},[]);
 async function save(e:FormEvent){e.preventDefault();setSaving(true);setFlash(null);try{setSettings(await api.saveSettings(settings));setFlash({kind:'success',message:'تم حفظ إعدادات النشاط والعملة.'})}catch(e){setFlash({kind:'danger',message:(e as Error).message})}finally{setSaving(false)}}
 async function testRouter(e:FormEvent){e.preventDefault();setTesting(true);setFlash(null);setInfo(null);try{const result=await api.testRouter(router);setInfo(result.router);setFlash({kind:'success',message:`تم الاتصال بالراوتر ${result.router.identity||router.address} بنجاح.`})}catch(e){setFlash({kind:'danger',message:(e as Error).message})}finally{setTesting(false)}}
 async function loadHotspot(){setLoadingHotspot(true);setFlash(null);setHotspot(null);try{const result=await api.hotspotOverview(router);setHotspot(result);setFlash({kind:'success',message:`تمت قراءة بيانات HotSpot: ${result.counts.users} مستخدم و${result.counts.activeSessions} جلسة نشطة.`})}catch(e){setFlash({kind:'danger',message:(e as Error).message})}finally{setLoadingHotspot(false)}}
 const setting=(key:keyof AppSettings,value:string|boolean)=>setSettings(s=>({...s,[key]:value}));
 const route=(key:keyof typeof router,value:string|number|boolean)=>setRouter(s=>({...s,[key]:value}));
 return <div className="app-shell">
  <aside className="sidebar"><div className="brand"><div className="brand-mark"><i className="bi bi-router-fill"/></div><div><strong>bt-manager</strong><small>إدارة شبكتك بذكاء</small></div></div><div className="sidebar-label">مساحة العمل</div>
   <a className="nav-entry active" href="#overview"><i className="bi bi-grid-1x2-fill"/> نظرة عامة</a>
   <a className="nav-entry" href="#router"><i className="bi bi-router"/> اتصال MikroTik</a>
   <a className="nav-entry disabled" href="#hotspot" onClick={e=>e.preventDefault()}><i className="bi bi-wifi"/> مستخدمو HotSpot <span className="soon">لاحقًا</span></a>
   <a className="nav-entry disabled" href="#cards" onClick={e=>e.preventDefault()}><i className="bi bi-ticket-perforated"/> كروت الخدش <span className="soon">لاحقًا</span></a>
   <a className="nav-entry disabled" href="#pos" onClick={e=>e.preventDefault()}><i className="bi bi-shop"/> نقاط البيع <span className="soon">لاحقًا</span></a>
   <div className="sidebar-bottom"><div className="secure-note"><i className="bi bi-shield-lock"/><div><b>تشغيل محلي</b><small>الوصول عبر LAN غير مفعل</small></div></div><small className="version">نسخة تأسيسية · 0.1.0-dev</small></div>
  </aside>
  <main className="main-content" id="overview">
   <header className="topbar"><div><div className="eyebrow">لوحة التحكم</div><h1>أهلًا بك في bt-manager</h1><p>إدارة الشبكة من مكان واحد، ببساطة ووضوح.</p></div><div className="top-status"><span className={`status-led ${health?.status==='ok'?'online':''}`}/>{health?.status==='ok'?'التطبيق يعمل':'جارٍ فحص الحالة'}</div></header>
   {flash&&<div className={`alert alert-${flash.kind} d-flex align-items-center gap-2`} role="alert"><i className={flash.kind==='success'?'bi bi-check-circle':'bi bi-exclamation-triangle'}/><div>{flash.message}</div><button className="btn-close me-auto" onClick={()=>setFlash(null)} aria-label="إغلاق"/></div>}
   <section className="welcome-panel"><div className="welcome-copy"><span className="pill">الخطوة الأولى</span><h2>لنبدأ بتجهيز شبكتك</h2><p>اضبط بيانات النشاط والعملة، ثم اختبر الاتصال براوتر MikroTik عبر الشبكة المحلية.</p><div className="setup-steps"><span className="step-done"><i className="bi bi-check2-circle"/> تشغيل التطبيق</span><span className={settings.setupComplete?'step-done':''}><i className={`bi ${settings.setupComplete?'bi-check2-circle':'bi-circle'}`}/> إعداد النشاط</span><span className={info?'step-done':''}><i className={`bi ${info?'bi-check2-circle':'bi-circle'}`}/> اتصال الراوتر</span></div></div><div className="welcome-art"><div className="orbit orbit-one"/><div className="orbit orbit-two"/><i className="bi bi-router-fill"/><span className="signal signal-a"/><span className="signal signal-b"/><span className="signal signal-c"/></div></section>
   <div className="section-heading"><div><h2>الإعداد الأولي</h2><p>تُحفظ الإعدادات في هذا الجهاز.</p></div><span className="section-index">01 / 02</span></div>
   <div className="row g-4">
    <div className="col-xl-5"><section className="panel h-100"><div className="panel-heading"><div className="panel-icon violet"><i className="bi bi-sliders2"/></div><div><h3>بيانات النشاط والعملة</h3><p>عملة واحدة لجميع العمليات المالية</p></div></div>
     <form onSubmit={save} className="panel-form"><label className="form-label" htmlFor="businessName">اسم الشبكة أو النشاط</label><input id="businessName" className="form-control" maxLength={120} value={settings.businessName} onChange={e=>setting('businessName',e.target.value)} required/>
      <div className="row g-3"><div className="col-6"><label className="form-label" htmlFor="currencyCode">رمز العملة (ISO)</label><input id="currencyCode" className="form-control text-uppercase" maxLength={3} minLength={3} pattern="[A-Za-z]{3}" value={settings.currencyCode} onChange={e=>setting('currencyCode',e.target.value.toUpperCase())} required/><div className="form-hint">مثل YER أو SAR</div></div><div className="col-6"><label className="form-label" htmlFor="currencySymbol">الرمز الظاهر</label><input id="currencySymbol" className="form-control" maxLength={12} value={settings.currencySymbol} onChange={e=>setting('currencySymbol',e.target.value)} required/><div className="form-hint">مثل ر.ي</div></div></div>
      <div className="helper-box"><i className="bi bi-info-circle"/><span>تأكد من العملة قبل بدء البيع؛ لن يمكن تغييرها بعد تسجيل أول معاملة مالية.</span></div><button className="btn btn-primary w-100" disabled={saving}>{saving?'جارٍ الحفظ...':'حفظ الإعدادات'}</button>
     </form></section></div>
    <div className="col-xl-7" id="router"><section className="panel h-100"><div className="panel-heading"><div className="panel-icon cyan"><i className="bi bi-router"/></div><div><h3>الاتصال براوتر MikroTik</h3><p>اختبار قراءة معلومات النظام عبر LAN</p></div><span className="api-chip">RouterOS API</span></div>
     <form onSubmit={testRouter} className="panel-form"><div className="row g-3"><div className="col-md-7"><label className="form-label" htmlFor="routerAddress">عنوان الراوتر</label><input id="routerAddress" className="form-control ltr-input" value={router.address} onChange={e=>route('address',e.target.value)} placeholder="192.168.88.1" autoComplete="off" required/></div><div className="col-md-5"><label className="form-label" htmlFor="routerPort">المنفذ</label><input id="routerPort" className="form-control ltr-input" type="number" min={1} max={65535} value={router.port} onChange={e=>route('port',Number(e.target.value))} required/></div><div className="col-md-6"><label className="form-label" htmlFor="routerUser">اسم المستخدم</label><input id="routerUser" className="form-control ltr-input" value={router.username} onChange={e=>route('username',e.target.value)} autoComplete="username" required/></div><div className="col-md-6"><label className="form-label" htmlFor="routerPassword">كلمة المرور</label><input id="routerPassword" className="form-control ltr-input" type="password" value={router.password} onChange={e=>route('password',e.target.value)} autoComplete="current-password" required/></div></div>
      <label className="tls-option"><input className="form-check-input" type="checkbox" checked={router.useTls} onChange={e=>{const enabled=e.target.checked;route('useTls',enabled);route('port',enabled&&router.port===8728?8729:!enabled&&router.port===8729?8728:router.port)}}/><span><b>استخدام API-SSL</b><small>يتطلب إعداد شهادة موثوقة على الراوتر.</small></span><i className="bi bi-shield-check"/></label><button type="submit" className="btn btn-outline-primary w-100" disabled={testing||loadingHotspot}>{testing?'جارٍ اختبار الاتصال...':'اختبار اتصال الراوتر'}</button><button type="button" className="btn btn-primary w-100" disabled={testing||loadingHotspot} onClick={()=>void loadHotspot()}>{loadingHotspot?'جارٍ قراءة بيانات HotSpot...':'قراءة بيانات HotSpot'}</button>
     </form>
     {info&&<div className="router-result"><div className="result-title"><i className="bi bi-check-circle-fill"/> معلومات الراوتر</div><div className="result-grid">{[['الهوية',info.identity],['RouterOS',info.version],['الطراز',info.model||info.boardName],['مدة التشغيل',info.uptime],['المعالج',info.cpu],['الذاكرة الحرة',info.freeMemory]].map(([k,v])=><div key={k}><small>{k}</small><b>{v||'—'}</b></div>)}</div></div>}
     {hotspot&&<div className="router-result hotspot-overview"><div className="result-title"><i className="bi bi-wifi"/> نظرة عامة على HotSpot</div>
      <div className="row g-2 hotspot-counts">
       {[['الخوادم',hotspot.counts.servers],['الملفات التعريفية',hotspot.counts.profiles],['المستخدمون',hotspot.counts.users],['الجلسات النشطة',hotspot.counts.activeSessions]].map(([label,count])=><div className="col-6 col-xl-3" key={label}><div className="hotspot-count"><small>{label}</small><strong>{count}</strong></div></div>)}
      </div>
      {hotspot.truncated&&<p className="small text-secondary mb-3">يُعرض أول {hotspot.previewLimit} عنصر من كل قائمة؛ الأعداد أعلاه تمثل العدد الكامل على الراوتر.</p>}
      <div className="hotspot-list-block"><h4>الجلسات النشطة ({hotspot.counts.activeSessions})</h4>
       {hotspot.activeSessions.length?<div className="table-responsive"><table className="table table-sm align-middle"><thead><tr><th>المستخدم</th><th>عنوان IP</th><th>مدة الاتصال</th><th>الخادم</th></tr></thead><tbody>{hotspot.activeSessions.map(row=><tr key={row.id||row.user+row.address}><td>{row.user||'—'}</td><td dir="ltr">{row.address||'—'}</td><td>{row.uptime||'—'}</td><td>{row.server||'—'}</td></tr>)}</tbody></table></div>:<p className="small text-secondary">لا توجد جلسات نشطة حاليًا.</p>}
      </div>
      <div className="hotspot-list-block"><h4>مستخدمو HotSpot ({hotspot.counts.users})</h4>
       {hotspot.users.length?<div className="table-responsive"><table className="table table-sm align-middle"><thead><tr><th>اسم المستخدم</th><th>الملف التعريفي</th><th>حد الوقت</th><th>الحالة</th></tr></thead><tbody>{hotspot.users.map(row=><tr key={row.id||row.name}><td>{row.name||'—'}</td><td>{row.profile||'default'}</td><td>{row.limitUptime||'—'}</td><td><span className={row.disabled?'text-danger':'text-success'}>{row.disabled?'معطّل':'مفعّل'}</span></td></tr>)}</tbody></table></div>:<p className="small text-secondary">لا توجد حسابات مستخدمين في القائمة.</p>}
      </div>
      <div className="hotspot-list-block"><h4>الملفات التعريفية ({hotspot.counts.profiles})</h4>
       {hotspot.profiles.length?<div className="table-responsive"><table className="table table-sm align-middle"><thead><tr><th>الاسم</th><th>حد السرعة</th><th>الأجهزة</th><th>مدة الجلسة</th></tr></thead><tbody>{hotspot.profiles.map(row=><tr key={row.name}><td>{row.name||'—'}</td><td>{row.rateLimit||'—'}</td><td>{row.sharedUsers||'—'}</td><td>{row.sessionTimeout||'—'}</td></tr>)}</tbody></table></div>:<p className="small text-secondary">لا توجد ملفات تعريفية.</p>}
      </div>
      <div className="hotspot-list-block"><h4>خوادم HotSpot ({hotspot.counts.servers})</h4>
       {hotspot.servers.length?<div className="table-responsive"><table className="table table-sm align-middle"><thead><tr><th>الاسم</th><th>الواجهة</th><th>مجمع العناوين</th><th>الحالة</th></tr></thead><tbody>{hotspot.servers.map(row=><tr key={row.name}><td>{row.name||'—'}</td><td>{row.interface||'—'}</td><td>{row.addressPool||'—'}</td><td><span className={row.disabled?'text-danger':'text-success'}>{row.disabled?'معطّل':'مفعّل'}</span></td></tr>)}</tbody></table></div>:<p className="small text-secondary">لم يتم إعداد خادم HotSpot على هذا الراوتر.</p>}
      </div>
     </div>}
    </section></div>
   </div>
   <footer className="page-footer"><div><span className="footer-mark">bt</span><span>bt-manager <small>· إدارة محلية لشبكات MikroTik</small></span></div><span><i className="bi bi-shield-check"/> يعمل على جهازك</span></footer>
  </main>
 </div>;
}
