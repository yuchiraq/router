(() => {
    const $ = id => document.getElementById(id);
    const dialog = $('settings-card'), form = $('backup-form');
    const state = {jobs:[], entries:[], busy:false, loading:false, loaded:false, baseline:''};
    const icon = name => { const el=document.createElement('span');el.className='icon';el.style.setProperty('--icon',`url(/static/icons/${name}.svg)`);el.setAttribute('aria-hidden','true');return el; };
    const node = (tag, cls, text) => {const el=document.createElement(tag);el.className=cls;if(text!==undefined)el.textContent=text;return el;};
    function button(label, symbol, action){const el=node('button','btn btn-icon');el.type='button';el.title=label;el.setAttribute('aria-label',label);el.append(icon(symbol));el.onclick=action;el.disabled=state.busy;return el;}
    const validDate = value => value && !value.startsWith('0001') && Number.isFinite(Date.parse(value));
    const date = value => validDate(value) ? new Date(value).toLocaleString('ru-RU',{day:'2-digit',month:'short',hour:'2-digit',minute:'2-digit'}) : 'Ещё не создана';
    function size(bytes){const units=['Б','КиБ','МиБ','ГиБ','ТиБ'];let i=0;while(bytes>=1024&&i<units.length-1){bytes/=1024;i++;}return new Intl.NumberFormat('ru-RU',{maximumFractionDigits:i?1:0}).format(bytes)+' '+units[i];}
    function status(message,error=false){$('status-msg').textContent=message;$('status-msg').style.color=error?'var(--accent-red)':'var(--text-secondary)';}
    function report(error){const message=window.RouterUI.errorMessage(error);status(message,true);if(dialog.open){$('editor-error').textContent=message;$('editor-error').hidden=false;}window.RouterUI.announce(message,true);}
    function serialize(){return new URLSearchParams(new FormData(form)).toString();}
    function openEditor(job){
        form.reset();$('job-id').value=job?.id||'';$('name').value=job?.name||'';$('sources').value=(job?.sources||[]).join('\n');$('destinationDir').value=job?.destinationDir||'';$('keepCopies').value=job?.keepCopies||10;$('intervalMinutes').value=job?.intervalMinutes||60;$('enabled').checked=!!job?.enabled;
        $('delete-btn').hidden=!job;$('editor-title').textContent=job?'Изменить задание':'Новое задание';$('editor-error').hidden=true;state.baseline=serialize();dialog.showModal();
        // Keep the mobile keyboard closed until a field is deliberately selected.
        if(matchMedia('(pointer:coarse)').matches)$('close-editor').focus();else $('name').focus();
    }
    async function closeEditor(){if(state.busy)return;if(serialize()!==state.baseline&&!await window.RouterUI.confirm('Закрыть без сохранения?','Изменения в задании будут потеряны.','Не сохранять'))return;dialog.close();}
    dialog.addEventListener('cancel',event=>{event.preventDefault();closeEditor();});$('close-editor').onclick=closeEditor;$('cancel-editor').onclick=closeEditor;$('new-job-btn').onclick=()=>openEditor();
    function empty(container,title,description,create=false){const box=node('div','backup-empty');box.append(icon('archive'),node('h2','',title));if(description)box.append(node('p','',description));if(create){const add=node('button','btn','Новое задание');add.type='button';add.onclick=()=>openEditor();box.append(add);}container.append(box);}
    function render(){
        const query=$('backup-search').value.trim().toLocaleLowerCase('ru');
        $('jobs-count').textContent=state.jobs.length;$('archives-count').textContent=state.entries.length;$('backup-active').textContent=state.jobs.filter(j=>j.enabled).length;$('backup-count').textContent=state.entries.length;$('backup-size').textContent=size(state.entries.reduce((sum,e)=>sum+e.sizeBytes,0));$('backup-latest').textContent=state.entries.length?date(state.entries[0].createdAt):'Пока нет';
        const jobs=$('jobs-table');jobs.replaceChildren();
        for(const job of state.jobs.filter(j=>[j.name,j.destinationDir,...(j.sources||[])].join(' ').toLocaleLowerCase('ru').includes(query))){
            const row=node('article','backup-row');const main=node('div','backup-job-main'),symbol=node('div','service-icon');symbol.append(icon('archive'));const copy=node('div','backup-job-copy');copy.append(node('h2','disk-title',job.name));
            const meta=node('div','backup-job-meta');meta.append(node('span','backup-schedule'+(job.enabled?' is-enabled':''),job.enabled?'По расписанию':'Вручную'),node('span','',job.enabled?`Каждые ${job.intervalMinutes} мин`:`Хранить ${job.keepCopies} копий`));copy.append(meta,node('p','backup-path',job.destinationDir));main.append(symbol,copy);
            const latest=state.entries.find(e=>e.jobId===job.id);const last=node('div','backup-last',latest?date(latest.createdAt):'Копий пока нет');last.append(node('small','',latest?size(latest.sizeBytes):`${(job.sources||[]).length} источн.`));
            const actions=node('div','backup-row-actions');actions.append(button(`Создать копию: ${job.name}`,'play',()=>run(job)),button(`Изменить: ${job.name}`,'settings-2',()=>openEditor(job)));row.append(main,last,actions);jobs.append(row);
        }
        if(!jobs.children.length)empty(jobs,state.jobs.length?'Ничего не найдено':'Заданий пока нет',state.jobs.length?'Измените поисковый запрос.':'',!state.jobs.length);
        const archives=$('backups-table');archives.replaceChildren();
        for(const entry of state.entries.filter(e=>(!$('archive-job').value||e.jobId===$('archive-job').value)&&[e.jobName,e.path].join(' ').toLocaleLowerCase('ru').includes(query))){
            const row=node('article','backup-row backup-archive');const main=node('div','backup-job-main'),copy=node('div','backup-job-copy');copy.append(node('h2','',entry.jobName),node('p','backup-path',entry.path));main.append(copy);row.append(main,node('time','backup-last',date(entry.createdAt)),node('span','archive-size',size(entry.sizeBytes)));row.querySelector('time').dateTime=entry.createdAt;archives.append(row);
        }
        if(!archives.children.length)empty(archives,state.entries.length?'Ничего не найдено':'Архивов пока нет',state.entries.length?'Измените поиск или выбранное задание.':'');
    }
    function showTab(tab){document.querySelectorAll('[data-backup-tab]').forEach(el=>{const active=el.dataset.backupTab===tab;el.setAttribute('aria-selected',String(active));el.tabIndex=active?0:-1;});$('jobs-panel').hidden=tab!=='jobs';$('archives-panel').hidden=tab!=='archives';window.dispatchEvent(new Event('router-tabs-change'));}
    const tabs=[...document.querySelectorAll('[data-backup-tab]')];tabs.forEach((tab,i)=>{tab.onclick=()=>showTab(tab.dataset.backupTab);tab.onkeydown=event=>{if(!['ArrowLeft','ArrowRight','Home','End'].includes(event.key))return;event.preventDefault();const next=tabs[event.key==='Home'?0:event.key==='End'?tabs.length-1:1-i];next.focus();next.click();};});
    $('backup-search').oninput=render;$('archive-job').onchange=render;
    async function load(){
        if(state.loading)return;state.loading=true;$('refresh-backups').disabled=true;
        try{const response=await fetch('/backups/data',{cache:'no-store',headers:{Accept:'application/json'}});if(!response.ok)throw new Error('Не удалось загрузить резервные копии');const data=await response.json();state.jobs=(data.jobs||[]).sort((a,b)=>a.name.localeCompare(b.name,'ru'));state.entries=(data.entries||[]).sort((a,b)=>Date.parse(b.createdAt)-Date.parse(a.createdAt));state.loaded=true;
            const filter=$('archive-job'),selected=filter.value;filter.replaceChildren(new Option('Все задания',''));for(const job of state.jobs)filter.append(new Option(job.name,job.id));filter.value=state.jobs.some(j=>j.id===selected)?selected:'';
            $('backup-error').hidden=!data.lastError;$('backup-error-detail').textContent=data.lastError||'';render();
        }catch(error){report(error);if(!state.loaded){$('jobs-table').replaceChildren();empty($('jobs-table'),'Не удалось загрузить задания','Повторите загрузку кнопкой обновления.');}}
        finally{state.loading=false;$('refresh-backups').disabled=state.busy;}
    }
    async function post(path,body,message){const response=await fetch(path,{method:'POST',headers:{'Content-Type':'application/x-www-form-urlencoded'},body});if(!response.ok)throw new Error(message);return response;}
    async function perform(action){if(state.busy)return;state.busy=true;form.setAttribute('aria-busy','true');document.querySelectorAll('.backups-page button,.backup-editor button,.backup-editor input,.backup-editor textarea').forEach(el=>el.disabled=true);
        try{await action();await load();}catch(error){report(error);}finally{state.busy=false;form.removeAttribute('aria-busy');document.querySelectorAll('.backups-page button,.backup-editor button,.backup-editor input,.backup-editor textarea').forEach(el=>el.disabled=false);if(state.loaded)render();}}
    form.onsubmit=event=>{event.preventDefault();if(!form.reportValidity())return;const body=serialize();perform(async()=>{await post('/backups/config',body,'Не удалось сохранить задание');dialog.close();status('Задание сохранено');window.RouterUI.announce('Задание сохранено');});};
    function run(job){perform(async()=>{status(`Создаётся копия: ${job.name}…`);await post('/backups/run',new URLSearchParams({id:job.id}),'Не удалось создать копию. Проверьте пути, права доступа и свободное место.');status('Резервная копия создана');window.RouterUI.announce('Резервная копия создана');});}
    $('delete-btn').onclick=async()=>{const id=$('job-id').value;const count=state.entries.filter(e=>e.jobId===id).length;if(!await window.RouterUI.confirm('Удалить задание и архивы?',`Будут удалены задание и все его архивы (${count}). Исходные файлы останутся. Отменить удаление нельзя.`,'Удалить всё'))return;perform(async()=>{await post('/backups/delete',new URLSearchParams({id}),'Не удалось удалить задание');dialog.close();status('Задание и архивы удалены');});};
    $('refresh-backups').onclick=()=>{status('');load();};load();
    setInterval(()=>{if(!document.hidden&&!dialog.open&&!state.busy)load();},15000);
})();
