(() => {
    const $ = id => document.getElementById(id);
    const form = $('notification-form');
    let busy = false;
    let dirty = false;
    const status = (message, error = false) => { $('status-msg').textContent = message; $('status-msg').style.color = error ? 'var(--accent-red)' : 'var(--text-secondary)'; };
    async function request(url, options) {
        const response = await fetch(url, {credentials:'same-origin', headers:{Accept:'application/json'}, ...options});
        if (!response.ok) throw new Error(response.status === 401 ? 'Сессия завершена. Войдите снова.' : (await response.text()) || 'Ошибка запроса');
        return response.status === 204 ? null : response.json();
    }
    const strings = ['token','chatIds','webhookUrl','webhookSecret','diskPaths'];
    const numbers = ['quietStart','quietEnd','diskWarning','diskCritical','diskReminderHours'];
    function quietSummary() {
        $('quiet-summary').textContent = !$('quietEnabled').checked ? 'Без ограничения по времени' : $('quietStart').value === $('quietEnd').value ? 'Пауза на весь день, включая критические уведомления' : 'В указанные часы приостановлены все уведомления, включая критические';
    }
    function renderMonitor(data) {
        const monitor = data.monitor;
        if (monitor) {
            $('monitor-state').textContent = monitor.reason;
            const date = value => value && !value.startsWith('0001') ? new Date(value).toLocaleString('ru-RU') : 'ещё не было';
            $('monitor-detail').textContent = `Проверка: ${date(monitor.lastCheck)}. Последняя отправка: ${date(monitor.lastSent)}.`;
            $('monitor-error').textContent = monitor.lastError; $('monitor-error').hidden = !monitor.lastError;
            $('server-timezone').textContent = `Часовой пояс сервера: ${monitor.timezone}`;
        }
        const list = $('monitored-disks'); list.replaceChildren();
        (data.disks || []).forEach(disk => {
            const row = document.createElement('div'); row.className = 'disk-row';
            const name = document.createElement('strong'); name.textContent = disk.mountpoint;
            const value = document.createElement('span'); value.className = 'muted';
            const stale = Date.now() - new Date(disk.sampledAt).getTime() > 120000;
            value.textContent = stale ? 'Данные устарели' : `${disk.usedPercent.toFixed(1)}% · свободно ${disk.freeGB.toFixed(1)} ГиБ`;
            row.classList.toggle('disk-warning',stale || disk.usedPercent >= data.config.diskWarning);
            row.append(name,value); list.append(row);
        });
        (data.unavailable || []).forEach(path => { const row = document.createElement('p'); row.className='form-error'; row.textContent=`Недоступен путь: ${path}`; list.append(row); });
        if (!list.children.length) list.textContent = 'Нет данных о дисках. Добавьте путь и дождитесь проверки.';
    }
    function fill(cfg) {
        $('enabled').checked = !!cfg.enabled; $('quietEnabled').checked = !!cfg.quietHoursOn;
        $('token').value = cfg.token || ''; $('chatIds').value = (cfg.chatIds || []).join(', ');
        $('webhookUrl').value = cfg.webhookUrl || ''; $('webhookSecret').value = cfg.webhookSecret || '';
        $('diskPaths').value = (cfg.diskPaths || []).join('\n');
        const values = {quietStart:cfg.quietHoursStart ?? 20, quietEnd:cfg.quietHoursEnd ?? 8, diskWarning:cfg.diskWarning || 85, diskCritical:cfg.diskCritical || 95, diskReminderHours:cfg.diskReminderHours || 6};
        numbers.forEach(id => $(id).value = values[id]);
        quietSummary();
        document.querySelectorAll('[data-event]').forEach(el => el.checked = !!cfg.events?.[el.dataset.event]);
        $('known-chat-ids').replaceChildren();
        if (!cfg.knownChatIds?.length) $('known-chat-ids').textContent = 'Нет известных чатов';
        (cfg.knownChatIds || []).forEach(id => {
            const button = document.createElement('button'); button.type = 'button'; button.className = 'btn btn-secondary'; button.textContent = String(id); button.title = 'Добавить получателя';
            button.onclick = () => { const ids = $('chatIds').value.split(',').map(s => s.trim()).filter(Boolean); if (!ids.includes(String(id))) ids.push(String(id)); $('chatIds').value = ids.join(', '); dirty = true; status('Получатель добавлен. Сохраните настройки.'); };
            $('known-chat-ids').append(button);
        });
    }
    form.addEventListener('input', () => { dirty = true; status('Есть несохранённые изменения'); $('diskCritical').setCustomValidity(''); quietSummary(); });
    window.addEventListener('beforeunload', event => { if (dirty) { event.preventDefault(); event.returnValue = ''; } });
    form.addEventListener('submit', async event => {
        event.preventDefault();
        if (busy) return;
        if (+$('diskCritical').value <= +$('diskWarning').value) { $('diskCritical').setCustomValidity('Критический уровень должен быть выше предупреждения'); $('diskCritical').reportValidity(); return; }
        if (!form.reportValidity()) return;
        const body = new URLSearchParams();
        [...strings,...numbers].forEach(id => body.set(id,$(id).value.trim()));
        body.set('enabled',$('enabled').checked?'on':''); body.set('quietEnabled',$('quietEnabled').checked?'on':'');
        document.querySelectorAll('[data-event]').forEach(el => body.set('event_'+el.dataset.event,el.checked?'on':''));
        busy = true; $('save-btn').disabled = $('test-btn').disabled = true; $('notification-fields').disabled = true; status('Сохранение…');
        $('save-btn').textContent = 'Сохранение…';
        try { const data = await request('/notifications/config',{method:'POST',body}); fill(data.config); renderMonitor(data); dirty = false; status(data.warning || 'Настройки сохранены',!!data.warning); }
        catch (error) { status(window.RouterUI.errorMessage(error),true); }
        finally { busy = false; $('save-btn').disabled = $('test-btn').disabled = false; $('notification-fields').disabled = false; $('save-btn').textContent = 'Сохранить'; }
    });
    async function test(kind) {
        if (busy) return;
        if (dirty) { status('Сначала сохраните настройки',true); return; }
        busy = true; $('save-btn').disabled = $('test-btn').disabled = $('disk-test-btn').disabled = true; status('Отправка…');
        try { await request('/notifications/test',{method:'POST',body:new URLSearchParams({kind})}); status('Тестовое сообщение отправлено'); window.RouterUI.announce('Тестовое сообщение отправлено'); }
        catch (error) { status(window.RouterUI.errorMessage(error),true); }
        finally { busy = false; $('save-btn').disabled = $('test-btn').disabled = $('disk-test-btn').disabled = false; }
    }
    $('test-btn').onclick = () => test('general');
    $('disk-test-btn').onclick = () => test('disk');
    request('/notifications/data').then(data => { fill(data.config); renderMonitor(data); $('notification-fields').disabled = false; $('save-btn').disabled = $('test-btn').disabled = $('disk-test-btn').disabled = false; status(''); }).catch(error => { status(window.RouterUI.errorMessage(error),true); $('monitor-state').textContent='Не удалось загрузить состояние'; });
    setInterval(() => { if (!document.hidden && !busy) request('/notifications/data').then(renderMonitor).catch(() => { $('monitor-state').textContent='Нет связи с сервером'; }); },15000);
})();
