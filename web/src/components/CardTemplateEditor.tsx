import {useEffect,useState,type FormEvent} from 'react';
import {api} from '../api';
import type {CardPackage,CardTemplate,CardTemplateElement,PointOfSale} from '../types';
import type {Language} from '../i18n';

type Props={language:Language;template:CardTemplate|null;packages:CardPackage[];points:PointOfSale[];onCancel:()=>void;onSaved:(message:string)=>void;onMessage:(kind:'success'|'danger',message:string)=>void};
type ElementField=CardTemplateElement['field'];
const fields:ElementField[]=['title','usernameLabel','username','passwordLabel','password','profile','uptime','note'];
const emptyElement=(index:number):CardTemplateElement=>({id:'element-'+(index+1),field:'username',x:5,y:40,width:90,fontSize:12,color:'#123456',align:'center',bold:true,visible:true});
export default function CardTemplateEditor({language,template,packages,points,onCancel,onSaved,onMessage}:Props){
 const ar=language==='ar',tx=(a:string,e:string)=>ar?a:e;
 const [name,setName]=useState(template?.name??'');
 const [packageId,setPackageId]=useState(String(template?.packageId??packages.find(p=>p.active)?.id??''));
 const [pointOfSaleId,setPointOfSaleId]=useState(String(template?.pointOfSaleId??''));
 const [isDefault,setIsDefault]=useState(template?.isDefault??false);
 const [width,setWidth]=useState(String(template?.cardWidthMm??50));
 const [height,setHeight]=useState(String(template?.cardHeightMm??19.1));
 const [columns,setColumns]=useState(String(template?.pageLayout.columns??4));
 const [rows,setRows]=useState(String(template?.pageLayout.rows??15));
 const [margin,setMargin]=useState(String(template?.pageLayout.marginMm??5));
 const [gap,setGap]=useState(String(template?.pageLayout.gapMm??0));
 const [elements,setElements]=useState<CardTemplateElement[]>(template?.design??[]);
 const [saving,setSaving]=useState(false);
 useEffect(()=>{setName(template?.name??'');setPackageId(String(template?.packageId??packages.find(p=>p.active)?.id??''));setPointOfSaleId(String(template?.pointOfSaleId??''));setIsDefault(template?.isDefault??false);setWidth(String(template?.cardWidthMm??50));setHeight(String(template?.cardHeightMm??19.1));setColumns(String(template?.pageLayout.columns??4));setRows(String(template?.pageLayout.rows??15));setMargin(String(template?.pageLayout.marginMm??5));setGap(String(template?.pageLayout.gapMm??0));setElements(template?.design??[])},[template,packages]);
 const updateElement=(index:number,patch:Partial<CardTemplateElement>)=>setElements(current=>current.map((item,i)=>i===index?{...item,...patch}:item));
 async function submit(event:FormEvent){event.preventDefault();if(!Number(packageId)){onMessage('danger',tx('اختر الباقة المرتبطة بالقالب.','Choose a package for this template.'));return}setSaving(true);try{await api.saveCardTemplate({...(template?{id:template.id}:{}),packageId:Number(packageId),...(pointOfSaleId?{pointOfSaleId:Number(pointOfSaleId)}:{}),name,isDefault:!pointOfSaleId&&isDefault,cardWidthMm:Number(width),cardHeightMm:Number(height),design:elements,pageLayout:{columns:Number(columns),rows:Number(rows),marginMm:Number(margin),gapMm:Number(gap)}});onSaved(tx('تم حفظ القالب بنجاح.','Template saved successfully.'))}catch(error){onMessage('danger',(error as Error).message||tx('تعذر حفظ القالب.','Could not save template.'))}finally{setSaving(false)}}
 return <section className="panel card-template-editor" dir={ar?'rtl':'ltr'}>
  <div className="panel-heading"><div className="panel-icon violet"><i className="bi bi-palette2"/></div><div><h3>{template?tx('تعديل القالب','Edit template'):tx('إنشاء قالب جديد','Create template')}</h3><p>{tx('إعدادات القالب الأساسية وتخطيط صفحة الطباعة.','Template basics and print-page layout.')}</p></div></div>
  <form className="panel-form" onSubmit={submit}>
   <div className="row g-3">
    <div className="col-md-6"><label className="form-label">{tx('اسم القالب','Template name')}</label><input className="form-control" value={name} onChange={e=>setName(e.target.value)} maxLength={100} required/></div>
    <div className="col-md-6"><label className="form-label">{tx('الباقة','Package')}</label><select className="form-select" value={packageId} onChange={e=>setPackageId(e.target.value)} required><option value="">{tx('اختر الباقة','Select package')}</option>{packages.map(p=><option key={p.id} value={p.id}>{p.name} · {p.serviceSystem==='hotspot'?'HotSpot':'User Manager'}{!p.active?' (غير نشطة)':''}</option>)}</select></div>
    <div className="col-md-6"><label className="form-label">{tx('نطاق الاستخدام','Template scope')}</label><select className="form-select" value={pointOfSaleId} onChange={e=>{setPointOfSaleId(e.target.value);if(e.target.value)setIsDefault(false)}}><option value="">{tx('قالب عام للباقة','Global for package')}</option>{points.map(p=><option key={p.id} value={p.id}>{p.name}</option>)}</select></div>
    <div className="col-md-6 d-flex align-items-end"><label className="form-check mb-2"><input className="form-check-input" type="checkbox" checked={isDefault&&!pointOfSaleId} disabled={Boolean(pointOfSaleId)} onChange={e=>setIsDefault(e.target.checked)}/> <span className="form-check-label">{tx('القالب الافتراضي للباقة','Default template for package')}</span></label></div>
    <div className="col-6 col-md-3"><label className="form-label">{tx('عرض البطاقة (مم)','Card width (mm)')}</label><input className="form-control" type="number" min="1" max="300" step="0.1" value={width} onChange={e=>setWidth(e.target.value)} required/></div>
    <div className="col-6 col-md-3"><label className="form-label">{tx('ارتفاع البطاقة (مم)','Card height (mm)')}</label><input className="form-control" type="number" min="1" max="300" step="0.1" value={height} onChange={e=>setHeight(e.target.value)} required/></div>
    <div className="col-6 col-md-3"><label className="form-label">{tx('الأعمدة','Columns')}</label><input className="form-control" type="number" min="1" max="10" value={columns} onChange={e=>setColumns(e.target.value)} required/></div>
    <div className="col-6 col-md-3"><label className="form-label">{tx('الصفوف','Rows')}</label><input className="form-control" type="number" min="1" max="40" value={rows} onChange={e=>setRows(e.target.value)} required/></div>
    <div className="col-6 col-md-3"><label className="form-label">{tx('الهامش (مم)','Margin (mm)')}</label><input className="form-control" type="number" min="0" max="20" step="0.1" value={margin} onChange={e=>setMargin(e.target.value)} required/></div>
    <div className="col-6 col-md-3"><label className="form-label">{tx('الفاصل (مم)','Gap (mm)')}</label><input className="form-control" type="number" min="0" max="10" step="0.1" value={gap} onChange={e=>setGap(e.target.value)} required/></div>
    <div className="col-12"><div className="alert alert-info py-2 mb-0">{tx('عدد البطاقات في الصفحة: ','Cards per page: ')}<strong>{Math.max(0,Number(columns)||0)*Math.max(0,Number(rows)||0)}</strong>. {tx('يتم حفظ التعديلات كإصدار جديد مع الاحتفاظ بالإصدارات السابقة.','Saving creates a new version while preserving previous versions.')}</div></div>
   </div>
   <div className="d-flex justify-content-between align-items-center mt-4 mb-2"><h4 className="h6 mb-0">{tx('عناصر التصميم','Design elements')}</h4><button type="button" className="btn btn-sm btn-outline-primary" disabled={elements.length>=50} onClick={()=>setElements(current=>[...current,emptyElement(current.length)])}><i className="bi bi-plus-lg me-1"/>{tx('إضافة عنصر','Add element')}</button></div>
   {elements.length===0&&<p className="small text-secondary">{tx('لا توجد عناصر تصميم في هذا القالب بعد. أضف عنصرًا لتحديد الحقل ومكانه وتنسيقه.','No design elements yet. Add an element to configure its field, position, and style.')}</p>}
   {elements.map((item,index)=><div className="border rounded-3 p-3 mb-2" key={item.id}><div className="row g-2 align-items-end">
    <div className="col-md-3"><label className="form-label">{tx('الحقل','Field')}</label><select className="form-select" value={item.field} onChange={e=>updateElement(index,{field:e.target.value as ElementField})}>{fields.map(field=><option key={field} value={field}>{field}</option>)}</select></div>
    <div className="col-4 col-md-2"><label className="form-label">X %</label><input className="form-control" type="number" min="0" max="100" value={item.x} onChange={e=>updateElement(index,{x:Number(e.target.value)})}/></div>
    <div className="col-4 col-md-2"><label className="form-label">Y %</label><input className="form-control" type="number" min="0" max="100" value={item.y} onChange={e=>updateElement(index,{y:Number(e.target.value)})}/></div>
    <div className="col-4 col-md-2"><label className="form-label">{tx('العرض %','Width %')}</label><input className="form-control" type="number" min="1" max="100" value={item.width} onChange={e=>updateElement(index,{width:Number(e.target.value)})}/></div>
    <div className="col-6 col-md-1"><label className="form-label">{tx('الحجم','Size')}</label><input className="form-control" type="number" min="4" max="72" value={item.fontSize} onChange={e=>updateElement(index,{fontSize:Number(e.target.value)})}/></div>
    <div className="col-6 col-md-2"><label className="form-label">{tx('اللون','Color')}</label><input className="form-control form-control-color w-100" type="color" value={item.color} onChange={e=>updateElement(index,{color:e.target.value})}/></div>
    <div className="col-6 col-md-3"><label className="form-label">{tx('المحاذاة','Alignment')}</label><select className="form-select" value={item.align} onChange={e=>updateElement(index,{align:e.target.value as CardTemplateElement['align']})}><option value="right">{tx('يمين','Right')}</option><option value="center">{tx('وسط','Center')}</option><option value="left">{tx('يسار','Left')}</option></select></div>
    <div className="col-6 col-md-3"><label className="form-check mb-2"><input className="form-check-input" type="checkbox" checked={item.bold} onChange={e=>updateElement(index,{bold:e.target.checked})}/> <span className="form-check-label">{tx('عريض','Bold')}</span></label><label className="form-check"><input className="form-check-input" type="checkbox" checked={item.visible!==false} onChange={e=>updateElement(index,{visible:e.target.checked})}/> <span className="form-check-label">{tx('ظاهر','Visible')}</span></label></div>
    <div className="col-12 col-md-2 text-end"><button type="button" className="btn btn-sm btn-outline-danger" onClick={()=>setElements(current=>current.filter((_,i)=>i!==index))}><i className="bi bi-trash me-1"/>{tx('إزالة','Remove')}</button></div>
   </div></div>)}
   <div className="d-flex flex-wrap gap-2 mt-4"><button className="btn btn-primary" type="submit" disabled={saving}>{saving?tx('جارٍ الحفظ...','Saving...'):tx('حفظ القالب','Save template')}</button><button className="btn btn-outline-secondary" type="button" onClick={onCancel} disabled={saving}>{tx('إلغاء','Cancel')}</button></div>
  </form>
 </section>;
}
