(() => {
    const root = document.documentElement;
    const icon = name => { const el = document.createElement('span'); el.className = 'icon'; el.style.setProperty('--icon', `url(/static/icons/${name}.svg)`); el.setAttribute('aria-hidden', 'true'); return el; };
    const setTheme = theme => {
        if (theme === 'light' || theme === 'dark') root.dataset.theme = theme; else delete root.dataset.theme;
        window.dispatchEvent(new Event('router-theme-change'));
    };
    let preference = 'system';
    try { preference = localStorage.getItem('router-theme') || 'system'; } catch (_) {}
    setTheme(preference);
    const main = document.querySelector('main');
    if (main) {
        main.id = 'main-content'; main.tabIndex = -1;
        const skip = document.createElement('a'); skip.className = 'skip-link'; skip.href = '#main-content'; skip.textContent = 'К содержимому'; document.body.prepend(skip);
    }
    const feedback = document.createElement('div'); feedback.className = 'feedback'; feedback.setAttribute('role','status'); feedback.setAttribute('aria-live','polite'); feedback.hidden = true; document.body.append(feedback);
    let feedbackTimer;
    function announce(message, error = false) {
        clearTimeout(feedbackTimer); feedback.hidden = false; feedback.classList.toggle('is-error',error); feedback.textContent = message;
        feedbackTimer = setTimeout(() => { feedback.hidden = true; }, error ? 14000 : 7000);
    }
    function makeDialog(id, title) {
        const dialog = document.createElement('dialog'); dialog.className = 'sheet'; dialog.id = id; dialog.setAttribute('aria-labelledby',id+'-title');
        const header = document.createElement('div'); header.className = 'sheet-header';
        const h = document.createElement('h2'); h.id = id+'-title'; h.textContent = title;
        const close = document.createElement('button'); close.type = 'button'; close.className = 'btn btn-icon'; close.title = 'Закрыть'; close.setAttribute('aria-label','Закрыть'); close.append(icon('x')); close.onclick = () => dialog.close();
        header.append(h,close); dialog.append(header); document.body.append(dialog); return dialog;
    }
    const confirmDialog = makeDialog('confirm-action','Подтвердите действие');
    const confirmText = document.createElement('p'); confirmText.className = 'sheet-description';
    const confirmActions = document.createElement('div'); confirmActions.className = 'sheet-actions';
    const cancel = document.createElement('button'); cancel.type = 'button'; cancel.className = 'btn btn-secondary'; cancel.textContent = 'Отмена';
    const accept = document.createElement('button'); accept.type = 'button'; accept.className = 'btn btn-danger';
    confirmActions.append(cancel,accept); confirmDialog.append(confirmText,confirmActions);
    function confirmAction(title, message, action) {
        if (confirmDialog.open) return Promise.resolve(false);
        confirmDialog.querySelector('h2').textContent = title; confirmText.textContent = message; accept.textContent = action;
        confirmDialog.returnValue = ''; confirmDialog.showModal(); cancel.focus();
        return new Promise(resolve => {
            cancel.onclick = () => confirmDialog.close('cancel'); accept.onclick = () => confirmDialog.close('accept');
            confirmDialog.addEventListener('close',() => resolve(confirmDialog.returnValue === 'accept'),{once:true});
        });
    }
    const errorMessage = error => error instanceof TypeError ? 'Нет связи с сервером. Проверьте подключение и повторите попытку.' : error instanceof SyntaxError ? 'Не удалось прочитать ответ сервера. Обновите страницу.' : error.message || 'Не удалось выполнить действие';
    window.RouterUI = {announce, confirm:confirmAction, errorMessage};
    const nav = document.querySelector('.nav-container');
    if (nav) {
        const logo = nav.querySelector('.nav-logo');
        const brand = document.createElement('span'); brand.className = 'brand-lockup';
        const product = document.createElement('strong'); product.textContent = 'Router';
        const author = document.createElement('small'); author.textContent = 'by Qoncept';
        brand.append(product,author); logo.replaceChildren(icon('qoncept'),brand); logo.href = '/';
        const links = nav.querySelector('.nav-links'); links.replaceChildren();
        const entries = [['/','Маршруты','route'],['/stats','Обзор','layout-dashboard'],['/backups','Копии','archive'],['/notifications','Уведомления','bell'],['/settings','GPT','settings-2'],['/account','Аккаунт','user-round']];
        function navLink([href,name,symbol]) {
            const a = document.createElement('a'); a.href = href;
            const label = document.createElement('span'); label.className = 'nav-label'; label.textContent = name;
            a.append(icon(symbol),label);
            if (location.pathname === href) { a.className = 'active'; a.setAttribute('aria-current','page'); }
            return a;
        }
        entries.forEach(entry => { const a = navLink(entry); if (['/backups','/settings','/account'].includes(entry[0])) a.classList.add('nav-secondary'); links.append(a); });
        nav.parentElement.setAttribute('aria-label','Основная навигация');
        const moreDialog = makeDialog('more-navigation','Другие разделы');
        const moreLinks = document.createElement('nav'); moreLinks.className = 'more-links'; moreLinks.setAttribute('aria-label','Другие разделы');
        entries.filter(e => ['/backups','/settings','/account'].includes(e[0])).forEach(entry => moreLinks.append(navLink(entry)));
        moreDialog.append(moreLinks);
        const more = document.createElement('button'); more.type = 'button'; more.className = 'nav-more'; more.setAttribute('aria-haspopup','dialog'); more.setAttribute('aria-controls','more-navigation');
        more.append(icon('ellipsis'),document.createTextNode('Ещё'));
        if (['/backups','/settings','/account'].includes(location.pathname)) more.classList.add('active');
        more.onclick = () => moreDialog.showModal(); links.append(more);
        const measureNavigation = () => root.style.setProperty('--bottom-nav-height',`${links.getBoundingClientRect().height}px`);
        new ResizeObserver(measureNavigation).observe(links);
        const footer = document.createElement('div'); footer.className = 'nav-footer';
        const themeLabel = document.createElement('label'); themeLabel.className = 'appearance';
        themeLabel.append(icon('sun-moon'));
        const select = document.createElement('select'); select.setAttribute('aria-label','Оформление');
        [['system','Системное'],['light','Светлое'],['dark','Тёмное']].forEach(([value,text]) => { const option = document.createElement('option'); option.value = value; option.textContent = text; select.append(option); });
        select.value = preference;
        select.onchange = () => { setTheme(select.value); try { localStorage.setItem('router-theme',select.value); } catch (_) {} };
        themeLabel.append(select); footer.append(themeLabel); nav.append(footer);
    }
    const title = document.querySelector('h1');
    const titles = {'/':'Маршруты','/stats':'Обзор сервера','/backups':'Резервные копии','/notifications':'Уведомления','/account':'Аккаунт'};
    if (title && titles[location.pathname]) { title.textContent = titles[location.pathname]; document.title = `${title.textContent} · Router by Qoncept`; }
    root.lang = 'ru';
    document.querySelectorAll('#status,#status-msg').forEach(el => { el.setAttribute('role','status'); el.setAttribute('aria-live','polite'); });
    document.querySelectorAll('.dashboard-widget').forEach(widget => {
        const title = widget.querySelector('.card-header span:last-child').textContent;
        const body = widget.querySelector('.card-body'); body.tabIndex = 0; body.setAttribute('role','region'); body.setAttribute('aria-label',title);
        const canvas = body.querySelector('canvas');
        if (canvas) { canvas.setAttribute('role','img'); canvas.setAttribute('aria-label',title); }
    });
    document.querySelectorAll('input.form-control,textarea.form-control').forEach(el => {
        if (!el.labels?.length && !el.hasAttribute('aria-label')) el.setAttribute('aria-label', el.title || el.placeholder || el.name || el.id);
    });
    document.querySelectorAll('[data-open-dialog]').forEach(button => button.addEventListener('click',() => document.getElementById(button.dataset.openDialog).showModal()));
    document.querySelectorAll('[data-close-dialog]').forEach(button => button.addEventListener('click',() => button.closest('dialog').close()));
    function renderMaintenance() {
        const global = document.querySelector('.maintenance-form input').checked;
        const symbol = document.querySelector('.status-symbol');
        symbol.classList.toggle('warning',global);
        symbol.replaceChildren(icon(global ? 'triangle-alert' : 'circle-check'));
        document.querySelector('.status-copy strong').textContent = global ? 'Сайты на обслуживании' : 'Маршрутизация включена';
        document.querySelectorAll('.rule-item').forEach(row => {
            const panel = row.dataset.panel === 'true';
            const own = row.querySelector('input[name="maintenance"]').checked;
            const state = row.querySelector('.route-state');
            const down = row.dataset.down === 'true';
            const checked = row.dataset.checked && !row.dataset.checked.startsWith('0001');
            const paused = !panel && (global || own || down);
            row.dataset.state = paused ? 'paused' : panel || checked ? 'online' : 'pending';
            state.textContent = panel ? 'Панель управления' : global ? 'Общая пауза' : own ? 'Ручная пауза' : down ? 'Авто: сервис недоступен' : checked ? 'Доступен' : 'Проверка…';
            state.classList.toggle('is-paused',paused);
            const time = row.querySelector('.health-time');
            if (time) time.textContent = checked ? 'Проверен в '+new Date(row.dataset.checked).toLocaleTimeString('ru-RU') : '';
        });
        window.dispatchEvent(new Event('router-routes-change'));
    }
    window.RouterUI.refreshMaintenance = renderMaintenance;
    if (document.querySelector('.maintenance-form')) renderMaintenance();
    document.querySelectorAll('form[data-route-form]').forEach(form => {
        let pending = false;
        async function submit() {
            if (pending || !form.reportValidity()) return;
            pending = true;
            const body = new URLSearchParams(new FormData(form));
            const controls = [...form.querySelectorAll('button,input')];
            controls.forEach(el => el.disabled = true); form.setAttribute('aria-busy','true');
            try {
                const toggle = form.querySelector('input[name="maintenance"]');
                const response = await fetch(form.action,{method:'POST',body,credentials:'same-origin',headers:{Accept:toggle ? 'application/json' : 'text/html'}});
                if (response.status === 401) throw new Error('Сессия завершена. Обновите страницу и войдите снова.');
                if (!response.ok) throw new Error((await response.text()).trim() || 'Не удалось сохранить изменения');
                if (toggle) {
                    const data = await response.json();
                    toggle.checked = toggle.defaultChecked = data.maintenance;
                    controls.forEach(el => el.disabled = false); form.removeAttribute('aria-busy'); pending = false;
                    renderMaintenance();
                    announce(data.maintenance ? 'Режим обслуживания включён' : 'Режим обслуживания выключен');
                    return;
                }
                location.assign(response.url || '/');
            } catch (error) {
                const toggle = form.querySelector('input[name="maintenance"]'); if (toggle) toggle.checked = toggle.defaultChecked;
                controls.forEach(el => el.disabled = false); form.removeAttribute('aria-busy'); pending = false;
                const message = form.querySelector('[data-form-error]');
                if (message) { message.textContent = errorMessage(error); message.hidden = false; } else announce(errorMessage(error),true);
            }
        }
        form.addEventListener('submit',async event => {
            event.preventDefault();
            if (form.getAttribute('action') === '/remove' && !await confirmAction('Удалить маршрут?',`${form.elements.host.value} перестанет открываться через роутер. Файлы сайта останутся на сервере.`,'Удалить маршрут')) return;
            submit();
        });
        const toggle = form.querySelector('input[name="maintenance"]');
        if (toggle) toggle.addEventListener('change',async () => {
            if (pending) return;
            const enable = toggle.checked;
            if (enable) {
                toggle.checked = false; toggle.disabled = true;
                const global = form.getAttribute('action') === '/';
                const confirmed = await confirmAction('Включить обслуживание?',global ? 'Все сайты, кроме панели управления, временно покажут страницу обслуживания.' : `${form.elements.host.value} временно покажет страницу обслуживания.`,'Включить');
                toggle.disabled = false;
                if (!confirmed) return;
                toggle.checked = true;
            }
            submit();
        });
    });
    window.dispatchEvent(new Event('router-theme-change'));
})();
