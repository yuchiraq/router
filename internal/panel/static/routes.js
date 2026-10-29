(() => {
    const rows=[...document.querySelectorAll('.rule-item')];
    const tabs=[...document.querySelectorAll('[data-filter]')];
    const search=document.getElementById('rule-search');
    let filter='all'; let polling=false; let changes=0;
    document.addEventListener('submit',()=>changes++,true);
    function update() {
        const query=search.value.trim().toLocaleLowerCase(); let visible=0;
        rows.forEach(row=>{
            row.hidden=!(filter==='all'||row.dataset.state===filter)||!row.querySelector('.rule-info').textContent.toLocaleLowerCase().includes(query);
            if(!row.hidden)visible++;
        });
        document.getElementById('rule-search-empty').hidden=visible>0||!rows.length;
        for(const state of ['all','online','paused','pending']){
            const count=state==='all'?rows.length:rows.filter(r=>r.dataset.state===state).length;
            const el=document.getElementById('count-'+state);
            if(el.textContent!==String(count)) {el.textContent=count;if(!matchMedia('(prefers-reduced-motion:reduce)').matches)el.animate([{opacity:.3,transform:'translateY(4px)'},{opacity:1,transform:'translateY(0)'}],{duration:240});}
        }
    }
    tabs.forEach(tab=>{
        tab.addEventListener('click',()=>{
            filter=tab.dataset.filter;
            tabs.forEach(t=>{t.setAttribute('aria-selected',String(t===tab));t.tabIndex=t===tab?0:-1;});
            document.getElementById('routes-panel').setAttribute('aria-labelledby',tab.id);
            update(); window.dispatchEvent(new Event('router-tabs-change'));
        });
        tab.addEventListener('keydown',event=>{
            if(!['ArrowLeft','ArrowRight','Home','End'].includes(event.key))return;
            event.preventDefault(); const i=tabs.indexOf(tab);
            tabs[event.key==='Home'?0:event.key==='End'?tabs.length-1:(i+(event.key==='ArrowLeft'?-1:1)+tabs.length)%tabs.length].focus();
        });
    });
    search.addEventListener('input',update);
    window.addEventListener('router-routes-change',update);update();
    async function poll() {
        if(polling||document.hidden||document.querySelector('form[aria-busy="true"],dialog[open]'))return;
        polling=true; const revision=changes;
        try {
            const response=await fetch('/routes/status',{headers:{Accept:'application/json'},cache:'no-store'});
            if(!response.ok)throw new Error('Не удалось обновить состояние сервисов');
            const data=await response.json();
            if(revision!==changes||document.querySelector('form[aria-busy="true"],dialog[open]'))return;
            const global=document.querySelector('.maintenance-form input');global.checked=global.defaultChecked=data.maintenance;
            rows.forEach(row=>{
                const status=data.rules.find(r=>r.host===row.dataset.host);if(!status)return;
                row.dataset.down=String(status.down);row.dataset.checked=status.checkedAt;
                const toggle=row.querySelector('input[name="maintenance"]');toggle.checked=toggle.defaultChecked=status.maintenance;
                row.querySelector('.route-state').title=status.error||'';
            });
            window.RouterUI.refreshMaintenance();
            document.querySelector('.service-status').classList.remove('status-stale');
        } catch(_) {document.querySelector('.service-status').classList.add('status-stale');}
        finally {polling=false;}
    }
    setInterval(poll,10000);document.addEventListener('visibilitychange',poll);poll();
})();
