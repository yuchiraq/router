(() => {
    const root=document.documentElement,preview=document.body.dataset.preview==='true';
    try{const theme=localStorage.getItem('router-theme');if(theme==='dark'||theme==='light')root.dataset.theme=theme;}catch(_){}
    document.getElementById('theme-button').onclick=()=>{const dark=root.dataset.theme==='dark'||!root.dataset.theme&&matchMedia('(prefers-color-scheme:dark)').matches;root.dataset.theme=dark?'light':'dark';try{localStorage.setItem('router-theme',root.dataset.theme)}catch(_){}};
    const button=document.getElementById('check-service'),result=document.getElementById('check-result'),link=document.getElementById('return-link');
    link.href=location.pathname+location.search+location.hash;
    let pending=false,available=false;
    async function check(manual=false){
        if(pending||document.hidden)return;
        if(preview){if(manual)result.textContent='Предпросмотр: проверка рабочего сайта отключена';return;}
        pending=true;button.disabled=true;
        if(manual)result.textContent='Проверяем соединение…';
        try{
            const response=await fetch('/_router/maintenance/status',{cache:'no-store',credentials:'omit',signal:AbortSignal.timeout(5000),headers:{Accept:'application/json'}});
            if(!response.ok)throw new Error();
            const status=await response.json();available=status.available===true;
            document.body.classList.toggle('is-ready',available);link.hidden=!available;button.hidden=available;
            document.getElementById('service-label').textContent=available?'Сайт снова доступен':'Сайт пока недоступен';
            document.getElementById('page-title').textContent=available?'Можно возвращаться.':'Небольшая пауза.';
            document.getElementById('service-description').textContent=available?'Соединение восстановлено. Игра сохранится.':'Ожидаем восстановления работы сайта.';
            result.textContent=available?'Откройте сайт, когда будете готовы.':'Проверено в '+new Date().toLocaleTimeString('ru-RU');
        }catch(_){result.textContent='Не удалось проверить соединение. Повторим автоматически.';}
        finally{pending=false;button.disabled=false;}
    }
    button.addEventListener('click',()=>check(true));
    setInterval(()=>check(),15000);document.addEventListener('visibilitychange',()=>check());check();
})();
