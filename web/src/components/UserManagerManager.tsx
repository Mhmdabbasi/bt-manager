import type {Language} from '../i18n';
import {t} from '../i18n';
import type {UserManagerOverview,UserManagerProfile} from '../types';

type Props={
 language:Language;
 overview:UserManagerOverview|null;
 loading:boolean;
 onRefresh:()=>void;
};

const propertyLabels:Record<string,string>={
 'name-for-users':'الاسم الظاهر للمستخدم',
 'starts-when':'بدء الصلاحية',
 'starts-at':'وقت بدء الصلاحية',
 'override-shared-users':'تجاوز عدد المستخدمين المشتركين',
 'validity':'الصلاحية',
 'price':'السعر',
 'transfer-limit':'حد نقل البيانات',
 'uptime-limit':'حد وقت الاتصال',
 'download-limit':'حد التنزيل',
 'upload-limit':'حد الرفع',
 'rate-limit':'حد السرعة',
 'rate-limit-rx':'سرعة الاستقبال',
 'rate-limit-tx':'سرعة الإرسال',
 'reset-counters-interval':'فترة تصفير العدادات',
 'reset-counters-start-time':'وقت بدء تصفير العدادات',
 'rate-limit-priority':'أولوية السرعة',
 'rate-limit-min-rx':'أدنى استقبال',
 'rate-limit-min-tx':'أدنى إرسال',
 'rate-limit-burst-rx':'استقبال Burst',
 'rate-limit-burst-tx':'إرسال Burst',
 'rate-limit-burst-threshold-rx':'عتبة استقبال Burst',
 'rate-limit-burst-threshold-tx':'عتبة إرسال Burst',
 'rate-limit-burst-time-rx':'مدة استقبال Burst',
 'rate-limit-burst-time-tx':'مدة إرسال Burst',
 'group-name':'المجموعة',
 'address-list':'قائمة العناوين',
 'ip-pool':'مجموعة عناوين IP',
 'from-time':'وقت بداية التفعيل',
 'till-time':'وقت نهاية التفعيل',
 'weekdays':'أيام التفعيل',
 'comment':'ملاحظات',
 'owner':'المالك',
};

function displayDate(value:string,language:Language){
 if(!value)return '—';
 const date=new Date(value);
 if(Number.isNaN(date.getTime()))return value;
 return date.toLocaleString(language==='ar'?'ar-YE':'en-US');
}

function Properties({properties,language}:{properties:Record<string,string>;language:Language}){
 const tx=(value:string)=>t(language,value);
 const entries=Object.entries(properties).filter(([,value])=>value!==''&&value!==undefined);
 if(entries.length===0)return <span className="text-secondary small">{tx('لا توجد تفاصيل إضافية.')}</span>;
 return <dl className="row g-2 mb-0 um-property-grid">
  {entries.map(([key,value])=><div className="col-12 col-md-6" key={key}>
   <div className="small text-secondary">{tx(propertyLabels[key]??key)}</div>
   <div className="fw-semibold text-break" dir="auto">{value}</div>
  </div>)}
 </dl>;
}

function ProfileDetails({profile,language}:{profile:UserManagerProfile;language:Language}){
 const tx=(value:string)=>t(language,value);
 return <div className="um-profile-details">
  <div className="mb-3">
   <div className="fw-semibold mb-2">{tx('خصائص الملف التعريفي')}</div>
   <Properties properties={profile.properties} language={language}/>
  </div>
  <div className="fw-semibold mb-2">{tx('القيود المرتبطة')}</div>
  {profile.limitations.length===0
   ? <div className="alert alert-light border py-2 mb-0 small">{tx('لا توجد قيود مرتبطة بهذا الملف في آخر مزامنة.')}</div>
   : <div className="row g-3">{profile.limitations.map((binding,index)=><div className="col-12 col-xl-6" key={binding.limitation.id}>
      <section className="border rounded-3 p-3 h-100">
       <div className="d-flex align-items-start justify-content-between gap-2 mb-3">
        <div><div className="fw-bold">{binding.limitation.name}</div><div className="small text-secondary">{tx('ترتيب القيد')}: {index+1}</div></div>
        <span className="badge text-bg-light border">{binding.limitation.present?tx('موجود على الراوتر'):tx('غير موجود حاليًا')}</span>
       </div>
       <Properties properties={binding.limitation.properties} language={language}/>
       {Object.keys(binding.properties).length>0&&<div className="mt-3 pt-3 border-top">
        <div className="small fw-semibold mb-2">{tx('إعدادات الربط')}</div>
        <Properties properties={binding.properties} language={language}/>
       </div>}
      </section>
     </div>)}</div>}
 </div>;
}

export default function UserManagerManager({language,overview,loading,onRefresh}:Props){
 const tx=(value:string)=>t(language,value);
 const profileCount=overview?.profiles.filter(profile=>profile.present).length??0;
 const limitationCount=overview?.sync.limitationCount??0;
 const relationshipCount=overview?.sync.relationshipCount??0;

 return <div className="usermanager-page">
  <section className="panel mb-4">
   <div className="panel-heading">
    <div className="panel-icon violet"><i className="bi bi-person-gear"/></div>
    <div className="flex-grow-1">
     <h3>{tx('مصادر User Manager')}</h3>
     <p>{tx('استيراد ملفات الخدمة والقيود وعلاقاتها من الراوتر وحفظ آخر نسخة محلية.')}</p>
    </div>
    <button type="button" className="btn btn-primary" onClick={onRefresh} disabled={loading}>
     <i className={`bi ${loading?'bi-arrow-repeat':'bi-cloud-download'} me-1`}/>
     {loading?tx('جارٍ الاستيراد...'):tx('مزامنة من الراوتر')}
    </button>
   </div>
   {!overview
    ? <div className="empty-state py-5 text-center">
       <div className="panel-icon violet mx-auto mb-3"><i className="bi bi-database-down"/></div>
       <h4>{tx('لم يتم استيراد ملفات User Manager بعد')}</h4>
       <p className="text-secondary mx-auto" style={{maxWidth:560}}>{tx('اختبر اتصال الراوتر من صفحة الإعدادات، ثم اضغط مزامنة لاستيراد الملفات التعريفية والقيود المرتبطة بها.')}</p>
       <button type="button" className="btn btn-outline-primary" onClick={onRefresh} disabled={loading}>{tx('بدء الاستيراد')}</button>
      </div>
    : <>
      <div className="row g-3 mb-4">
       <div className="col-12 col-md-4"><div className="dashboard-stat h-100"><small>{tx('الملفات الموجودة على الراوتر')}</small><strong>{profileCount}</strong></div></div>
       <div className="col-12 col-md-4"><div className="dashboard-stat h-100"><small>{tx('القيود المستوردة')}</small><strong>{limitationCount}</strong></div></div>
       <div className="col-12 col-md-4"><div className="dashboard-stat h-100"><small>{tx('علاقات الملفات بالقيود')}</small><strong>{relationshipCount}</strong></div></div>
      </div>
      <div className="helper-box mb-4"><i className="bi bi-shield-check"/><span>
       {tx('هذه المرحلة للقراءة والاستيراد فقط؛ لا تنشئ أو تعدل مستخدمين على الراوتر.')}
      </span></div>
      <div className="border rounded-3 p-3 mb-4">
       <div className="row g-3">
        <div className="col-12 col-md-4"><small className="text-secondary d-block">{tx('الراوتر')}</small><strong>{overview.router.identity||overview.sync.routerIdentity||'—'}</strong><div className="small text-secondary">{overview.sync.model||overview.router.boardName||'—'}</div></div>
        <div className="col-6 col-md-4"><small className="text-secondary d-block">{tx('إصدار RouterOS')}</small><strong dir="ltr">{overview.router.version}</strong><div className="small text-secondary">{tx(overview.majorVersion===7?'مسارات User Manager v7':'مسارات User Manager v6')}</div></div>
        <div className="col-6 col-md-4"><small className="text-secondary d-block">{tx('آخر مزامنة ناجحة')}</small><strong>{displayDate(overview.sync.syncedAt,language)}</strong></div>
       </div>
      </div>
      <div className="section-heading mb-3">
       <div><h2>{tx('الملفات التعريفية المستوردة')}</h2><p>{tx('افتح تفاصيل أي ملف لعرض خصائصه وجميع القيود المرتبطة به.')}</p></div>
       <span className="badge text-bg-light border">{overview.profiles.length} {tx('ملف')}</span>
      </div>
      {overview.profiles.length===0
       ? <div className="alert alert-warning mb-0">{tx('نجحت المزامنة لكن لم يعثر User Manager على ملفات تعريفية.')}</div>
       : <div className="table-responsive">
         <table className="table align-middle">
          <thead><tr><th>{tx('اسم الملف')}</th><th>{tx('الصلاحية')}</th><th>{tx('سعر الملف في User Manager')}</th><th>{tx('عدد القيود')}</th><th>{tx('التفاصيل')}</th></tr></thead>
          <tbody>{overview.profiles.map(profile=><tr key={profile.id}>
           <td><div className="fw-bold">{profile.name}</div><div className="small text-secondary">{profile.sourceId}</div>{!profile.present&&<span className="badge text-bg-warning mt-1">{tx('غير موجود حاليًا')}</span>}</td>
           <td>{profile.properties.validity||'—'}</td>
           <td>{profile.properties.price||'—'}</td>
           <td><span className="badge text-bg-light border">{profile.limitations.length}</span></td>
           <td><details><summary className="text-primary">{tx('عرض التفاصيل')}</summary><ProfileDetails profile={profile} language={language}/></details></td>
          </tr>)}</tbody>
         </table>
        </div>}
      <div className="section-heading mt-5 mb-3">
       <div><h2>{tx('جميع القيود المستوردة')}</h2><p>{tx('تظهر هنا القيود حتى إن لم تكن مرتبطة حاليًا بملف تعريفي.')}</p></div>
       <span className="badge text-bg-light border">{overview.limitations.length} {tx('قيد')}</span>
      </div>
      {overview.limitations.length===0
       ? <div className="alert alert-light border mb-0">{tx('لم يعثر User Manager على قيود.')}</div>
       : <div className="table-responsive">
         <table className="table align-middle">
          <thead><tr><th>{tx('اسم القيد')}</th><th>{tx('الحالة')}</th><th>{tx('خصائص القيد')}</th><th>{tx('آخر ظهور')}</th></tr></thead>
          <tbody>{overview.limitations.map(limitation=><tr key={limitation.id}>
           <td><div className="fw-bold">{limitation.name}</div><div className="small text-secondary">{limitation.sourceId}</div></td>
           <td><span className={`badge ${limitation.present?'text-bg-success':'text-bg-warning'}`}>{limitation.present?tx('موجود على الراوتر'):tx('غير موجود حاليًا')}</span></td>
           <td><details><summary className="text-primary">{tx('عرض التفاصيل')}</summary><div className="pt-3"><Properties properties={limitation.properties} language={language}/></div></details></td>
           <td className="small text-secondary">{displayDate(limitation.lastSeenAt,language)}</td>
          </tr>)}</tbody>
         </table>
        </div>}
     </>}
  </section>
 </div>;
}
