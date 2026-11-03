'use strict';
const $ = id => document.getElementById(id);
let assets = [], selectedAsset = null, files = [], selectedFile = null, currentReport = null;
let previewSerial = 0, assetSerial = 0, serverStopped = false;
const labels = {processor:'Процессор',disk:'Физический накопитель',memory:'Модуль памяти',motherboard:'Материнская плата',video:'Видеоконтроллер',network:'Сетевой адаптер',bios:'BIOS',volume:'Логический том',monitor:'Монитор',keyboard:'Клавиатура',mouse:'Указательное устройство',printer:'Принтер'};
const attrs = {capacity_bytes:'Объём',free_bytes:'Свободно на момент сбора',speed_mhz:'Скорость, МГц',driver_version:'Версия драйвера',pnp_device_id:'PNP ID',total_memory_bytes:'Память, доступная Windows',installed_memory_bytes:'Установленная память',windows_name:'Редакция Windows',windows_version:'Версия Windows',windows_architecture:'Архитектура Windows',windows_install_date:'Дата установки Windows',windows_device_id:'Код устройства Windows (SQM)',windows_product_id:'Код продукта Windows',hardware_uuid:'Аппаратный UUID',system_type:'Тип системы',touch_support:'Перо и сенсорный ввод',cores:'Физические ядра',logical_processors:'Логические процессоры',max_clock_mhz:'Макс. частота, МГц',interface_type:'Интерфейс',media_type:'Тип носителя (WMI)',mac_address:'MAC-адрес',ip_addresses:'IP-адреса',release_date:'Дата выпуска',video_memory_bytes:'Видеопамять по WMI',filesystem:'Файловая система',volume_type:'Тип тома',printer_driver:'Драйвер',printer_port:'Порт',screen_width:'Ширина, пикселей',screen_height:'Высота, пикселей'};
const assetStatuses = {in_use:'В работе',reserve:'В резерве',retired:'Выбыл'};
const byteKeys = new Set(['capacity_bytes','free_bytes','total_memory_bytes','installed_memory_bytes','video_memory_bytes']);
function node(tag, text, cls){ const n=document.createElement(tag); if(text!==undefined)n.textContent=text; if(cls)n.className=cls; return n; }
function clear(el){ el.replaceChildren(); }
function itemValue(x){ return !!(x && ['collected','template'].includes(x.status) && String(x.value ?? '').trim()); }
function v(x){ if(x && x.status==='not_collected')return 'Не собиралось'; if(x && x.status==='error')return 'Ошибка сбора'; if(!itemValue(x))return 'Нет данных'; return String(x.value).trim()+(x.status==='template'?' (шаблон)':''); }
function bytes(n){ const value=Number(n); if(!Number.isSafeInteger(value)||value<0)return String(n)+' байт'; if(value>=1073741824)return (value/1073741824).toLocaleString('ru-RU',{maximumFractionDigits:2})+' ГиБ'; if(value>=1048576)return (value/1048576).toLocaleString('ru-RU',{maximumFractionDigits:2})+' МиБ'; return value.toLocaleString('ru-RU')+' байт'; }
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
function showReport(r,origin,canImport){
  currentReport=r; $('empty-report').hidden=true; $('report-content').hidden=false; $('report-origin').hidden=false; $('report-origin').textContent=origin;
  const d=r.device||{}, at=d.attributes||{}, processors=byType(r,'processor'),disks=byType(r,'disk'),video=byType(r,'video');
  $('device-name').textContent=v(d.computer_name); $('device-subtitle').textContent=[itemValue(d.manufacturer)?v(d.manufacturer):'',itemValue(d.model)?v(d.model):''].filter(Boolean).join(' · ')||'Сведения из обследования';
  const a=assets.find(x=>x.id===selectedAsset); $('device-state').textContent=origin==='Из базы'&&a?(assetStatuses[a.status]||a.status):'ОТЧЁТ';
  $('report-meta').textContent='Обследование: '+when(r.collected_at)+'  ·  Коллектор: '+(r.collector_version||'—')+'  ·  ID: '+(r.scan_id||'—');
  const overview=$('overview-details');clear(overview);
  spec(overview,'Процессор',processors.length?processors.map(c=>v(c.name)).join('\n'):'Нет данных');
  spec(overview,'Оперативная память',installedMemory(r));
  spec(overview,'Накопители',disks.length?disks.map(c=>shortComponent(c)+'  ·  '+formatted(c.attributes?.capacity_bytes,'capacity_bytes')).join('\n'):'Нет данных');
  spec(overview,'Видеоадаптер',video.length?video.map(c=>v(c.name)+(itemValue(c.attributes?.video_memory_bytes)?' · '+formatted(c.attributes.video_memory_bytes,'video_memory_bytes')+' (WMI)':'')).join('\n'):'Нет данных');
  spec(overview,'Код устройства',v(at.windows_device_id)); spec(overview,'Код продукта',v(at.windows_product_id));
  spec(overview,'Тип системы',[itemValue(at.windows_architecture)?v(at.windows_architecture):'',itemValue(at.system_type)?v(at.system_type):''].filter(Boolean).join(' · ')||'Нет данных');
  spec(overview,'Дата установки Windows',v(at.windows_install_date)); spec(overview,'Перо и сенсорный ввод',v(at.touch_support));
  const problems=warnings(r);$('quality').hidden=!problems.length;$('quality').textContent='Неполный сбор: '+problems.join('; ')+'. Подробности — в исходных данных.';
  clear($('device'));$('device').append(node('h3','Компьютер и Windows','detail-section-title'));
  const entries=[['Имя',v(d.computer_name)],['Производитель',v(d.manufacturer)],['Модель',v(d.model)],['Серийный номер',v(d.serial_number)],['ОС',v(d.os)+(d.os?.raw&&itemValue(d.os)?' — '+d.os.raw:'')],...Object.entries(at).map(([k,x])=>[attrs[k]||k,formatted(x,k)])];kv($('device'),entries);
  clear($('components'));
  const icons={processor:'▦',memory:'▤',disk:'◉',volume:'▣',video:'▧',network:'⌁',motherboard:'▦',bios:'◈',monitor:'▣',keyboard:'⌨',mouse:'◉',printer:'▤'};
  const order=['processor','memory','disk','volume','video','network','motherboard','bios','monitor','keyboard','mouse','printer'];
  const types=[...new Set((d.components||[]).map(c=>c.type||'unknown'))].sort((a,b)=>{const ai=order.indexOf(a),bi=order.indexOf(b);return (ai<0?999:ai)-(bi<0?999:bi)||a.localeCompare(b);});
  if(!types.length)$('components').append(node('p','Компоненты в отчёте отсутствуют.','subtle'));
  for(const type of types){
    const group=node('section',undefined,'component-group');group.append(node('h3',(icons[type]||'◇')+'  '+(labels[type]||type),'detail-section-title'));
    const grid=node('div',undefined,'detail-grid');
    for(const c of (d.components||[]).filter(c=>(c.type||'unknown')===type)){
      const box=node('div',undefined,'detail-card');box.append(node('strong',itemValue(c.name)?v(c.name):(labels[type]||type)));
      if(!valid(c)){box.append(node('span','Данные недоступны или запрос завершился ошибкой.','subtle'));}
      else {const pairs=[['Производитель',v(c.manufacturer)],['Модель',v(c.model)],['Серийный номер',v(c.serial_number)]];for(const [k,x] of Object.entries(c.attributes||{}))pairs.push([(k==='capacity_bytes'?'◫ ':k==='free_bytes'?'◔ ':k==='ip_addresses'||k==='mac_address'?'⌁ ':'')+(attrs[k]||k),formatted(x,k),['capacity_bytes','free_bytes','ip_addresses','mac_address'].includes(k)]);kv(box,pairs);}
      grid.append(box);
    }group.append(grid);$('components').append(group);
  }
  $('raw').textContent=JSON.stringify(r,null,2);
}
let organizations=[], activeOrg=0, currentPage='', lastHash='', reverting=false;
async function api(path,opts){const r=await fetch(path,opts);if(!r.ok)throw new Error((await r.text()).trim()||('HTTP '+r.status));return r.json();}
function jsonPost(obj,method='POST'){return {method,headers:{'Content-Type':'application/json'},body:JSON.stringify(obj)};}
function notify(text,kind='danger'){const el=$('global-message');el.textContent=text;el.className='alert alert-'+kind;}
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
function renderOrganization(id){
  activeOrg=id;const org=organizations.find(x=>x.id===id);if(id&&!org){notify('Организация не найдена.');location.hash='#/overview';return;}
  $('org-title').textContent=orgName(id);const members=assets.filter(a=>a.organization_id===id);$('org-subtitle').textContent=computerCount(members.length);
  const shown=members.filter(a=>$('filter').value==='all'||a.status===$('filter').value);$('org-count').textContent=shown.length+' / '+members.length;
  const root=$('asset-list');clear(root);if(!shown.length){root.append(node('div','В этой категории пока нет компьютеров.','surface subtle'));return;}
  for(const a of shown){const card=node('a',undefined,'pc-card pc-state-'+a.status+' '+(a.cpu_vendor==='intel'?'pc-intel':a.cpu_vendor==='amd'?'pc-amd':'pc-unknown'));card.href='#/asset/'+a.id;card.setAttribute('aria-label',a.name+' — '+(assetStatuses[a.status]||a.status)+'. Подробнее');card.title=assetStatuses[a.status]||a.status;
    const top=node('div',undefined,'pc-card-top');top.append(node('span','▣','pc-icon'),node('span',a.cpu_vendor==='intel'?'INTEL':a.cpu_vendor==='amd'?'AMD':'ПРОЦЕССОР НЕ ОПРЕДЕЛЁН','cpu-badge'));card.append(top,node('h3',a.name),node('p',a.cpu||'Процессор: нет данных','pc-cpu'));
    const foot=node('div',undefined,'pc-card-foot');foot.append(node('span','ПОДРОБНЕЕ →'));card.append(foot);root.append(card);
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
function dirty(){const a=assets.find(x=>x.id===selectedAsset);return currentPage==='asset'&&a&&(a.notes!==$('notes').value||a.status!==$('asset-status').value||String(a.organization_id)!==$('asset-org').value);}
window.addEventListener('beforeunload',e=>{if(dirty()){e.preventDefault();e.returnValue='';}});
async function openAsset(id){
  const token=++assetSerial;const a=assets.find(x=>x.id===id);if(!a){notify('Компьютер не найден.');location.hash='#/overview';return;}
  selectedAsset=id;$('asset-page-title').textContent=a.name;$('notes').value=a.notes;$('asset-status').value=a.status;$('asset-org').value=String(a.organization_id);$('notes-status').textContent='';$('back-org').href='#/organizations/'+a.organization_id;
  $('empty-report').hidden=false;$('report-content').hidden=true;$('report-origin').hidden=true;clear($('history'));
  try{const hist=await api('/api/assets/'+id);if(token!==assetSerial||currentPage!=='asset')return;if(!hist.length)$('history').append(node('p','История пуста.','subtle'));for(const h of hist){const b=node('button',when(h.collected_at),'history-btn');b.type='button';b.title='Импортирован: '+when(h.imported_at);b.onclick=()=>showReport(h.report,'Из базы',false);$('history').append(b);}if(hist.length)showReport(hist[0].report,'Из базы',false);}catch(e){notify('Не удалось открыть компьютер: '+e.message);}
}
$('save-notes').onclick=async()=>{const id=selectedAsset;if(id===null)return;try{await api('/api/assets/'+id,jsonPost({notes:$('notes').value,status:$('asset-status').value,organization_id:Number($('asset-org').value)},'PUT'));await reloadData();if(selectedAsset===id){$('notes-status').textContent='Сохранено';$('back-org').href='#/organizations/'+$('asset-org').value;}}catch(e){$('notes-status').textContent='Ошибка: '+e.message;}};
$('delete-asset').onclick=async()=>{const a=assets.find(x=>x.id===selectedAsset);if(!a||!confirm('Удалить компьютер «'+a.name+'» и всю историю обследований? Это нельзя отменить. Для списания без удаления выберите статус «Выбыл».'))return;try{await api('/api/assets/'+a.id,jsonPost({},'DELETE'));selectedAsset=null;await reloadData();location.hash='#/organizations/'+a.organization_id;}catch(e){$('notes-status').textContent='Ошибка удаления: '+e.message;}};
function chooseFiles(list){files=[...list].filter(f=>f.name.toLowerCase().endsWith('.json')).sort((a,b)=>(a.webkitRelativePath||a.name).localeCompare(b.webkitRelativePath||b.name));selectedFile=null;currentReport=null;previewSerial++;$('empty-report').hidden=false;$('report-content').hidden=true;$('report-origin').hidden=true;$('import-panel').hidden=true;$('file-error').textContent=files.length?'':'JSON-файлы не найдены.';renderFiles();}
$('file-input').onchange=e=>{chooseFiles(e.target.files);e.target.value='';};$('folder-input').onchange=e=>{chooseFiles(e.target.files);e.target.value='';};
function renderFiles(){clear($('files'));for(const f of files){const b=node('button',f.webkitRelativePath||f.name,'asset file'+(f===selectedFile?' active':''));b.type='button';b.onclick=()=>openFile(f);$('files').append(b);}}
async function openFile(f){selectedFile=f;currentReport=null;previewSerial++;renderFiles();$('file-error').textContent='';$('empty-report').hidden=false;$('report-content').hidden=true;$('report-origin').hidden=true;$('import-panel').hidden=true;
  try{if(f.size>7*1024*1024)throw new Error('Файл больше 7 МБ');const r=JSON.parse(await f.text());if(f!==selectedFile)return;if(!r||r.schema_version!==1||!r.scan_id||!r.collected_at||!r.device||!Array.isArray(r.device.components))throw new Error('Это не поддерживаемый отчёт GoRig версии 1');showReport(r,f.name,true);$('import-panel').hidden=false;$('target').value='';$('import-status').value='in_use';$('import-org').value='0';$('import-notes').value='';$('preview').hidden=true;clear($('diff'));$('import').disabled=true;}catch(e){if(f!==selectedFile)return;selectedFile=null;currentReport=null;renderFiles();$('file-error').textContent='Не удалось открыть файл: '+e.message;}}
const statusText={new:'Новый компьютер. Проверьте отчёт и нажмите «Добавить в базу».',changed:'Найдены отличия от последнего сохранённого обследования.',unchanged:'Отличий среди сравниваемых данных нет. Новое обследование не будет добавлено.',duplicate:'Этот отчёт уже есть в базе (компьютер ID ',older:'Отчёт старше последнего сохранённого обследования. Для защиты текущего состояния он не будет добавлен.'};
async function runPreview(){const token=++previewSerial;$('import').disabled=true;clear($('diff'));if(!currentReport||$('target').value===''){$('preview').hidden=true;return;}$('preview').hidden=false;$('preview').className='alert alert-info mt-3';$('preview').textContent='Сравнение…';try{const p=await api('/api/preview',jsonPost({asset_id:Number($('target').value),report:currentReport}));if(token!==previewSerial)return;$('preview').className='alert mt-3 '+(['new','changed'].includes(p.status)?'alert-success':'alert-warning');$('preview').textContent=statusText[p.status]+(p.status==='duplicate'?p.duplicate_asset_id+').':'')+(p.latest_at?'\nПоследнее сохранённое обследование: '+when(p.latest_at):'');$('import').disabled=!['new','changed'].includes(p.status);if(p.differences?.length){const t=node('table',undefined,'table table-sm align-middle');const head=node('tr');for(const s of ['Поле','В базе','В отчёте'])head.append(node('th',s));t.append(head);for(const d of p.differences){const tr=node('tr');for(const s of [d.field,d.before||'Нет данных',d.after||'Нет данных'])tr.append(node('td',s));t.append(tr);}$('diff').append(t);}}catch(e){if(token!==previewSerial)return;$('preview').className='alert alert-danger mt-3';$('preview').textContent='Ошибка: '+e.message;}}
$('target').onchange=()=>{const a=assets.find(x=>String(x.id)===$('target').value);$('import-status').value=a?.status||'in_use';$('import-org').value=String(a?.organization_id||0);$('import-notes').value=a?.notes||'';runPreview();};
$('import').onclick=async()=>{const target=$('target').value,report=currentReport,file=selectedFile;if(!report||target==='')return;const details={notes:$('import-notes').value,status:$('import-status').value,organization_id:Number($('import-org').value)};$('import').disabled=true;try{const res=await api('/api/import',jsonPost({asset_id:Number(target),report,details}));if(!['new','changed'].includes(res.preview.status)){await runPreview();return;}await reloadData();if(currentReport!==report||selectedFile!==file){notify('Отчёт добавлен. Компьютер ID '+res.asset_id+'.','success');return;}selectedFile=null;currentReport=null;location.hash='#/asset/'+res.asset_id;notify('Отчёт добавлен в базу.','success');}catch(e){$('preview').className='alert alert-danger mt-3';$('preview').hidden=false;$('preview').textContent='Ошибка импорта: '+e.message;}};
$('shutdown').onclick=async()=>{if(serverStopped||!confirm((dirty()?'Есть несохранённые изменения заметки или статуса.\n\n':'')+'Остановить GoRig Base? Чтобы продолжить работу, программу нужно будет запустить снова.'))return;try{await api('/api/shutdown',jsonPost({}));serverStopped=true;$('shutdown').disabled=true;document.querySelector('.server-badge').textContent='Сервер остановлен';notify('GoRig Base остановлен. Чтобы продолжить работу, запустите программу снова.','warning');}catch(e){notify('Не удалось остановить сервер: '+e.message);}};
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
          box.append(node('h4',(index+1)+'. '+a.name));
          const brief=[itemValue(d.manufacturer)?v(d.manufacturer):'',itemValue(d.model)?v(d.model):'',itemValue(d.os)?v(d.os):''].filter(Boolean).join(' · ')||'Нет данных';
          printLine(box,'Краткая информация',brief);
          printLine(box,'Заметка',a.notes?.trim()||'Нет заметки');
          const cpu=byType(report,'processor');printLine(box,'Процессор',cpu.length?cpu.map(c=>itemValue(c.name)?v(c.name):v(c.model)).join('; '):'Нет данных');
          printLine(box,'Память',installedMemory(report));
          const disks=byType(report,'disk');printLine(box,'Накопители',disks.length?disks.map(c=>(itemValue(c.model)||itemValue(c.name)?shortComponent(c)+' — ':'')+formatted(c.attributes?.capacity_bytes,'capacity_bytes')).join('; '):'Нет данных');
          section.append(box);
        }
        group.append(section);
      }
      root.append(group);
    }
    const sign=node('div',undefined,'print-signature');sign.append(node('p','Дата: «____» __________________ 20____ г.'),node('p','Подпись: _____________________ / _____________________'));root.append(sign);
    $('report-message').textContent='';root.hidden=false;$('report-actions').hidden=false;if(currentPage==='reports')document.body.classList.add('report-print-mode');
  }catch(e){if(serial!==reportSerial)return;$('report-message').textContent='Не удалось подготовить отчёт: '+e.message;}
}
function printLine(root,label,value){const p=node('p',undefined,'print-line');p.append(node('strong',label+': '),document.createTextNode(value));root.append(p);}
$('report-all').onclick=()=>generatePrintableReport(null);
$('print-report').onclick=()=>window.print();
function route(){
  if(reverting){reverting=false;return;}
  const hash=location.hash||'#/overview';
  if(lastHash&&hash!==lastHash&&dirty()&&!confirm('Есть несохранённые изменения заметки, организации или статуса. Перейти без сохранения?')){reverting=true;location.hash=lastHash;return;}
  lastHash=hash;assetSerial++;previewSerial++;
  const parts=hash.replace(/^#\/?/,'').split('/');let page=parts[0];
  if(!['overview','organizations','asset','import','reports','settings'].includes(page))page='overview';
  if(page==='organizations'&&parts[1]&&!/^\d+$/.test(parts[1])){notify('Организация не найдена.');location.hash='#/organizations';return;}
  currentPage=page==='organizations'?(parts[1]?'organization':'organizations'):page;
  document.querySelectorAll('.page').forEach(el=>{el.hidden=el.id!=='page-'+currentPage;});
  document.querySelectorAll('[data-nav]').forEach(el=>el.classList.toggle('active',el.dataset.nav===(page==='asset'?'organizations':page)));
  const titles={overview:'Обзор',organizations:'Организации',asset:'Компьютер',import:'Импорт отчёта',reports:'Отчёты',settings:'Настройки'};
  highlightSubmenu();$('breadcrumb').textContent=titles[page];$('breadcrumbs').hidden=['overview','organizations','settings'].includes(page);$('top-import').hidden=page!=='overview';
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
