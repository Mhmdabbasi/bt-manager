import {useEffect,useState,type FormEvent} from 'react';
import {api} from '../api';
import {formatT,t,type Language} from '../i18n';
import type {HotSpotProfile,HotSpotServer,POSSale,POSSaleResult,VoucherProduct} from '../types';

type FlashKind='success'|'danger';
type Props={
 language:Language;
 profiles:HotSpotProfile[];
 servers:HotSpotServer[];
 currencyCode:string;
 currencySymbol:string;
 onMessage:(kind:FlashKind,message:string)=>void;
};
type ProductForm={name:string;profileName:string;serverName:string;limitUptime:string;price:string;active:boolean};
const emptyForm:ProductForm={name:'',profileName:'',serverName:'all',limitUptime:'',price:'',active:true};
type PaymentMethod='cash'|'transfer'|'card'|'other';
const paymentLabels:Record<PaymentMethod,string>={cash:'نقدًا',transfer:'حوالة',card:'بطاقة',other:'أخرى'};

function formatAmount(minor:number,code:string,symbol:string,language:Language='ar'){
 const amount=(minor/100).toLocaleString(language==='ar'?'ar-YE':'en-US',{minimumFractionDigits:2,maximumFractionDigits:2});
 return amount+' '+(symbol||code);
}
function makeRequestID(){
 if(typeof crypto!=='undefined'&&typeof crypto.randomUUID==='function')return crypto.randomUUID();
 return Date.now().toString(36)+'-'+Math.random().toString(36).slice(2,14);
}

export default function POSPanel({language,profiles,servers,currencyCode,currencySymbol,onMessage}:Props){
 const tx=(text:string)=>t(language,text);
 const [products,setProducts]=useState<VoucherProduct[]>([]);
 const [sales,setSales]=useState<POSSale[]>([]);
 const [form,setForm]=useState<ProductForm>(emptyForm);
 const [editingID,setEditingID]=useState<number|null>(null);
 const [savingProduct,setSavingProduct]=useState(false);
 const [productLoading,setProductLoading]=useState(true);
 const [saleProductID,setSaleProductID]=useState('');
 const [quantity,setQuantity]=useState(1);
 const [paymentMethod,setPaymentMethod]=useState<PaymentMethod>('cash');
 const [selling,setSelling]=useState(false);
 const [requestID,setRequestID]=useState(makeRequestID);
 const [receipt,setReceipt]=useState<POSSaleResult|null>(null);
 const [receiptLoading,setReceiptLoading]=useState(false);

 async function reloadData(){
  setProductLoading(true);
  try{
   const results=await Promise.all([api.voucherProducts(true),api.posSales()]);
   setProducts(results[0].products);
   setSales(results[1].sales);
   setSaleProductID(current=>{
    const valid=results[0].products.find(item=>String(item.id)===current&&item.active&&item.stockCount>0);
    if(valid)return current;
    const first=results[0].products.find(item=>item.active&&item.stockCount>0);
    return first?String(first.id):'';
   });
  }catch(error){
   onMessage('danger',tx((error as Error).message||'تعذر تحميل بيانات نقطة البيع.'));
  }finally{setProductLoading(false)}
 }

 useEffect(()=>{void reloadData()},[profiles,servers]);

 const selectedProduct=products.find(item=>String(item.id)===saleProductID);
 const maxQuantity=selectedProduct?Math.min(200,selectedProduct.stockCount):0;
 const parsedPrice=Number(form.price);
 const priceMinor=Math.round(parsedPrice*100);
 const validPrice=Number.isFinite(parsedPrice)&&priceMinor>0&&priceMinor<=1000000000000;

 function resetProduct(){setForm(emptyForm);setEditingID(null)}
 function startEdit(product:VoucherProduct){
  setEditingID(product.id);
  setForm({name:product.name,profileName:product.profileName,serverName:product.serverName||'all',limitUptime:product.limitUptime||'',price:(product.priceMinor/100).toString(),active:product.active});
  onMessage('success',tx('تم تحميل الباقة للتعديل.'));
 }
 async function submitProduct(event:FormEvent){
  event.preventDefault();
  if(!form.profileName||!validPrice){
   onMessage('danger',tx('اختر ملفًا تعريفيًا وأدخل سعرًا أكبر من صفر.'));
   return;
  }
  setSavingProduct(true);
  try{
   const payload={name:form.name.trim(),profileName:form.profileName,serverName:form.serverName,limitUptime:form.limitUptime.trim(),priceMinor,active:form.active};
   if(editingID!==null){
    await api.updateVoucherProduct(editingID,payload);
    onMessage('success',tx('تم تحديث باقة البيع.'));
   }else{
    await api.createVoucherProduct({name:payload.name,profileName:payload.profileName,serverName:payload.serverName,limitUptime:payload.limitUptime,priceMinor:payload.priceMinor});
    onMessage('success',tx('تم إنشاء باقة البيع.'));
   }
   resetProduct();
   await reloadData();
  }catch(error){
   onMessage('danger',tx((error as Error).message||'تعذر حفظ باقة البيع.'));
  }finally{setSavingProduct(false)}
 }

 async function toggleProduct(product:VoucherProduct){
  try{
   await api.updateVoucherProduct(product.id,{name:product.name,profileName:product.profileName,serverName:product.serverName,limitUptime:product.limitUptime,priceMinor:product.priceMinor,active:!product.active});
   onMessage('success',tx(product.active?'تم إيقاف الباقة عن البيع.':'تم تفعيل الباقة للبيع.'));
   await reloadData();
  }catch(error){onMessage('danger',tx((error as Error).message||'تعذر تغيير حالة الباقة.'))}
 }

 async function submitSale(event:FormEvent){
  event.preventDefault();
  if(!selectedProduct||quantity<1||quantity>maxQuantity){
   onMessage('danger',tx('المخزون المتاح لا يكفي للكمية المطلوبة.'));
   return;
  }
  setSelling(true);
  try{
   const result=await api.createPOSSale({requestId:requestID,productId:selectedProduct.id,quantity,paymentMethod});
   setReceipt(result);
   setRequestID(makeRequestID());
   onMessage('success',language==='ar'?'تم تسجيل البيع '+result.sale.reference+' وإخراج '+result.items.length+' كرت.':formatT(language,'Sale {reference} recorded and {count} vouchers issued.',{reference:result.sale.reference,count:result.items.length}));
   await reloadData();
  }catch(error){
   // Keep the same request ID after failures so a safe retry cannot double-sell.
   onMessage('danger',tx((error as Error).message||'تعذر إتمام البيع.'));
  }finally{setSelling(false)}
 }

 async function openReceipt(id:number){
  setReceiptLoading(true);
  try{setReceipt(await api.posSale(id))}
  catch(error){onMessage('danger',tx((error as Error).message||'تعذر قراءة الإيصال.'))}
  finally{setReceiptLoading(false)}
 }

 function printReceipt(){
  if(!receipt)return;
  document.body.classList.add('print-pos-receipt');
  const cleanup=()=>document.body.classList.remove('print-pos-receipt');
  window.addEventListener('afterprint',cleanup,{once:true});
  window.print();
  window.setTimeout(cleanup,1500);
 }

 return <section className="pos-panel hotspot-management-section" id="pos">
  <div className="hotspot-section-title"><div><h3>{tx("نقطة البيع (POS)")}</h3><p className="small text-secondary mb-0 mt-1">{tx("تسجيل المبيعات من مخزون القسائم الذي تم توليده وحفظه محليًا.")}</p></div><span>{currencyCode}</span></div>
  <div className="row g-3 pos-stat-row">
   <div className="col-sm-4"><div className="pos-stat"><small>{tx("باقات البيع")}</small><strong>{products.filter(item=>item.active).length}</strong></div></div>
   <div className="col-sm-4"><div className="pos-stat"><small>{tx("قوالب متاحة بالمخزون")}</small><strong>{products.reduce((sum,item)=>sum+item.stockCount,0)}</strong></div></div>
   <div className="col-sm-4"><div className="pos-stat"><small>{tx("عمليات بيع مسجلة")}</small><strong>{sales.length}</strong></div></div>
  </div>

  <section className="pos-subsection">
   <div className="hotspot-section-title"><h3>{tx("باقات البيع والأسعار")}</h3><span>{tx("مخزون حسب الملف التعريفي")}</span></div>
   <p className="small text-secondary">{tx("كل باقة مرتبطة بملف HotSpot تعريفي واحد. يجب توليد القسائم بهذا الملف أولًا حتى تظهر كمية المخزون القابلة للبيع.")}</p>
   <form className="hotspot-editor" onSubmit={submitProduct}>
    <h4>{editingID!==null?'تعديل باقة':'إضافة باقة إلى نقطة البيع'}</h4>
    <div className="row g-3">
     <div className="col-md-4"><label className="form-label" htmlFor="posProductName">{tx("اسم الباقة")}</label><input id="posProductName" className="form-control" maxLength={80} value={form.name} onChange={event=>setForm(old=>({...old,name:event.target.value}))} placeholder="مثال: كرت 200 ريال" required/></div>
     <div className="col-md-4"><label className="form-label" htmlFor="posProductProfile">{tx("الملف التعريفي")}</label><select id="posProductProfile" className="form-select form-control" value={form.profileName} onChange={event=>setForm(old=>({...old,profileName:event.target.value}))} required><option value="">{tx("اختر ملفًا")}</option>{profiles.map(item=><option key={item.id||item.name} value={item.name}>{item.name}</option>)}</select></div>
     <div className="col-md-4"><label className="form-label" htmlFor="posProductServer">{tx("خادم HotSpot")}</label><select id="posProductServer" className="form-select form-control" value={form.serverName} onChange={event=>setForm(old=>({...old,serverName:event.target.value}))} required><option value="all">{tx("كل الخوادم (all)")}</option>{servers.filter(item=>!item.disabled).map(item=><option key={item.name} value={item.name}>{item.name}</option>)}</select></div>
     <div className="col-md-4"><label className="form-label" htmlFor="posProductLimit">{tx("حد الاستخدام الزمني")}</label><input id="posProductLimit" className="form-control ltr-input" maxLength={64} value={form.limitUptime} onChange={event=>setForm(old=>({...old,limitUptime:event.target.value}))} placeholder="مثال: 1d00:00:00"/><div className="form-hint">{tx("اتركه فارغًا إذا لم يُحدَّد حد زمني.")}</div></div>
     <div className="col-md-4"><label className="form-label" htmlFor="posProductPrice">سعر الوحدة ({currencySymbol||currencyCode})</label><input id="posProductPrice" type="number" step="0.01" min="0.01" max="10000000000" className="form-control ltr-input" value={form.price} onChange={event=>setForm(old=>({...old,price:event.target.value}))} required/></div>
    </div>
    {editingID!==null&&<label className="form-check mt-3"><input className="form-check-input" type="checkbox" checked={form.active} onChange={event=>setForm(old=>({...old,active:event.target.checked}))}/><span className="form-check-label">{tx("الباقة نشطة ويمكن بيعها")}</span></label>}
    <div className="d-flex flex-wrap gap-2 mt-3"><button className="btn btn-primary" type="submit" disabled={savingProduct||!profiles.length||!validPrice}>{savingProduct?'جارٍ الحفظ...':editingID!==null?'حفظ التغييرات':'إضافة الباقة'}</button>{editingID!==null&&<button type="button" className="btn btn-light" onClick={resetProduct} disabled={savingProduct}>{tx("إلغاء")}</button>}</div>
   </form>

   <div className="table-responsive mt-3"><table className="table table-sm align-middle hotspot-data-table"><thead><tr><th>{tx("الباقة")}</th><th>{tx("الملف")}</th><th>{tx("الخادم")}</th><th>{tx("حد الوقت")}</th><th>{tx("السعر")}</th><th>{tx("المخزون")}</th><th>{tx("الحالة")}</th><th>{tx("إجراءات")}</th></tr></thead><tbody>
    {products.map(product=><tr key={product.id}><td>{product.name}</td><td>{product.profileName}</td><td>{product.serverName||'all'}</td><td>{product.limitUptime||'—'}</td><td>{formatAmount(product.priceMinor,currencyCode,currencySymbol,language)}</td><td>{product.stockCount}</td><td><span className={product.active?'text-success':'text-secondary'}>{product.active?'نشطة':'موقوفة'}</span></td><td><div className="d-flex flex-wrap gap-1"><button type="button" className="btn btn-sm btn-outline-secondary" onClick={()=>startEdit(product)} disabled={savingProduct}>{tx("تعديل")}</button><button type="button" className="btn btn-sm btn-outline-primary" onClick={()=>void toggleProduct(product)} disabled={savingProduct}>{product.active?'إيقاف':'تفعيل'}</button></div></td></tr>)}
    {!products.length&&<tr><td colSpan={8} className="text-center text-secondary py-3">{tx("لا توجد باقات بعد. أنشئ باقة واختر الملف التعريفي وخادم HotSpot المطابقين للقسائم التي ولدتها.")}</td></tr>}
   </tbody></table></div>
  </section>

  <section className="pos-subsection">
   <div className="hotspot-section-title"><h3>{tx("بيع كروت الدخول")}</h3><span>{tx("عملية ذرّية — لا بيع فوق المخزون")}</span></div>
   <form className="hotspot-editor" onSubmit={submitSale}>
    <div className="row g-3">
     <div className="col-md-6"><label className="form-label" htmlFor="posSaleProduct">{tx("باقة البيع")}</label><select id="posSaleProduct" className="form-select form-control" value={saleProductID} onChange={event=>{setSaleProductID(event.target.value);setQuantity(1)}} required><option value="">{tx("اختر باقة متاحة")}</option>{products.filter(item=>item.active&&item.stockCount>0).map(item=><option key={item.id} value={item.id}>{item.name} — {item.serverName||'all'} — {item.limitUptime||'بلا حد زمني'} — {formatAmount(item.priceMinor,currencyCode,currencySymbol,language)} — المخزون {item.stockCount}</option>)}</select></div>
     <div className="col-md-3"><label className="form-label" htmlFor="posSaleQuantity">{tx("الكمية")}</label><input id="posSaleQuantity" className="form-control ltr-input" type="number" min={1} max={Math.max(1,maxQuantity)} value={quantity} onChange={event=>setQuantity(Number(event.target.value))} required/></div>
     <div className="col-md-3"><label className="form-label" htmlFor="posPaymentMethod">{tx("طريقة الدفع")}</label><select id="posPaymentMethod" className="form-select form-control" value={paymentMethod} onChange={event=>setPaymentMethod(event.target.value as PaymentMethod)}>{Object.entries(paymentLabels).map(([value,label])=><option key={value} value={value}>{tx(label)}</option>)}</select></div>
    </div>
    <div className="pos-sale-total"><span>{tx("الإجمالي")}</span><strong>{formatAmount((selectedProduct?.priceMinor??0)*Math.max(0,quantity),currencyCode,currencySymbol,language)}</strong></div>
    {selectedProduct&&<div className="helper-box"><i className="bi bi-box-seam"/><span>المتاح {selectedProduct.stockCount} كرت. سيُخصم المخزون وتُسجل المعاملة المالية معًا في قاعدة البيانات، ولن يُباع الكرت نفسه مرتين.</span></div>}
    <button type="submit" className="btn btn-primary mt-3" disabled={selling||!selectedProduct||quantity<1||quantity>maxQuantity}>{selling?'جارٍ تسجيل البيع...':'تأكيد البيع وإصدار الإيصال'}</button>
   </form>
  </section>

  <section className="pos-subsection">
   <div className="hotspot-section-title"><h3>{tx("سجل المبيعات")}</h3><span>آخر {sales.length} عملية</span></div>
   {productLoading?<p className="small text-secondary">{tx("جارٍ تحميل السجل...")}</p>:<div className="table-responsive"><table className="table table-sm align-middle hotspot-data-table"><thead><tr><th>{tx("المرجع")}</th><th>{tx("التاريخ")}</th><th>{tx("الإجمالي")}</th><th>{tx("الدفع")}</th><th>{tx("الإيصال")}</th></tr></thead><tbody>
    {sales.map(sale=><tr key={sale.id}><td dir="ltr">{sale.reference}</td><td>{new Date(sale.createdAt).toLocaleString(language==='ar'?'ar-YE':'en-US')}</td><td>{formatAmount(sale.totalMinor,sale.currencyCode,currencySymbol,language)}</td><td>{tx(paymentLabels[sale.paymentMethod])}</td><td><button type="button" className="btn btn-sm btn-outline-secondary" onClick={()=>void openReceipt(sale.id)} disabled={receiptLoading}>{tx("عرض الإيصال")}</button></td></tr>)}
    {!sales.length&&<tr><td colSpan={5} className="text-center text-secondary py-3">{tx("لا توجد مبيعات حتى الآن.")}</td></tr>}
   </tbody></table></div>}
  </section>

  {receipt&&<section className="pos-receipt-preview mt-3">
   <div className="d-flex flex-wrap align-items-center justify-content-between gap-2 no-print"><h3>إيصال البيع {receipt.sale.reference}</h3><button type="button" className="btn btn-outline-primary" onClick={printReceipt}><i className="bi bi-printer ms-1"/> طباعة الإيصال</button></div>
   <ReceiptContent language={language} receipt={receipt} currencySymbol={currencySymbol} />
  </section>}
  {receipt&&<div className="pos-receipt-print-root" id="posReceiptPrintRoot"><ReceiptContent language={language} receipt={receipt} currencySymbol={currencySymbol}/></div>}
 </section>;
}

function ReceiptContent({language,receipt,currencySymbol}:{language:Language;receipt:POSSaleResult;currencySymbol:string}){
 const tx=(text:string)=>t(language,text);
 return <div className="pos-receipt">
  <header><strong>{tx("إيصال بيع كروت إنترنت")}</strong><span dir="ltr">{receipt.sale.reference}</span></header>
  <div className="pos-receipt-meta"><span>التاريخ: {new Date(receipt.sale.createdAt).toLocaleString('ar')}</span><span>طريقة الدفع: {paymentLabels[receipt.sale.paymentMethod]}</span></div>
  <table className="table table-sm"><thead><tr><th>{tx("اسم المستخدم")}</th><th>{tx("الباقة")}</th><th>{tx("الخادم")}</th><th>{tx("حد الوقت")}</th><th>{tx("السعر")}</th></tr></thead><tbody>{receipt.items.map(item=><tr key={item.id}><td dir="ltr">{item.username}</td><td>{item.productName}</td><td>{item.serverName||'all'}</td><td>{item.limitUptime||'—'}</td><td>{formatAmount(item.unitPriceMinor,receipt.sale.currencyCode,currencySymbol,language)}</td></tr>)}</tbody></table>
  <div className="pos-receipt-total"><span>{tx("الإجمالي")}</span><strong>{formatAmount(receipt.sale.totalMinor,receipt.sale.currencyCode,currencySymbol,language)}</strong></div>
  <p className="pos-receipt-thanks">{tx("شكرًا لثقتكم")}</p>
 </div>;
}
