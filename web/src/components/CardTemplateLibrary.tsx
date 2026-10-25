import {useEffect,useMemo,useState} from 'react';
import {api} from '../api';
import type {CardPackage,CardTemplate,PointOfSale} from '../types';
import type {Language} from '../i18n';
import CardTemplateEditor from './CardTemplateEditor';

type Props={language:Language;onMessage:(kind:'success'|'danger',message:string)=>void};
export default function CardTemplateLibrary({language,onMessage}:Props){
 const ar=language==='ar',tx=(a:string,e:string)=>ar?a:e;
 const [templates,setTemplates]=useState<CardTemplate[]>([]);
 const [packages,setPackages]=useState<CardPackage[]>([]);
 const [points,setPoints]=useState<PointOfSale[]>([]);
 const [loading,setLoading]=useState(true);
 const [query,setQuery]=useState('');
 const [packageFilter,setPackageFilter]=useState('all');
 const [scopeFilter,setScopeFilter]=useState('all');
 const [reload,setReload]=useState(0);
 const [editing,setEditing]=useState<CardTemplate|null|undefined>(undefined);
 useEffect(()=>{
  let active=true;setLoading(true);
  void Promise.all([api.cardTemplates(0),api.cardPackages(true),api.pointsOfSale(true)]).then(([t,p,pos])=>{
   if(!active)return;setTemplates(t.templates);setPackages(p.packages);setPoints(pos.pointsOfSale);
  }).catch(error=>{if(active)onMessage('danger',(error as Error).message||tx('تعذر تحميل قوالب الكروت.','Could not load card templates.'))})
   .finally(()=>{if(active)setLoading(false)});
  return()=>{active=false};
 },[reload]);
 const filtered=useMemo(()=>{
  const q=query.trim().toLocaleLowerCase();
  return templates.filter(item=>{
   const packageName=packages.find(p=>p.id===item.packageId)?.name||'';
   const pointName=points.find(p=>p.id===item.pointOfSaleId)?.name||'';
   return (!q||[item.name,packageName,pointName,String(item.id)].some(v=>v.toLocaleLowerCase().includes(q)))
    &&(packageFilter==='all'||String(item.packageId)===packageFilter)
    &&(scopeFilter==='all'||item.scopeKind===scopeFilter);
  });
 },[templates,packages,points,query,packageFilter,scopeFilter]);
 if(editing!==undefined)return <CardTemplateEditor language={language} template={editing} packages={packages} points={points} onCancel={()=>setEditing(undefined)} onMessage={onMessage} onSaved={message=>{setEditing(undefined);setReload(v=>v+1);onMessage('success',message)}}/>;
 return <section className="hotspot-management-section card-template-library" dir={ar?'rtl':'ltr'}>
  <div className="hotspot-section-title"><div><h3>{tx('مكتبة قوالب الكروت','Card template library')}</h3><p className="small text-secondary mb-0 mt-1">{tx('أنشئ القوالب وعدّل إعداداتها بصورة مستقلة عن توليد القسائم.','Create and configure templates independently from voucher generation.')}</p></div><div className="d-flex gap-2"><button type="button" className="btn btn-primary" onClick={()=>setEditing(null)}><i className="bi bi-plus-lg me-1"/>{tx('قالب جديد','New template')}</button><button type="button" className="btn btn-outline-primary" onClick={()=>setReload(v=>v+1)} disabled={loading}><i className="bi bi-arrow-clockwise me-1"/>{tx('تحديث','Refresh')}</button></div></div>
  <div className="row g-2 mb-3">
   <div className="col-lg-5"><label className="form-label" htmlFor="template-search">{tx('بحث','Search')}</label><input id="template-search" className="form-control" value={query} onChange={e=>setQuery(e.target.value)} placeholder={tx('اسم القالب أو الباقة أو نقطة البيع','Template, package, or point of sale')}/></div>
   <div className="col-md-6 col-lg-4"><label className="form-label" htmlFor="template-package-filter">{tx('الباقة','Package')}</label><select id="template-package-filter" className="form-select" value={packageFilter} onChange={e=>setPackageFilter(e.target.value)}><option value="all">{tx('كل الباقات','All packages')}</option>{packages.map(p=><option key={p.id} value={p.id}>{p.name}</option>)}</select></div>
   <div className="col-md-6 col-lg-3"><label className="form-label" htmlFor="template-scope-filter">{tx('نطاق القالب','Template scope')}</label><select id="template-scope-filter" className="form-select" value={scopeFilter} onChange={e=>setScopeFilter(e.target.value)}><option value="all">{tx('كل القوالب','All templates')}</option><option value="global">{tx('عام','Global')}</option><option value="point_of_sale">{tx('مخصص لنقطة بيع','Point-of-sale')}</option></select></div>
  </div>
  <div className="d-flex justify-content-between align-items-center small text-secondary mb-2"><span>{tx('القوالب المطابقة','Matching templates')}: {filtered.length}</span><span>{tx('الإجمالي','Total')}: {templates.length}</span></div>
  <div className="table-responsive"><table className="table table-sm align-middle hotspot-data-table"><thead><tr><th>{tx('اسم القالب','Template')}</th><th>{tx('الباقة','Package')}</th><th>{tx('النطاق','Scope')}</th><th>{tx('الأبعاد','Dimensions')}</th><th>{tx('تخطيط الصفحة','Page layout')}</th><th>{tx('الإصدار','Version')}</th><th>{tx('الحالة','Status')}</th><th>{tx('الإجراءات','Actions')}</th></tr></thead><tbody>
   {loading&&<tr><td colSpan={8} className="text-center py-4">{tx('جارٍ تحميل القوالب...','Loading templates...')}</td></tr>}
   {!loading&&filtered.map(item=><tr key={item.id}><td><strong>{item.name}</strong>{item.isDefault&&<span className="badge text-bg-primary ms-2">{tx('افتراضي','Default')}</span>}<div className="small text-secondary">#{item.id}</div></td><td>{packages.find(p=>p.id===item.packageId)?.name||'—'}</td><td>{item.scopeKind==='global'?tx('عام','Global'):(points.find(p=>p.id===item.pointOfSaleId)?.name||tx('نقطة بيع غير معروفة','Unknown point of sale'))}</td><td dir="ltr">{item.cardWidthMm} × {item.cardHeightMm} mm</td><td dir="ltr">{item.pageLayout.columns} × {item.pageLayout.rows} ({item.pageLayout.columns*item.pageLayout.rows})</td><td>{item.versionNumber||1}</td><td><span className={item.active?'text-success':'text-secondary'}>{item.active?tx('نشط','Active'):tx('معطل','Inactive')}</span></td><td><button type="button" className="btn btn-sm btn-outline-primary" onClick={()=>setEditing(item)}><i className="bi bi-pencil-square me-1"/>{tx('تعديل','Edit')}</button></td></tr>)}
   {!loading&&!filtered.length&&<tr><td colSpan={8} className="text-center text-secondary py-4">{templates.length?tx('لا توجد قوالب تطابق البحث والفلاتر.','No templates match these filters.'):tx('لم يُعثر على قوالب محفوظة. يمكنك إنشاء قالب جديد من الزر أعلاه.','No saved templates found. Create one using the button above.')}</td></tr>}
  </tbody></table></div>
  <div className="alert alert-info small mb-0 mt-3">{tx('يدعم هذا الإصدار إنشاء القالب وتعديل إعداداته وعناصر التصميم الأساسية. ستأتي المعاينة المرئية والخلفيات والسحب والإفلات والحذف الآمن وربط القالب بالتوليد في مراحل لاحقة.','This release supports creating templates and editing basic settings and design elements. Visual preview, backgrounds, drag-and-drop, safe deletion, and generation binding will follow in later milestones.')}</div>
 </section>;
}
