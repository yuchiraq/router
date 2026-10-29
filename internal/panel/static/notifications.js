(() => {
    const $ = id => document.getElementById(id);
    const form = $('notification-form');
    let dirty = false;
    const status = (message, error = false) => { $('status-msg').textContent = message; $('status-msg').style.color = error ? 'var(--accent-red)' : 'var(--text-secondary)'; };
    async function request(url, options) {
        const response = await fetch(url, {credentials:'same-origin', headers:{Accept:'application/json'}, ...options});
        if (!response.ok) throw new Error(response.status === 401 ? 'Сессия завершена. Войдите снова.' : (await response.text()) || 'Ошибка запроса');
        return response.status === 204 ? null : response.json();
    }
    const strings = ['token','chatIds','webhookUrl','webhookSecret','diskPaths'];
    const numbers = ['quietStart','quietEnd','diskWarning','diskCritical','diskReminderHours'];
    function fill(cfg) {
        $('enabled').checked = !!cfg.enabled; $('quietEnabled').checked = !!cfg.quietHoursOn;
        $('token').value = cfg.token || ''; $('chatIds').value = (cfg.chatIds || []).join(', ');
        $('webhookUrl').value = cfg.webhookUrl || ''; $('webhookSecret').value = cfg.webhookSecret || '';
        $('diskPaths').value = (cfg.diskPaths || []).join('\n');
        const values = {quietStart:cfg.quietHoursStart ?? 20, quietEnd:cfg.quietHoursEnd ?? 8, diskWarning:cfg.diskWarning || 85, diskCritical:cfg.diskCritical || 95, diskReminderHours:cfg.diskReminderHours || 6};
        numbers.forEach(id => $(id).value = values[id]);
        document.querySelectorAll('[data-event]').forEach(el => el.checked = !!cfg.events?.[el.dataset.event]);
        $('known-chat-ids').replaceChildren();
        if (!cfg.knownChatIds?.length) $('known-chat-ids').textContent = 'Нет известных чатов';
        (cfg.knownChatIds || []).forEach(id => {
            const button = document.createElement('button'); button.type = 'button'; button.className = 'btn btn-secondary'; button.textContent = String(id); button.title = 'Добавить получателя';
            button.onclick = () => { const ids = $('chatIds').value.split(',').map(s => s.trim()).filter(Boolean); if (!ids.includes(String(id))) ids.push(String(id)); $('chatIds').value = ids.join(', '); dirty = true; status('Получатель добавлен. Сохраните настройки.'); };
            $('known-chat-ids').append(button);
        });
    }
    form.addEventListener('input', () => { dirty = true; status('Есть несохранённые изменения'); $('diskCritical').setCustomValidity(''); });
    window.addEventListener('beforeunload', event => { if (dirty) { event.preventDefault(); event.returnValue = ''; } });
    form.addEventListener('submit', async event => {
        event.preventDefault();
        if (+$('diskCritical').value <= +$('diskWarning').value) { $('diskCritical').setCustomValidity('Критический уровень должен быть выше предупреждения'); $('diskCritical').reportValidity(); return; }
        if (!form.reportValidity()) return;
        const body = new URLSearchParams();
        [...strings,...numbers].forEach(id => body.set(id,$(id).value.trim()));
        body.set('enabled',$('enabled').checked?'on':''); body.set('quietEnabled',$('quietEnabled').checked?'on':'');
        document.querySelectorAll('[data-event]').forEach(el => body.set('event_'+el.dataset.event,el.checked?'on':''));
        $('save-btn').disabled = $('test-btn').disabled = true; $('notification-fields').disabled = true; status('Сохранение…');
        $('save-btn').textContent = 'Сохранение…';
        try { const data = await request('/notifications/config',{method:'POST',body}); fill(data.config); dirty = false; status(data.warning || 'Настройки сохранены',!!data.warning); }
        catch (error) { status(error.message,true); }
        finally { $('save-btn').disabled = $('test-btn').disabled = false; $('notification-fields').disabled = false; $('save-btn').textContent = 'Сохранить'; }
    });
    $('test-btn').onclick = async () => {
        if (dirty) { status('Сначала сохраните настройки',true); return; }
        $('test-btn').disabled = true; status('Отправка…');
        try { await request('/notifications/test',{method:'POST'}); status('Тестовое сообщение отправлено'); }
        catch (error) { status(error.message,true); }
        finally { $('test-btn').disabled = false; }
    };
    request('/notifications/data').then(data => { fill(data.config); $('notification-fields').disabled = false; $('save-btn').disabled = $('test-btn').disabled = false; status(''); }).catch(error => status(error.message,true));
})();
