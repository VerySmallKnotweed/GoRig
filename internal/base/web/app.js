'use strict';
const $ = id => document.getElementById(id);
let assets = [], selectedAsset = null, files = [], selectedFile = null, currentReport = null;
let previewSerial = 0, assetSerial = 0, serverStopped = false;
const labels = {processor:'Процессор',disk:'Физический накопитель',memory:'Модуль памяти',motherboard:'Материнская плата',video:'Видеоконтроллер',network:'Сетевой адаптер',bios:'BIOS',volume:'Логический том',monitor:'Монитор',keyboard:'Клавиатура',mouse:'Указательное устройство',printer:'Принтер',usb:'USB-устройство'};
const attrs = {capacity_bytes:'Объём',free_bytes:'Свободно на момент сбора',speed_mhz:'Скорость, МГц',driver_version:'Версия драйвера',pnp_device_id:'PNP ID',total_memory_bytes:'Память, доступная Windows',installed_memory_bytes:'Установленная память',windows_name:'Редакция Windows',windows_version:'Версия Windows',windows_architecture:'Архитектура Windows',windows_install_date:'Дата установки Windows',windows_device_id:'Код устройства Windows (SQM)',windows_product_id:'Код продукта Windows',hardware_uuid:'Аппаратный UUID',system_type:'Тип системы',touch_support:'Перо и сенсорный ввод',cores:'Физические ядра',logical_processors:'Логические процессоры',max_clock_mhz:'Макс. частота, МГц',interface_type:'Интерфейс',media_type:'Тип носителя (WMI)',mac_address:'MAC-адрес',ip_addresses:'IP-адреса',release_date:'Дата выпуска',video_memory_bytes:'Видеопамять по WMI',filesystem:'Файловая система',volume_type:'Тип тома',printer_driver:'Драйвер',printer_port:'Порт',pnp_class:'Класс устройства',windows_display_version:'Версия Windows',windows_build:'Сборка Windows',screen_width:'Ширина, пикселей',screen_height:'Высота, пикселей'};
const assetStatuses = {in_use:'В работе',reserve:'В резерве',retired:'Выбыл'};
const byteKeys = new Set(['capacity_bytes','free_bytes','total_memory_bytes','installed_memory_bytes','video_memory_bytes']);
function node(tag, text, cls){ const n=document.createElement(tag); if(text!==undefined)n.textContent=text; if(cls)n.className=cls; return n; }
function clear(el){ el.replaceChildren(); }
function displayName(text){return String(text||'').trim().toLowerCase()==='default string'?'N/A':text;}
function isTemplate(x){return !!x&&(x.status==='template'||String(x.value??'').trim().toLowerCase()==='default string');}
function itemValue(x){ return !!(x && x.status==='collected' && !isTemplate(x) && String(x.value ?? '').trim()); }
function v(x){ if(isTemplate(x))return 'N/A'; if(x && x.status==='not_collected')return 'Не собиралось'; if(x && x.status==='error')return 'Ошибка сбора'; if(!itemValue(x))return 'Нет данных'; return String(x.value).trim(); }
function bytes(n){const value=Number(n);if(!Number.isSafeInteger(value)||value<0)return String(n)+' Б';for(const [size,label] of [[1073741824,'гБ'],[1048576,'мБ'],[1024,'кБ']])if(value>=size)return (value/size).toLocaleString('ru-RU',{maximumFractionDigits:2})+' '+label;return value.toLocaleString('ru-RU')+' Б';}
function formatted(x,key){ return byteKeys.has(key)&&itemValue(x)&&/^\d+$/.test(String(x.value))?bytes(x.value):v(x); }
function when(s){ const d=new Date(s); return Number.isNaN(d.valueOf())?String(s||'—'):d.toLocaleString('ru-RU'); }
function kv(container,pairs){const d=node('div',undefined,'kv');for(const [k,val,important] of pairs)d.append(node('span',k),node('span',val,important?'key-value':'') );container.append(d);}
function spec(container,label,text){const row=node('div',undefined,'spec-row');row.append(node('span',label,'spec-label'),node('span',text,'spec-value'));container.append(row);}
function valid(c){return [c.name,c.manufacturer,c.model,c.serial_number,...Object.values(c.attributes||{})].some(itemValue);}
function byType(r,type){return (r.device?.components||[]).filter(c=>c.type===type && valid(c));}
function shortComponent(c){return itemValue(c.model)?v(c.model):v(c.name);}
function installedMemory(r){
  const recorded=r.device?.attributes?.installed_memory_bytes;
  if(itemValue(recorded))return formatted(recorded,'installed_memory_bytes')+(String(recorded.source||'').includes('сумма модулей')?' (сумма обнаруженных модулей)':'');
  const modules=(r.device?.components||[]).filter(c=>c.type==='memory');
  if(!modules.length||modules.some(c=>!itemValue(c.attributes?.capacity_bytes)||!/^\d+$/.test(String(c.attributes.capacity_bytes.value))))return 'Нет данных';
  let total=0;for(const m of modules){const n=Number(m.attributes.capacity_bytes.value);if(!Number.isSafeInteger(n)||n<=0||!Number.isSafeInteger(total+n))return 'Нет данных';total+=n;}
  return bytes(total)+' (сумма обнаруженных модулей)';
}
function warnings(r){const problems=[];for(const c of r.device?.components||[])if(!valid(c))problems.push((labels[c.type]||c.type||'Компонент')+': нет данных или ошибка запроса');if(!itemValue(r.device?.attributes?.windows_install_date))problems.push('дата установки Windows недоступна или не собиралась');if(installedMemory(r)==='Нет данных')problems.push('установленный объём памяти не определён');return [...new Set(problems)];}
function diskKind(c){
  const iface=itemValue(c.attributes?.interface_type)?v(c.attributes.interface_type).toUpperCase():'',pnp=itemValue(c.attributes?.pnp_device_id)?v(c.attributes.pnp_device_id).toUpperCase():'',media=itemValue(c.attributes?.media_type)?v(c.attributes.media_type).toLowerCase():'';
  if(iface==='USB'||iface.startsWith('USB ')||pnp.startsWith('USB\\')||pnp.startsWith('USBSTOR\\'))return 'usb';
  if(media.includes('removable')||media.includes('съёмн')||media.includes('съемн'))return 'removable';
  return iface&&!['UNKNOWN','UNSPECIFIED','OTHER','N/A'].includes(iface)?'internal':'unknown';
}
function fixedDisks(r){return byType(r,'disk').filter(c=>diskKind(c)==='internal');}
function dateParts(x){
  if(!itemValue(x))return null;const m=/^(\d{4})-(\d{2})-(\d{2})$/.exec(String(x.value).trim());if(!m)return null;
  const y=Number(m[1]),month=Number(m[2]),day=Number(m[3]);const d=new Date(Date.UTC(y,month-1,day));return d.getUTCFullYear()===y&&d.getUTCMonth()===month-1&&d.getUTCDate()===day?d:null;
}
function formatInstallDate(x){const d=dateParts(x);return d?[String(d.getUTCDate()).padStart(2,'0'),String(d.getUTCMonth()+1).padStart(2,'0'),d.getUTCFullYear()].join('.'):(itemValue(x)?'Нет данных':v(x));}
function addCalendarMonths(d,n){const y=d.getUTCFullYear(),m=d.getUTCMonth()+n,day=d.getUTCDate(),first=new Date(Date.UTC(y,m,1)),last=new Date(Date.UTC(first.getUTCFullYear(),first.getUTCMonth()+1,0)).getUTCDate();return new Date(Date.UTC(first.getUTCFullYear(),first.getUTCMonth(),Math.min(day,last)));}
function ageSince(x,now=new Date()){
  const start=dateParts(x);if(!start)return 'Нет данных';const today=new Date(Date.UTC(now.getFullYear(),now.getMonth(),now.getDate()));if(start>today)return 'Дата установки в будущем';
  let months=(today.getUTCFullYear()-start.getUTCFullYear())*12+today.getUTCMonth()-start.getUTCMonth();if(addCalendarMonths(start,months)>today)months--;const days=Math.round((today-addCalendarMonths(start,months))/86400000),years=Math.floor(months/12);months%=12;
  const unit=(n,forms)=>n+' '+forms[russianPlurals.select(n)];return [unit(years,{one:'год',few:'года',many:'лет',other:'года'}),unit(months,{one:'месяц',few:'месяца',many:'месяцев',other:'месяца'}),unit(days,{one:'день',few:'дня',many:'дней',other:'дня'})].join(' ');
}
function diskSummary(r){const disks=fixedDisks(r),unknown=byType(r,'disk').some(c=>diskKind(c)==='unknown');return (disks.length?disks.map(c=>shortComponent(c)+' · '+formatted(c.attributes?.capacity_bytes,'capacity_bytes')).join('\n'):'Нет данных')+(unknown?'\nНакопители с неизвестным подключением — в подробностях.':'');}
function pairsText(title,pairs){return title+'\n'+pairs.map(([k,val])=>k+': '+val).join('\n');}
let deviceCopy='',windowsCopy='',componentsCopy='';
async function copyToClipboard(text,status){
  try{
    let done=false;if(navigator.clipboard?.writeText){try{await navigator.clipboard.writeText(text);done=true;}catch{}}
    if(!done){const area=node('textarea');area.value=text;area.setAttribute('readonly','');area.style.position='fixed';area.style.left='-9999px';document.body.append(area);area.select();done=document.execCommand('copy');area.remove();}
    if(!done)throw new Error('Браузер не разрешил доступ к буферу обмена.');status.textContent='Скопировано';
  }catch(e){status.textContent='';notify('Не удалось скопировать: '+e.message);}
}
$('copy-device').onclick=()=>copyToClipboard(deviceCopy,$('copy-device-status'));
$('copy-windows').onclick=()=>copyToClipboard(windowsCopy,$('copy-windows-status'));
$('copy-components').onclick=()=>copyToClipboard(componentsCopy,$('copy-components-status'));
function showReport(r,origin,canImport){
  currentReport=r;$('empty-report').hidden=true;$('report-content').hidden=false;$('report-origin').hidden=false;$('report-origin').textContent=origin;
  const d=r.device||{},at=d.attributes||{},processors=byType(r,'processor'),video=byType(r,'video');
  $('device-name').textContent=v(d.computer_name);$('device-subtitle').textContent=[itemValue(d.manufacturer)?v(d.manufacturer):'',itemValue(d.model)?v(d.model):''].filter(Boolean).join(' · ')||'Сведения из обследования';
  const a=assets.find(x=>x.id===selectedAsset);$('device-state').textContent=origin==='Из базы'&&a?(assetStatuses[a.status]||a.status):'ОТЧЁТ';
  $('report-meta').textContent='Обследование: '+when(r.collected_at)+'  ·  Коллектор: '+(r.collector_version||'—')+'  ·  ID: '+(r.scan_id||'—');
  const cpu=processors.length?processors.map(c=>itemValue(c.name)?v(c.name):v(c.model)).join('\n'):'Нет данных';
  const gpu=video.length?video.map(c=>v(c.name)+(itemValue(c.attributes?.video_memory_bytes)?' · '+formatted(c.attributes.video_memory_bytes,'video_memory_bytes')+' (WMI)':'')).join('\n'):'Нет данных';
  const memory='Всего: '+installedMemory(r)+' / доступно Windows: '+formatted(at.total_memory_bytes,'total_memory_bytes');
  const devicePairs=[['Имя устройства',v(d.computer_name)],['Процессор',cpu],['Оперативная память',memory],['Видеоадаптер',gpu],['Накопители (HDD/SSD, без USB)',diskSummary(r)],['Код устройства',v(at.windows_device_id)],['Код продукта',v(at.windows_product_id)],['Тип системы',[itemValue(at.windows_architecture)?v(at.windows_architecture):'',itemValue(at.system_type)?v(at.system_type):''].filter(Boolean).join(' · ')||'Нет данных'],['Перо и сенсорный ввод',v(at.touch_support)]];
  const version=itemValue(at.windows_display_version)?v(at.windows_display_version):'Нет данных';let build=v(at.windows_build);
  if(!itemValue(at.windows_build)&&itemValue(at.windows_version)){const match=/^\d+\.\d+\.(\d+)(?:\.|$)/.exec(v(at.windows_version));if(match)build=match[1];}
  const windowsPairs=[['Выпуск',itemValue(at.windows_name)?v(at.windows_name):v(d.os)],['Версия',version],['Дата установки',formatInstallDate(at.windows_install_date)],['Работает',ageSince(at.windows_install_date)],['Сборка',build]];
  const overview=$('overview-details');clear(overview);overview.hidden=!canImport;for(const [k,val] of devicePairs.slice(0,5))spec(overview,k,val);
  const problems=warnings(r);$('quality').hidden=!problems.length;$('quality').textContent='Неполный сбор: '+problems.join('; ')+'. Подробности — в исходных данных.';
  clear($('device'));kv($('device'),devicePairs);deviceCopy=pairsText('Характеристики устройства',devicePairs);
  clear($('windows-details'));kv($('windows-details'),windowsPairs);windowsCopy=pairsText('Характеристики WINDOWS',windowsPairs);
  for(const id of ['copy-device-status','copy-windows-status','copy-components-status'])$(id).textContent='';
  $('device-accordion').open=!canImport;$('windows-accordion').open=false;$('components-accordion').open=false;
  clear($('components'));const order=['processor','memory','disk','removable','disk-unknown','volume','video','network','motherboard','bios','monitor','keyboard','mouse','printer','usb'];
  const all=d.components||[],seen=new Set();const categories=new Map();
  for(const c of all){
    let type=c.type||'unknown',pnp=itemValue(c.attributes?.pnp_device_id)?v(c.attributes.pnp_device_id).toUpperCase():'';
    if(type==='disk'){const kind=diskKind(c);type=kind==='usb'?'usb':kind==='removable'?'removable':kind==='unknown'?'disk-unknown':'disk';}
    else if(type==='usb'||pnp.startsWith('USB\\')||pnp.startsWith('USBSTOR\\'))type='usb';
    if(type==='usb'&&pnp){if(seen.has(pnp))continue;seen.add(pnp);}
    if(!categories.has(type))categories.set(type,[]);categories.get(type).push(c);
  }
  const names={...labels,'disk-unknown':'Накопитель — подключение не определено',removable:'Съёмные накопители',usb:'USB-устройства'};
  const extraPairs=[['Производитель',v(d.manufacturer)],['Модель',v(d.model)],['Серийный номер',v(d.serial_number)]];
  const shownKeys=new Set(['installed_memory_bytes','total_memory_bytes','windows_device_id','windows_product_id','windows_architecture','system_type','touch_support','windows_name','windows_display_version','windows_install_date','windows_build']);
  for(const [k,x] of Object.entries(at))if(!shownKeys.has(k))extraPairs.push([attrs[k]||k,formatted(x,k)]);
  const types=[...categories.keys()].sort((a,b)=>{const ai=order.indexOf(a),bi=order.indexOf(b);return (ai<0?999:ai)-(bi<0?999:bi)||a.localeCompare(b);});let copies=[];
  const extraGroup=node('section',undefined,'component-group');extraGroup.dataset.group='system';extraGroup.append(node('h3','Дополнительные сведения о компьютере','detail-section-title'));const extraBox=node('div',undefined,'detail-card');kv(extraBox,extraPairs);const extraText=pairsText('Дополнительные сведения о компьютере',extraPairs),extraButton=node('button','Копировать','btn btn-outline-secondary btn-sm device-copy'),extraStatus=node('span','','copy-status');extraButton.type='button';extraStatus.setAttribute('role','status');extraButton.onclick=()=>copyToClipboard(extraText,extraStatus);extraBox.append(extraButton,extraStatus);extraGroup.append(extraBox);$('components').append(extraGroup);copies.push(extraText);
  if(!types.length)$('components').append(node('p','Компоненты в отчёте отсутствуют.','subtle'));
  for(const type of types){
    const group=node('section',undefined,'component-group');group.dataset.group=type;group.append(node('h3',names[type]||type,'detail-section-title'));const grid=node('div',undefined,'detail-grid');
    for(const c of categories.get(type)){
      const title=itemValue(c.name)?v(c.name):itemValue(c.model)?v(c.model):(labels[c.type]||names[type]||type),box=node('div',undefined,'detail-card');box.append(node('strong',title));
      const pairs=[['Производитель',v(c.manufacturer)],['Модель',v(c.model)],['Серийный номер',v(c.serial_number)]];for(const [k,x] of Object.entries(c.attributes||{}))pairs.push([attrs[k]||k,formatted(x,k)]);
      if(!valid(c))box.append(node('span','Данные недоступны или запрос завершился ошибкой.','subtle'));else kv(box,pairs);
      const text=pairsText((names[type]||type)+' — '+title,pairs);copies.push(text);const copy=node('button','Копировать','btn btn-outline-secondary btn-sm device-copy');copy.type='button';copy.setAttribute('aria-label','Копировать: '+title);const status=node('span','','copy-status');status.setAttribute('role','status');copy.onclick=()=>copyToClipboard(text,status);box.append(copy,status);grid.append(box);
    }group.append(grid);$('components').append(group);
  }
  componentsCopy=copies.length?copies.join('\n\n'):'Компоненты в отчёте отсутствуют.';
  $('raw').textContent=JSON.stringify(r,null,2);
}
let organizations=[], activeOrg=0, currentPage='', lastHash='', pendingHash='', allowLeave=false, leaveConfirmed=false, leaveAfterHide='';
async function api(path,opts){const r=await fetch(path,opts);if(!r.ok)throw new Error((await r.text()).trim()||('HTTP '+r.status));return r.json();}
function jsonPost(obj,method='POST'){return {method,headers:{'Content-Type':'application/json'},body:JSON.stringify(obj)};}
const messageQueue=[];let messageShowing=false;
function notify(text,kind='danger'){messageQueue.push({text,kind});showNextMessage();}
function showNextMessage(){if(messageShowing||!messageQueue.length)return;messageShowing=true;const msg=messageQueue.shift();$('message-title').textContent=msg.kind==='success'?'Готово':'Сообщение';$('message-text').textContent=msg.text;bootstrap.Modal.getOrCreateInstance($('message-modal')).show();}
$('message-modal').addEventListener('hidden.bs.modal',()=>{messageShowing=false;showNextMessage();});
const russianPlurals = new Intl.PluralRules('ru-RU');
function computerCount(n){const forms={one:'компьютер',few:'компьютера',many:'компьютеров',other:'компьютера'};return n+' '+forms[russianPlurals.select(n)];}
function orgName(id){return organizations.find(o=>o.id===id)?.name||(id?'Организация не найдена':'Непривязанные');}
function fillOrganizations(){
  for(const select of document.querySelectorAll('.org-select')){const old=select.value;clear(select);select.append(new Option('Непривязанные','0'));for(const o of organizations)select.append(new Option(o.name,String(o.id)));select.value=[...select.options].some(x=>x.value===old)?old:'0';}
  const menu=$('org-nav');clear(menu);for(const o of organizations){const a=node('a',o.name);a.href='#/organizations/'+o.id;a.title=o.name;menu.append(a);}const unassigned=node('a','Непривязанные');unassigned.href='#/organizations/0';menu.append(unassigned);highlightSubmenu();
}
function highlightSubmenu(){const hash=location.hash||'#/overview';for(const a of document.querySelectorAll('.nav-submenu a')){if(a.getAttribute('href')===hash)a.setAttribute('aria-current','page');else a.removeAttribute('aria-current');}}
function stats(){
  $('stat-all').textContent=assets.length;$('stat-bar-all').style.width=assets.length?'100%':'0%';
  for(const key of ['in_use','reserve','retired']){const n=assets.filter(a=>a.status===key).length;$('stat-'+key).textContent=n;$('stat-bar-'+key).style.width=(assets.length?n/assets.length*100:0)+'%';}
}
function renderOverview(){
  stats();
  for(const root of [$('organization-cards'),$('all-organization-cards')]){
    clear(root);
    for(const o of [...organizations,{id:0,name:'Непривязанные'}]){
      const members=assets.filter(a=>a.organization_id===o.id);
      const card=node('a',undefined,'organization-card');card.href='#/organizations/'+o.id;
      const head=node('div',undefined,'org-card-head'),icon=node('span',o.id?undefined:'▤','org-card-icon');if(o.id)icon.append(document.querySelector('.nav-group-link[href="#/organizations"] svg').cloneNode(true));head.append(icon,node('span','Открыть →','org-card-link'));
      card.append(head,node('h3',o.name),node('p',computerCount(members.length),'org-total'));
      const breakdown=node('div',undefined,'org-breakdown');
      for(const k of ['all','in_use','reserve','retired']){
        const count=k==='all'?members.length:members.filter(a=>a.status===k).length, label=(k==='all'?'Всего':assetStatuses[k])+': '+count;
        const part=node('span',String(count),'org-metric metric-'+k);part.title=label;part.setAttribute('role','img');part.setAttribute('aria-label',label);breakdown.append(part);
      }
      card.append(breakdown);root.append(card);
    }
  }
}
const deviceTypes={laptop:'Ноутбук',desktop:'Компьютер',all_in_one:'Моноблок',other:'Другое','':'Тип не указан'};
const devicePaths={laptop:'M4 4h16v11H4z M2 19h20 M4 15l-2 4m18-4 2 4',desktop:'M3 4h13v11H3z M9.5 15v4m-4 0h8 M19 4h3v15h-3z M20.5 7h.01',all_in_one:'M3 4h18v12H3z M12 16v3m-4 0h8',other:'M12 3 21 8v8l-9 5-9-5V8z M3 8l9 5 9-5 M12 13v8','':'M4 4h16v12H4z M12 16v3m-4 0h8'};
function deviceIcon(type){const span=node('span',undefined,'pc-icon'),ns='http://www.w3.org/2000/svg',svg=document.createElementNS(ns,'svg'),path=document.createElementNS(ns,'path');svg.setAttribute('viewBox','0 0 24 24');svg.setAttribute('fill','none');svg.setAttribute('stroke','currentColor');svg.setAttribute('stroke-width','1.7');svg.setAttribute('stroke-linecap','round');svg.setAttribute('stroke-linejoin','round');svg.setAttribute('aria-hidden','true');path.setAttribute('d',devicePaths[type]||devicePaths['']);svg.append(path);span.append(svg);span.title=deviceTypes[type]||deviceTypes[''];span.setAttribute('role','img');span.setAttribute('aria-label',span.title);return span;}
function renderOrganization(id){
  activeOrg=id;const org=organizations.find(x=>x.id===id);if(id&&!org){notify('Организация не найдена.');location.hash='#/overview';return;}
  $('org-title').textContent=orgName(id);const members=assets.filter(a=>a.organization_id===id);$('org-subtitle').textContent=computerCount(members.length);
  const shown=members.filter(a=>$('filter').value==='all'||a.status===$('filter').value);$('org-count').textContent=shown.length+' / '+members.length;
  const root=$('asset-list');clear(root);if(!shown.length){root.append(node('div','В этой категории пока нет компьютеров.','surface subtle'));return;}
  for(const a of shown){const card=node('a',undefined,'pc-card pc-state-'+a.status+' '+(a.cpu_vendor==='intel'?'pc-intel':a.cpu_vendor==='amd'?'pc-amd':'pc-unknown'));card.href='#/asset/'+a.id;card.setAttribute('aria-label',displayName(a.name)+(a.identification?', '+a.identification:'')+', '+(deviceTypes[a.device_type]||deviceTypes[''])+' — '+(assetStatuses[a.status]||a.status)+'. Подробнее');card.title=assetStatuses[a.status]||a.status;
    const top=node('div',undefined,'pc-card-top');top.append(deviceIcon(a.device_type||''),node('span',a.cpu_vendor==='intel'?'INTEL':a.cpu_vendor==='amd'?'AMD':'ПРОЦЕССОР НЕ ОПРЕДЕЛЁН','cpu-badge'));card.append(top,node('h3',displayName(a.name)),node('p',a.cpu?displayName(a.cpu):'Процессор: нет данных','pc-cpu'));if(a.identification){const tag=node('p',a.identification,'pc-identification');tag.title=a.identification;card.append(tag);}
    const foot=node('div',undefined,'pc-card-foot');foot.append(node('span','ПОДРОБНЕЕ'));card.append(foot);root.append(card);
  }
}
function renderSettings(){
  const root=$('org-settings-list');clear(root);
  if(!organizations.length){root.append(node('p','Организаций пока нет.','subtle py-3'));return;}
  for(const o of organizations){
    const row=node('div',undefined,'org-setting-row');const name=node('div',undefined,'org-setting-name');
    name.append(node('strong',o.name),node('small',computerCount(assets.filter(a=>a.organization_id===o.id).length)));
    const edit=node('button','Переименовать','btn btn-outline-secondary btn-sm');edit.type='button';edit.onclick=()=>openOrgModal('rename',o,edit);
    const del=node('button','Удалить','btn btn-outline-danger btn-sm');del.type='button';del.onclick=()=>openOrgModal('delete',o,del);
    row.append(name,edit,del);root.append(row);
  }
}
let modalOrg=null, modalTrigger=null, modalBusy=false;
function openOrgModal(kind,org,trigger){
  modalOrg=org;modalTrigger=trigger;
  if(kind==='rename'){$('rename-org-input').value=org.name;$('rename-org-error').textContent='';}
  else {$('delete-org-name').textContent='«'+org.name+'»';$('delete-org-error').textContent='';}
  bootstrap.Modal.getOrCreateInstance($(kind+'-org-modal')).show();
}
$('rename-org-modal').addEventListener('shown.bs.modal',()=>{$('rename-org-input').focus();$('rename-org-input').select();});
for(const kind of ['rename','delete']){
  const element=$(kind+'-org-modal');
  element.addEventListener('hide.bs.modal',e=>{if(modalBusy)e.preventDefault();});
  element.addEventListener('hidden.bs.modal',()=>{const target=modalTrigger?.isConnected?modalTrigger:$('org-name');target.focus();modalOrg=null;modalTrigger=null;});
  $(kind+'-org-form').onsubmit=async e=>{
    e.preventDefault();if(modalBusy||!modalOrg)return;
    const error=$(kind+'-org-error'),button=$(kind+'-org-confirm');error.textContent='';
    const name=$('rename-org-input').value.trim();if(kind==='rename'&&!name){error.textContent='Укажите название организации.';$('rename-org-input').focus();return;}
    modalBusy=true;button.disabled=true;
    try{
      await api('/api/organizations/'+modalOrg.id,jsonPost(kind==='rename'?{name}:{},kind==='rename'?'PUT':'DELETE'));
      await reloadData();$('org-message').textContent=kind==='rename'?'Название обновлено.':'Организация удалена, компьютеры сохранены.';
      modalBusy=false;bootstrap.Modal.getInstance(element).hide();
    }catch(err){error.textContent='Ошибка: '+err.message;}
    finally{modalBusy=false;button.disabled=false;}
  };
}
$('org-form').onsubmit=async e=>{e.preventDefault();try{await api('/api/organizations',jsonPost({name:$('org-name').value}));$('org-name').value='';await reloadData();$('org-message').textContent='Организация создана.';}catch(err){$('org-message').textContent='Ошибка: '+err.message;}};
async function reloadData(){[organizations,assets]=await Promise.all([api('/api/organizations'),api('/api/assets')]);fillOrganizations();renderOverview();renderSettings();renderReportChoices();const target=$('target'),old=target.value;clear(target);target.append(new Option('— Выберите действие —',''),new Option('Новый компьютер','0'));for(const a of assets)target.append(new Option(a.name+' (ID '+a.id+', '+orgName(a.organization_id)+')',String(a.id)));target.value=old;if(currentPage==='organization')renderOrganization(activeOrg);}
$('filter').onchange=()=>renderOrganization(activeOrg);
for(const [button,menu] of [['org-toggle','org-nav'],['reports-toggle','reports-nav']]){
  $(button).onclick=()=>{const el=$(menu);el.hidden=!el.hidden;$(button).setAttribute('aria-expanded',String(!el.hidden));};
}
function dirty(){const a=assets.find(x=>x.id===selectedAsset);return !serverStopped&&currentPage==='asset'&&a&&(a.notes!==$('notes').value||a.identification!==$('asset-identification').value||a.device_type!==$('asset-device-type').value||a.status!==$('asset-status').value||String(a.organization_id)!==$('asset-org').value);}
window.addEventListener('beforeunload',e=>{if(dirty()){e.preventDefault();e.returnValue='';}});
function setAssetIdentification(text){$('asset-page-identification').textContent=text;$('asset-page-identification').hidden=!text;}
async function openAsset(id){
  const token=++assetSerial;const a=assets.find(x=>x.id===id);if(!a){notify('Компьютер не найден.');location.hash='#/overview';return;}
  selectedAsset=id;$('asset-page-title').textContent=displayName(a.name);setAssetIdentification(a.identification);$('asset-identification').value=a.identification;$('asset-device-type').value=a.device_type;$('asset-detail').open=false;$('notes').value=a.notes;$('asset-status').value=a.status;$('asset-org').value=String(a.organization_id);$('notes-status').textContent='';$('back-org').href='#/organizations/'+a.organization_id;
  $('empty-report').hidden=false;$('report-content').hidden=true;$('report-origin').hidden=true;clear($('history'));
  try{const hist=await api('/api/assets/'+id);if(token!==assetSerial||currentPage!=='asset')return;if(!hist.length)$('history').append(node('p','История пуста.','subtle'));for(const h of hist){const b=node('button',when(h.collected_at),'history-btn');b.type='button';b.title='Импортирован: '+when(h.imported_at);b.onclick=()=>showReport(h.report,'Из базы',false);$('history').append(b);}if(hist.length)showReport(hist[0].report,'Из базы',false);}catch(e){notify('Не удалось открыть компьютер: '+e.message);}
}
$('save-notes').onclick=async()=>{
  const id=selectedAsset;if(id===null||$('save-notes').disabled)return;
  const details={notes:$('notes').value,status:$('asset-status').value,organization_id:Number($('asset-org').value),identification:$('asset-identification').value,device_type:$('asset-device-type').value};
  $('save-notes').disabled=true;
  try{await api('/api/assets/'+id,jsonPost(details,'PUT'));await reloadData();if(selectedAsset===id){const updated=assets.find(a=>a.id===id);if(updated){setAssetIdentification(updated.identification);if($('report-origin').textContent==='Из базы')$('device-state').textContent=assetStatuses[updated.status]||updated.status;if($('asset-identification').value===details.identification)$('asset-identification').value=updated.identification;$('back-org').href='#/organizations/'+updated.organization_id;}$('notes-status').textContent=dirty()?'Сохранено; есть новые несохранённые изменения.':'Сохранено';}}
  catch(e){$('notes-status').textContent='Ошибка: '+e.message;}finally{$('save-notes').disabled=false;}
};
let deleteAssetID=null,deleteBusy=false,deleteDestination=null,deleteNotice='';
const deleteAssetModal=$('delete-asset-modal');
$('delete-asset').onclick=()=>{const a=assets.find(x=>x.id===selectedAsset);if(!a)return;deleteAssetID=a.id;deleteDestination=null;$('delete-asset-name').textContent='«'+displayName(a.name)+'»';$('delete-asset-error').textContent='';deleteNotice='';bootstrap.Modal.getOrCreateInstance(deleteAssetModal).show();};
deleteAssetModal.addEventListener('hide.bs.modal',e=>{if(deleteBusy)e.preventDefault();});
deleteAssetModal.addEventListener('hidden.bs.modal',()=>{if(deleteDestination!==null){const dest=deleteDestination;deleteDestination=null;location.hash=dest;if(deleteNotice)notify(deleteNotice);}else if(currentPage==='asset')$('delete-asset').focus();deleteAssetID=null;});
$('delete-asset-confirm').onclick=async()=>{if(deleteBusy||deleteAssetID===null)return;const a=assets.find(x=>x.id===deleteAssetID);if(!a)return;deleteBusy=true;$('delete-asset-confirm').disabled=true;$('delete-asset-error').textContent='';try{
  await api('/api/assets/'+a.id,jsonPost({},'DELETE'));selectedAsset=null;
  try{await reloadData();}catch(e){assets=assets.filter(x=>x.id!==a.id);renderOverview();deleteNotice='Компьютер удалён, но обновить списки не удалось: '+e.message+'. Обновите страницу.';}
  deleteDestination='#/organizations/'+a.organization_id;deleteBusy=false;bootstrap.Modal.getInstance(deleteAssetModal).hide();
}catch(e){$('delete-asset-error').textContent='Ошибка удаления: '+e.message;}finally{deleteBusy=false;$('delete-asset-confirm').disabled=false;}};
function chooseFiles(list){files=[...list].filter(f=>f.name.toLowerCase().endsWith('.json')).sort((a,b)=>(a.webkitRelativePath||a.name).localeCompare(b.webkitRelativePath||b.name));selectedFile=null;currentReport=null;previewSerial++;$('empty-report').hidden=false;$('report-content').hidden=true;$('report-origin').hidden=true;$('import-panel').hidden=true;$('file-error').textContent=files.length?'':'JSON-файлы не найдены.';renderFiles();}
$('file-input').onchange=e=>{chooseFiles(e.target.files);e.target.value='';};$('folder-input').onchange=e=>{chooseFiles(e.target.files);e.target.value='';};
function renderFiles(){clear($('files'));for(const f of files){const b=node('button',f.webkitRelativePath||f.name,'asset file'+(f===selectedFile?' active':''));b.type='button';b.onclick=()=>openFile(f);$('files').append(b);}}
async function openFile(f){selectedFile=f;currentReport=null;previewSerial++;renderFiles();$('file-error').textContent='';$('empty-report').hidden=false;$('report-content').hidden=true;$('report-origin').hidden=true;$('import-panel').hidden=true;
  try{if(f.size>7*1024*1024)throw new Error('Файл больше 7 мБ');const r=JSON.parse(await f.text());if(f!==selectedFile)return;if(!r||r.schema_version!==1||!r.scan_id||!r.collected_at||!r.device||!Array.isArray(r.device.components))throw new Error('Это не поддерживаемый отчёт GoRig версии 1');showReport(r,f.name,true);$('import-panel').hidden=false;$('target').value='';$('import-status').value='in_use';$('import-org').value='0';$('import-notes').value='';$('import-identification').value='';$('import-device-type').value='';$('preview').hidden=true;clear($('diff'));$('import').disabled=true;}catch(e){if(f!==selectedFile)return;selectedFile=null;currentReport=null;renderFiles();$('file-error').textContent='Не удалось открыть файл: '+e.message;}}
let importBusy=false;
const statusText={new:'Новый компьютер. Проверьте отчёт и нажмите «Добавить в базу».',changed:'Найдены отличия от последнего сохранённого обследования.',unchanged:'Отличий среди сравниваемых данных нет. Новое обследование не будет добавлено.',duplicate:'Этот отчёт уже есть в базе (компьютер ID ',older:'Отчёт старше последнего сохранённого обследования. Для защиты текущего состояния он не будет добавлен.'};
async function runPreview(){const token=++previewSerial;$('import').disabled=true;clear($('diff'));if(!currentReport||$('target').value===''){$('preview').hidden=true;return;}$('preview').hidden=false;$('preview').className='preview-status mt-3';$('preview').textContent='Сравнение…';try{const p=await api('/api/preview',jsonPost({asset_id:Number($('target').value),report:currentReport}));if(token!==previewSerial)return;$('preview').className='preview-status mt-3 '+(['new','changed'].includes(p.status)?'preview-success':'preview-warning');$('preview').textContent=statusText[p.status]+(p.status==='duplicate'?p.duplicate_asset_id+').':'')+(p.latest_at?'\nПоследнее сохранённое обследование: '+when(p.latest_at):'');$('import').disabled=importBusy||!['new','changed'].includes(p.status);if(p.differences?.length){const t=node('table',undefined,'table table-sm align-middle');const head=node('tr');for(const s of ['Поле','В базе','В отчёте'])head.append(node('th',s));t.append(head);for(const d of p.differences){const tr=node('tr');for(const s of [d.field,d.before||'Нет данных',d.after||'Нет данных'])tr.append(node('td',s));t.append(tr);}$('diff').append(t);}}catch(e){if(token!==previewSerial)return;$('preview').className='preview-status preview-error mt-3';$('preview').textContent='Ошибка: '+e.message;}}
$('target').onchange=()=>{const a=assets.find(x=>String(x.id)===$('target').value);$('import-status').value=a?.status||'in_use';$('import-org').value=String(a?.organization_id||0);$('import-notes').value=a?.notes||'';$('import-identification').value=a?.identification||'';$('import-device-type').value=a?.device_type||'';runPreview();};
$('import').onclick=async()=>{
  const target=$('target').value,report=currentReport,file=selectedFile;if(importBusy||!report||target===''||$('import').disabled)return;
  const details={notes:$('import-notes').value,status:$('import-status').value,organization_id:Number($('import-org').value),identification:$('import-identification').value,device_type:$('import-device-type').value};
  importBusy=true;previewSerial++;$('import').disabled=true;
  try{
    const res=await api('/api/import',jsonPost({asset_id:Number(target),report,details}));
    if(!['new','changed'].includes(res.preview.status))return;
    let refreshError='';try{await reloadData();}catch(e){refreshError=e.message;}
    const sameFile=currentReport===report&&selectedFile===file;
    if(sameFile){selectedFile=null;currentReport=null;$('import-panel').hidden=true;}
    if(sameFile&&!refreshError&&currentPage==='import')location.hash='#/asset/'+res.asset_id;
    notify('Отчёт добавлен в базу.'+(sameFile?'':' Компьютер ID '+res.asset_id+'.')+(refreshError?' Не удалось обновить списки: '+refreshError+'. Обновите страницу.':''),'success');
  }catch(e){notify('Ошибка импорта: '+e.message);}
  finally{importBusy=false;if(currentPage==='import'&&currentReport)runPreview();}
};
let shutdownBusy=false;
const shutdownModal=$('shutdown-modal');
$('shutdown').onclick=()=>{if(serverStopped)return;$('shutdown-error').textContent='';$('shutdown-dirty').hidden=!dirty();bootstrap.Modal.getOrCreateInstance(shutdownModal).show();};
shutdownModal.addEventListener('shown.bs.modal',()=>shutdownModal.querySelector('[data-bs-dismiss="modal"]').focus());
shutdownModal.addEventListener('hide.bs.modal',e=>{if(shutdownBusy)e.preventDefault();});
shutdownModal.addEventListener('hidden.bs.modal',()=>{if(serverStopped){document.querySelector('.app-shell').hidden=true;$('stopped-screen').hidden=false;document.title='GoRig Base остановлен';$('stopped-screen').querySelector('h1').focus();}else $('shutdown').focus();});
$('shutdown-form').onsubmit=async e=>{e.preventDefault();if(shutdownBusy||serverStopped)return;shutdownBusy=true;$('shutdown-confirm').disabled=true;$('shutdown-error').textContent='';try{await api('/api/shutdown',jsonPost({}));serverStopped=true;shutdownBusy=false;bootstrap.Modal.getInstance(shutdownModal).hide();}catch(err){$('shutdown-error').textContent='Не удалось остановить сервер: '+err.message;}finally{shutdownBusy=false;$('shutdown-confirm').disabled=false;}};
// Report data are loaded as a single database snapshot, never from stale cards.
let reportSerial=0;
function renderReportChoices(){
  const root=$('report-organizations');clear(root);
  for(const o of [...organizations,{id:0,name:'Непривязанные'}]){
    const b=node('button',o.name,'btn btn-outline-secondary report-org-button');b.type='button';b.onclick=()=>generatePrintableReport(o.id);root.append(b);
  }
}
async function generatePrintableReport(id){
  const serial=++reportSerial,root=$('printable-report');root.hidden=true;$('report-actions').hidden=true;document.body.classList.remove('report-print-mode');$('report-message').textContent='Подготовка отчёта…';
  try{
    const data=await api('/api/print-report'+(id===null?'':'?organization_id='+encodeURIComponent(id)));
    if(serial!==reportSerial)return;
    clear(root);root.append(node('h1',id===null?'Сводный отчёт по оборудованию':'Отчёт по организации'));
    root.append(node('p','Всего: '+computerCount(data.assets.length),'print-summary'),node('p','Сформирован: '+when(new Date().toISOString())));
    for(const org of data.organizations){
      const group=node('section',undefined,'print-org');const members=data.assets.filter(a=>a.organization_id===org.id);
      group.append(node('h2',org.name+' — '+computerCount(members.length)));
      for(const status of ['in_use','reserve','retired']){
        const items=members.filter(a=>a.status===status),section=node('section',undefined,'print-status');
        section.append(node('h3',assetStatuses[status]+' — '+computerCount(items.length)));
        if(!items.length)section.append(node('p','Нет компьютеров.','print-empty'));
        for(const [index,a] of items.entries()){
          const box=node('article',undefined,'print-computer'),report=a.report||{},d=report.device||{};
          box.append(node('h4',(index+1)+'. '+displayName(a.name)));if(a.identification)printLine(box,'Идентификация',a.identification);if(a.device_type)printLine(box,'Тип устройства',deviceTypes[a.device_type]||a.device_type);
          const brief=[itemValue(d.manufacturer)?v(d.manufacturer):'',itemValue(d.model)?v(d.model):'',itemValue(d.os)?v(d.os):''].filter(Boolean).join(' · ')||'Нет данных';
          printLine(box,'Краткая информация',brief);
          printLine(box,'Заметка',a.notes?.trim()||'Нет заметки');
          const cpu=byType(report,'processor');printLine(box,'Процессор',cpu.length?cpu.map(c=>itemValue(c.name)?v(c.name):v(c.model)).join('; '):'Нет данных');
          printLine(box,'Память',installedMemory(report));
          const disks=fixedDisks(report),unknown=byType(report,'disk').some(c=>diskKind(c)==='unknown');printLine(box,'Накопители',(disks.length?disks.map(c=>(itemValue(c.model)||itemValue(c.name)?shortComponent(c)+' — ':'')+formatted(c.attributes?.capacity_bytes,'capacity_bytes')).join('; '):'Нет данных')+(unknown?'; есть накопители с неопределённым подключением':''));
          section.append(box);
        }
        group.append(section);
      }
      root.append(group);
    }
    const sign=node('div',undefined,'print-signature');sign.append(node('p','Дата: «____» __________________ 20____ г.'),node('p','Подпись: _____________________ / _____________________'));root.append(sign);
    $('report-message').textContent='';root.hidden=false;$('report-actions').hidden=false;if(currentPage==='reports')document.body.classList.add('report-print-mode');
  }catch(e){if(serial!==reportSerial)return;$('report-message').textContent='';notify('Не удалось подготовить отчёт: '+e.message);}
}
function printLine(root,label,value){const p=node('p',undefined,'print-line');p.append(node('strong',label+': '),document.createTextNode(value));root.append(p);}
$('report-all').onclick=()=>generatePrintableReport(null);
$('print-report').onclick=()=>window.print();
document.addEventListener('click',e=>{
  const link=e.target.closest('a[href^="#/"]');if(!link||e.defaultPrevented||e.button!==0||e.metaKey||e.ctrlKey||e.shiftKey||e.altKey||link.target||link.hasAttribute('download'))return;
  const dest=link.getAttribute('href');if(dirty()&&dest!==lastHash){e.preventDefault();pendingHash=dest;bootstrap.Modal.getOrCreateInstance($('leave-modal')).show();}
});
$('leave-confirm').onclick=()=>{leaveConfirmed=true;bootstrap.Modal.getInstance($('leave-modal')).hide();};
$('leave-modal').addEventListener('hide.bs.modal',()=>{leaveAfterHide=leaveConfirmed?pendingHash:'';leaveConfirmed=false;pendingHash='';});
$('leave-modal').addEventListener('hidden.bs.modal',()=>{const dest=leaveAfterHide;leaveAfterHide='';if(dest){pendingHash='';allowLeave=true;location.hash=dest;}else if(pendingHash){bootstrap.Modal.getInstance($('leave-modal')).show();}else document.querySelector('#asset-detail summary')?.focus();});
function route(){
  if(serverStopped)return;
  const hash=location.hash||'#/overview';
  if(lastHash&&hash!==lastHash&&dirty()&&!allowLeave){pendingHash=hash;history.pushState(history.state,'',lastHash);bootstrap.Modal.getOrCreateInstance($('leave-modal')).show();return;}
  allowLeave=false;
  lastHash=hash;assetSerial++;previewSerial++;
  const parts=hash.replace(/^#\/?/,'').split('/');let page=parts[0];
  if(!['overview','organizations','asset','import','reports','settings'].includes(page))page='overview';
  if(page==='organizations'&&parts[1]&&!/^\d+$/.test(parts[1])){notify('Организация не найдена.');location.hash='#/organizations';return;}
  currentPage=page==='organizations'?(parts[1]?'organization':'organizations'):page;
  document.querySelectorAll('.page').forEach(el=>{el.hidden=el.id!=='page-'+currentPage;});
  document.querySelectorAll('[data-nav]').forEach(el=>el.classList.toggle('active',el.dataset.nav===(page==='asset'?'organizations':page)));
  const titles={overview:'Обзор',organizations:'Организации',asset:'Компьютер',import:'Импорт отчёта',reports:'Отчёты',settings:'Настройки'};
  highlightSubmenu();$('breadcrumb').textContent=titles[page];$('breadcrumbs').hidden=['overview','organizations','settings','asset','import'].includes(page);$('top-import').hidden=page!=='overview';
  if(page==='overview'||currentPage==='organizations')renderOverview();
  document.body.classList.toggle('report-print-mode',page==='reports'&&!$('printable-report').hidden);
  if(page==='reports'){renderReportChoices();reportSerial++;$('printable-report').hidden=true;$('report-actions').hidden=true;$('report-message').textContent='';document.body.classList.remove('report-print-mode');}
  if(page==='settings')renderSettings();
  if(currentPage==='organization')renderOrganization(Number(parts[1]));
  if(page==='asset'){$('asset-report-slot').append($('report-panel'));openAsset(Number(parts[1]));}
  if(page==='import'){selectedAsset=null;$('import-report-slot').append($('report-panel'));chooseFiles([]);$('file-error').textContent='';}
  window.scrollTo({top:0,behavior:'auto'});
}
window.addEventListener('hashchange',route);
reloadData().then(route).catch(e=>{route();notify('Не удалось загрузить данные: '+e.message);});
