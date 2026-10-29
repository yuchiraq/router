(() => {
    const $ = id => document.getElementById(id);
    let dirty = false;
    const status = (message,error=false) => { $('status-msg').textContent=message; $('status-msg').classList.toggle('form-error',error); };
    async function request(url,options) {
        const response = await fetch(url,{credentials:'same-origin',headers:{Accept:'application/json'},...options});
        if (!response.ok) throw new Error(response.status===401 ? 'Сессия завершена. Войдите снова.' : 'Не удалось сохранить или загрузить настройки');
        return response.json();
    }
    $('gpt-form').addEventListener('input',() => { dirty=true; status('Есть несохранённые изменения'); });
    window.addEventListener('beforeunload',event=>{if(dirty){event.preventDefault();event.returnValue='';}});
    $('gpt-form').addEventListener('submit',async event=>{
        event.preventDefault();
        if ($('save-btn').disabled || !$('gpt-form').reportValidity()) return;
        const body=new URLSearchParams();
        body.set('enabled',$('enabled').checked?'on':'');
        ['apiKey','model','systemPrompt','maxLogLines','onlyChatIds'].forEach(id=>body.set(id,$(id).value));
        $('gpt-fields').disabled=$('save-btn').disabled=true; $('save-btn').textContent='Сохранение…';
        try { await request('/settings/config',{method:'POST',body}); dirty=false; status('Настройки сохранены'); }
        catch(error){status(window.RouterUI.errorMessage(error),true);}
        finally {$('gpt-fields').disabled=$('save-btn').disabled=false; $('save-btn').textContent='Сохранить';}
    });
    request('/settings/data').then(data=>{
        const cfg=data.config;
        $('enabled').checked=cfg.enabled;
        ['apiKey','model','systemPrompt','maxLogLines'].forEach(id=>$(id).value=cfg[id] ?? '');
        $('onlyChatIds').value=(cfg.onlyChatIds||[]).join(', ');
        $('gpt-fields').disabled=$('save-btn').disabled=false; status('');
    }).catch(error=>status(window.RouterUI.errorMessage(error),true));
})();
