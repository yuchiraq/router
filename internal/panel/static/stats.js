
            (function () {
                var memoryCanvas = document.getElementById('memory-chart');
                var cpuCanvas = document.getElementById('cpu-chart');
                var requestsCanvas = document.getElementById('requests-chart');
                if (!memoryCanvas || !cpuCanvas || !requestsCanvas) {
                    console.warn('Charts not initialized: missing canvas elements.');
                    return;
                }

                var memoryContext = memoryCanvas.getContext('2d');
                var cpuContext = cpuCanvas.getContext('2d');
                var requestsContext = requestsCanvas.getContext('2d');

                if (!memoryContext || !cpuContext || !requestsContext) {
                    console.warn('Charts not initialized: missing canvas contexts.');
                    return;
                }

                // --- Chart Configuration ---
                var colorHash = new ColorHash({lightness: 0.6, saturation: 0.8});
                var styles = getComputedStyle(document.documentElement);
                var chartGrid = styles.getPropertyValue('--chart-grid').trim() || 'rgba(255, 255, 255, 0.1)';
                var chartTick = styles.getPropertyValue('--chart-tick').trim() || '#a0a0a5';

                var chartOptions = {
                    animation: false,
                    responsive: true,
                    maintainAspectRatio: false,
                    scales: {
                        x: {
                            grid: { color: chartGrid },
                            ticks: { color: chartTick }
                        },
                        y: {
                            grid: { color: chartGrid },
                            ticks: { color: chartTick },
                            beginAtZero: true
                        }
                    },
                    plugins: {
                        legend: {
                            labels: { color: chartTick }
                        },
                        tooltip: {
                            callbacks: {}
                        }
                    },
                    interaction: {
                        intersect: false,
                        mode: 'index'
                    },
                    tension: 0.4
                };

                // --- Memory Chart ---
                var memoryChartOptions = {
                    animation: false,
                    responsive: chartOptions.responsive,
                    maintainAspectRatio: chartOptions.maintainAspectRatio,
                    scales: {
                        x: {
                            grid: { color: chartGrid },
                            ticks: { color: chartTick }
                        },
                        y: {
                            type: 'linear',
                            display: true,
                            position: 'left',
                            grid: { color: chartGrid },
                            ticks: { color: '#32876d' },
                            beginAtZero: true,
                            suggestedMax: 100,
                            title: { display: true, text: '%', color: '#32876d' }
                        }
                    },
                    plugins: {
                        legend: chartOptions.plugins.legend,
                        tooltip: {
                            callbacks: {
                                label: function(context) {
                                    var label = context.dataset.label || '';
                                    if (label) { label += ': '; }
                                    if (context.parsed.y !== null) {
                                        label += context.parsed.y.toFixed(2) + ' %';
                                    }
                                    return label;
                                }
                            }
                        }
                    },
                    interaction: chartOptions.interaction,
                    tension: chartOptions.tension
                };

                var memoryChart = new Chart(memoryContext, {
                    type: 'line',
                    data: {
                        labels: [],
                        datasets: [{
                            label: 'Занято (%)',
                            data: [],
                            borderColor: '#32876d',
                            backgroundColor: 'rgba(50, 135, 109, 0.12)',
                            fill: true,
                            yAxisID: 'y'
                        }]
                    },
                    options: memoryChartOptions
                });

                // --- CPU Chart ---
                var cpuChartOptions = {
                    animation: false,
                    responsive: chartOptions.responsive,
                    maintainAspectRatio: chartOptions.maintainAspectRatio,
                    scales: {
                        x: {
                            grid: { color: chartGrid },
                            ticks: { color: chartTick }
                        },
                        y: {
                            type: 'linear',
                            display: true,
                            position: 'left',
                            grid: { color: chartGrid },
                            ticks: { color: '#927136' },
                            beginAtZero: true,
                            suggestedMax: 100,
                            title: { display: true, text: '%', color: '#927136' }
                        }
                    },
                    plugins: {
                        legend: chartOptions.plugins.legend,
                        tooltip: {
                            callbacks: {
                                label: function(context) {
                                    var label = context.dataset.label || '';
                                    if (label) { label += ': '; }
                                    if (context.parsed.y !== null) {
                                        label += context.parsed.y.toFixed(2) + ' %';
                                    }
                                    return label;
                                }
                            }
                        }
                    },
                    interaction: chartOptions.interaction,
                    tension: chartOptions.tension
                };

                var cpuChart = new Chart(cpuContext, {
                    type: 'line',
                    data: {
                        labels: [],
                        datasets: [{
                            label: 'Занято (%)',
                            data: [],
                            borderColor: '#927136',
                            backgroundColor: 'rgba(146, 113, 54, 0.12)',
                            fill: true,
                            yAxisID: 'y'
                        }]
                    },
                    options: cpuChartOptions
                });

                // --- Requests Chart ---
                var requestsChart = new Chart(requestsContext, {
                    type: 'line',
                    data: { labels: [], datasets: [] },
                    options: chartOptions
                });

                var canvas = document.getElementById('dashboard-canvas');
                var widgets = canvas ? Array.prototype.slice.call(canvas.querySelectorAll('.dashboard-widget')) : [];
                var layoutKey = 'router_stats_layout_v2';
                var mode = document.getElementById('layout-mode');
                try { mode.value = localStorage.getItem('router_stats_mode') === 'free' ? 'free' : 'grid'; } catch (_) {}
                function applyMode() {
                    canvas.classList.toggle('free-layout', mode.value === 'free' && window.innerWidth > 1100);
                    if (canvas.classList.contains('free-layout')) widgets.forEach(function(widget) { widget.style.width = Math.min(parseInt(widget.style.width)||360,canvas.clientWidth)+'px'; widget.style.left = Math.max(0,Math.min(parseInt(widget.style.left)||0,canvas.clientWidth-widget.offsetWidth))+'px'; });
                    memoryChart.resize(); cpuChart.resize(); requestsChart.resize();
                }
                mode.addEventListener('change', function() { try { localStorage.setItem('router_stats_mode', mode.value); } catch (_) {} applyMode(); });
                window.addEventListener('resize', applyMode);
                applyMode();
                window.addEventListener('router-theme-change', function() {
                    var colors = getComputedStyle(document.documentElement);
                    [memoryChart,cpuChart].forEach(function(chart,index) {
                        var color = colors.getPropertyValue(index === 0 ? '--accent-green' : '--accent-amber').trim();
                        chart.data.datasets[0].borderColor = color;
                        chart.data.datasets[0].backgroundColor = color + '20';
                        chart.options.scales.y.title.color = color;
                    });
                    [memoryChart,cpuChart,requestsChart].forEach(function(chart) {
                        Object.values(chart.options.scales).forEach(function(scale) { scale.grid.color = colors.getPropertyValue('--chart-grid').trim(); scale.ticks.color = colors.getPropertyValue('--chart-tick').trim(); });
                        chart.options.plugins.legend.labels.color = colors.getPropertyValue('--chart-tick').trim(); chart.update();
                    });
                });

                function clamp(value, min, max) {
                    return Math.max(min, Math.min(max, value));
                }

                function saveLayout() {
                    var payload = [];
                    for (var i = 0; i < widgets.length; i++) {
                        payload.push({
                            id: widgets[i].dataset.widgetId,
                            left: parseInt(widgets[i].style.left || '0', 10),
                            top: parseInt(widgets[i].style.top || '0', 10),
                            width: parseInt(widgets[i].style.width || '320', 10),
                            height: parseInt(widgets[i].style.height || '260', 10),
                            z: parseInt(widgets[i].style.zIndex || '1', 10)
                        });
                    }
                    try { localStorage.setItem(layoutKey, JSON.stringify(payload)); } catch (_) {}
                }

                function refreshCanvasHeight() {
                    var maxBottom = 0;
                    for (var i = 0; i < widgets.length; i++) {
                        var top = parseInt(widgets[i].style.top || '0', 10);
                        var h = parseInt(widgets[i].style.height || '260', 10);
                        maxBottom = Math.max(maxBottom, top + h);
                    }
                    canvas.style.minHeight = String(maxBottom + 40) + 'px';
                }

                function restoreLayout() {
                    var savedRaw;
                    try { savedRaw = localStorage.getItem(layoutKey); } catch (_) {}
                    if (!savedRaw) {
                        var columns = canvas.clientWidth > 1200 ? 3 : 2;
                        var width = (canvas.clientWidth - (columns-1)*16) / columns;
                        widgets.forEach(function(widget, index) { widget.style.left = ((index%columns)*(width+16))+'px'; widget.style.top = (Math.floor(index/columns)*356)+'px'; widget.style.width = width+'px'; widget.style.height = '340px'; });
                        refreshCanvasHeight();
                        return;
                    }
                    try {
                        var saved = JSON.parse(savedRaw);
                        for (var i = 0; i < saved.length; i++) {
                            for (var j = 0; j < widgets.length; j++) {
                                if (widgets[j].dataset.widgetId === saved[i].id) {
                                    widgets[j].style.left = String(saved[i].left || 0) + 'px';
                                    widgets[j].style.top = String(saved[i].top || 0) + 'px';
                                    widgets[j].style.width = String(saved[i].width || 360) + 'px';
                                    widgets[j].style.height = String(saved[i].height || 260) + 'px';
                                    widgets[j].style.zIndex = String(saved[i].z || 1);
                                }
                            }
                        }
                    } catch (err) {
                        console.warn('layout restore failed', err);
                    }
                    refreshCanvasHeight();
                }

                function bringToFront(widget) {
                    var maxZ = 1;
                    for (var i = 0; i < widgets.length; i++) {
                        maxZ = Math.max(maxZ, parseInt(widgets[i].style.zIndex || '1', 10));
                    }
                    widget.style.zIndex = String(maxZ + 1);
                }

                function bindDrag(widget) {
                    var handle = widget.querySelector('.card-header');
                    if (!handle) return;
                    handle.addEventListener('mousedown', function(ev) {
                        if (!canvas.classList.contains('free-layout')) return;
                        if (ev.target && ev.target.classList && ev.target.classList.contains('widget-resizer')) return;
                        bringToFront(widget);
                        widget.classList.add('dragging');
                        var startX = ev.clientX;
                        var startY = ev.clientY;
                        var startLeft = parseInt(widget.style.left || '0', 10);
                        var startTop = parseInt(widget.style.top || '0', 10);

                        function onMove(moveEv) {
                            var nextLeft = clamp(startLeft + (moveEv.clientX - startX), 0, Math.max(0, canvas.clientWidth-widget.offsetWidth));
                            var nextTop = Math.max(0, startTop + (moveEv.clientY - startY));
                            widget.style.left = String(nextLeft) + 'px';
                            widget.style.top = String(nextTop) + 'px';
                            refreshCanvasHeight();
                        }

                        function onUp() {
                            widget.classList.remove('dragging');
                            window.removeEventListener('mousemove', onMove);
                            window.removeEventListener('mouseup', onUp);
                            saveLayout();
                        }

                        window.addEventListener('mousemove', onMove);
                        window.addEventListener('mouseup', onUp);
                    });
                }

                function bindResize(widget) {
                    var resizer = widget.querySelector('.widget-resizer');
                    if (!resizer) return;
                    resizer.addEventListener('mousedown', function(ev) {
                        if (!canvas.classList.contains('free-layout')) return;
                        ev.preventDefault();
                        ev.stopPropagation();
                        bringToFront(widget);
                        var startX = ev.clientX;
                        var startY = ev.clientY;
                        var startW = parseInt(widget.style.width || '360', 10);
                        var startH = parseInt(widget.style.height || '260', 10);

                        function onMove(moveEv) {
                            var nextW = clamp(startW + (moveEv.clientX - startX), 260, Math.max(320, canvas.clientWidth - 20));
                            var nextH = clamp(startH + (moveEv.clientY - startY), 220, 900);
                            widget.style.width = String(nextW) + 'px';
                            widget.style.height = String(nextH) + 'px';
                            memoryChart.resize();
                            cpuChart.resize();
                            requestsChart.resize();
                            refreshCanvasHeight();
                        }

                        function onUp() {
                            window.removeEventListener('mousemove', onMove);
                            window.removeEventListener('mouseup', onUp);
                            saveLayout();
                        }

                        window.addEventListener('mousemove', onMove);
                        window.addEventListener('mouseup', onUp);
                    });
                }

                for (var wi = 0; wi < widgets.length; wi++) {
                    bindDrag(widgets[wi]);
                    bindResize(widgets[wi]);
                }

                var resetLayoutBtn = document.getElementById('reset-layout-btn');
                if (resetLayoutBtn) {
                    resetLayoutBtn.addEventListener('click', function() {
                        try { localStorage.removeItem(layoutKey); } catch (_) {}
                        window.location.reload();
                    });
                }

                restoreLayout();
                applyMode();

                // --- Data Fetching ---

                function banIP(ip) {
                    return fetch('/stats/ban', {
                        method: 'POST',
                        credentials: 'same-origin',
                        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
                        body: 'ip=' + encodeURIComponent(ip)
                    }).then(function(response) {
                        if (!response.ok) {
                            throw new Error('Не удалось заблокировать IP ' + ip);
                        }
                    });
                }

                function unbanIP(ip) {
                    return fetch('/stats/unban', {
                        method: 'POST',
                        credentials: 'same-origin',
                        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
                        body: 'ip=' + encodeURIComponent(ip)
                    }).then(function(response) {
                        if (!response.ok) {
                            throw new Error('Не удалось снять блокировку IP ' + ip);
                        }
                    });
                }

                function removeIP(ip) {
                    return fetch('/stats/remove', {
                        method: 'POST',
                        credentials: 'same-origin',
                        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
                        body: 'ip=' + encodeURIComponent(ip)
                    }).then(function(response) {
                        if (!response.ok) {
                            throw new Error('Не удалось удалить IP ' + ip);
                        }
                    });
                }

                var fetching = false;
                function reasonLabel(reason) { return {'unknown host':'Неизвестный домен','suspicious path probe':'Подозрительный запрос','manual ban':'Ручная блокировка','too many suspicious requests':'Много подозрительных запросов'}[reason] || reason; }
                function countryLabel(code,fallback) {
                    if (code === 'LAN' || code === 'LOCAL') return 'Локальная сеть';
                    if (!code || code === 'Unknown' || code === 'XX') return 'Не определена';
                    try { return new Intl.DisplayNames(['ru'],{type:'region'}).of(code) || fallback; } catch (_) { return fallback === 'Unknown' ? 'Не определена' : fallback; }
                }
                function escapeHTML(value) { return String(value ?? '').replace(/[&<>"']/g, function(c) { return {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]; }); }
                function fetchData() {
                    if (fetching || document.hidden) return;
                    fetching = true;
                    return fetch('/stats/data', { credentials: 'same-origin', headers: {Accept:'application/json'} })
                        .then(function(response) {
                            if (!response.ok) {
                                throw new Error('Ошибка загрузки показателей: ' + response.status);
                            }
                            return response.json();
                        })
                        .then(function(data) {
                            document.getElementById('stats-status').textContent = data.reputationError ? 'Не удалось сохранить список IP. Проверьте свободное место и права записи.' : 'Обновлено в ' + new Date().toLocaleTimeString('ru-RU');
                            if (data.memory && data.memory.labels.length > 0) {
                                memoryChart.data.labels = data.memory.labels;
                                memoryChart.data.datasets[0].data = data.memory.percents;
                                memoryChart.update();
                                memoryCanvas.setAttribute('aria-label','Оперативная память: занято '+Number(data.memory.percents.at(-1)).toFixed(1)+'%');
                            }

                            if (data.cpu && data.cpu.labels.length > 0) {
                                cpuChart.data.labels = data.cpu.labels;
                                cpuChart.data.datasets[0].data = data.cpu.percents;
                                cpuChart.update();
                                cpuCanvas.setAttribute('aria-label','Процессор: загрузка '+Number(data.cpu.percents.at(-1)).toFixed(1)+'%');
                            }

                            if (data.requests && data.requests.datasets) {
                                requestsChart.data.labels = data.requests.labels;

                                for (var i = 0; i < data.requests.datasets.length; i++) {
                                    var newDataset = data.requests.datasets[i];
                                    var existingDataset = null;

                                    for (var j = 0; j < requestsChart.data.datasets.length; j++) {
                                        if (requestsChart.data.datasets[j].label === newDataset.label) {
                                            existingDataset = requestsChart.data.datasets[j];
                                            break;
                                        }
                                    }

                                    if (existingDataset) {
                                        existingDataset.data = newDataset.data;
                                    } else {
                                        var color = colorHash.hex(newDataset.label);
                                        requestsChart.data.datasets.push({
                                            label: newDataset.label,
                                            data: newDataset.data,
                                            borderColor: color,
                                            backgroundColor: color + '33',
                                            fill: true
                                        });
                                    }
                                }

                                var filteredDatasets = [];
                                for (var k = 0; k < requestsChart.data.datasets.length; k++) {
                                    var current = requestsChart.data.datasets[k];
                                    var found = false;
                                    for (var n = 0; n < data.requests.datasets.length; n++) {
                                        if (data.requests.datasets[n].label === current.label) {
                                            found = true;
                                            break;
                                        }
                                    }
                                    if (found) {
                                        filteredDatasets.push(current);
                                    }
                                }
                                requestsChart.data.datasets = filteredDatasets;

                                requestsChart.update();
                            }


                            if (data.ssh) {
                                var sshCurrent = document.getElementById('ssh-current');
                                sshCurrent.textContent = 'Активных подключений: ' + (data.ssh.current || 0);

                                var sshTable = document.getElementById('ssh-table');
                                var sshSessions = data.ssh.sessions || [];
                                var sshRows = '';
                                for (var si = 0; si < sshSessions.length; si++) {
                                    var session = sshSessions[si];
                                    sshRows += '<div class="ssh-row">' +
                                        '<div class="ssh-ip">' + escapeHTML(session.ip) + ':' + session.port + '</div>' +
                                        '<div class="ssh-country">' + escapeHTML(countryLabel(session.countryCode,session.countryName)) + ' (' + escapeHTML(session.countryCode) + ')</div>' +
                                        '<div class="ssh-date">' + escapeHTML(session.date) + '</div>' +
                                        '<div class="ssh-time">' + escapeHTML(session.time) + '</div>' +
                                        '<div class="ssh-device">' + escapeHTML(session.device) + '</div>' +
                                    '</div>';
                                }
                                sshTable.innerHTML = sshRows || '<div class="disk-empty">Нет активных SSH-подключений</div>';
                            }

                            if (data.countries) {
                                var countryTable = document.getElementById('country-table');
                                var countryRows = '';
                                for (var ci = 0; ci < data.countries.length; ci++) {
                                    var country = data.countries[ci];
                                    countryRows += '<div class="country-row">' +
                                        '<div class="country-name">' + escapeHTML(countryLabel(country.code,country.name)) + ' <span>(' + escapeHTML(country.code) + ')</span></div>' +
                                        '<div class="country-count">' + country.count + '</div>' +
                                    '</div>';
                                }
                                countryTable.innerHTML = countryRows || '<div class="disk-empty">Запросов пока нет</div>';
                            }

                            if (data.disks) {
                                var table = document.getElementById('disk-table');
                                var rows = '';
                                for (var di = 0; di < data.disks.length; di++) {
                                    var disk = data.disks[di];
                                    rows += '<div class="disk-row">' +
                                        '<div class="disk-main">' +
                                            '<div class="disk-title">' + escapeHTML(disk.mountpoint) + '</div>' +
                                            '<div class="disk-subtitle">' + [disk.device,disk.fstype].filter(Boolean).map(escapeHTML).join(' • ') + '</div>' +
                                        '</div>' +
                                        '<div class="disk-metrics">' +
                                            '<div><strong>' + disk.usedPercent.toFixed(1) + '%</strong> занято</div>' +
                                            '<div>' + disk.freeGB.toFixed(1) + ' ГиБ свободно / ' + disk.totalGB.toFixed(1) + ' ГиБ всего</div>' +
                                        '</div>' +
                                        '<div class="disk-bar">' +
                                            '<div class="disk-bar-fill ' + (disk.usedPercent >= data.diskCritical ? 'critical' : disk.usedPercent >= data.diskWarning ? 'warning' : '') + '" style="width: ' + Math.min(100,Math.max(0,disk.usedPercent)).toFixed(1) + '%"></div>' +
                                        '</div>' +
                                    '</div>';
                                }
                                (data.diskUnavailable || []).forEach(function(p) { rows += '<div class="disk-row"><div class="disk-title">'+escapeHTML(p)+'</div><div class="disk-subtitle">Недоступен: проверьте путь и права</div></div>'; });
                                table.innerHTML = (rows || '<div class="disk-empty">Нет данных о дисках</div>') + '<a href="/notifications#disk-settings">Настроить отслеживание</a>';
                            }

                            if (data.suspicious) {
                                var suspiciousTable = document.getElementById('suspicious-table');
                                var suspiciousRows = '';
                                for (var xi = 0; xi < data.suspicious.length; xi++) {
                                    var item = data.suspicious[xi];
                                    var action = item.banned
                                        ? '<button class="btn" data-unban-ip="' + escapeHTML(item.ip) + '">Разблокировать</button>'
                                        : '<button class="btn btn-danger" data-ban-ip="' + escapeHTML(item.ip) + '">Заблокировать</button>';
                                    if (!item.banned) {
                                        action += ' <button class="btn btn-icon" title="Удалить из списка" aria-label="Удалить из списка" data-remove-ip="' + escapeHTML(item.ip) + '">✕</button>';
                                    }
                                    suspiciousRows += '<div class="disk-row">' +
                                        '<div class="disk-main">' +
                                            '<div class="disk-title">' + escapeHTML(item.ip) + '</div>' +
                                            '<div class="disk-subtitle">' + escapeHTML(reasonLabel(item.reason)) + ' • запросов: ' + item.count + (item.autoBanned ? ' · автоблокировка' : '') + '</div>' +
                                        '</div>' +
                                        '<div class="disk-metrics">' +
                                            '<div>Первый: ' + new Date(item.firstSeen).toLocaleString('ru-RU') + '</div>' +
                                            '<div>Последний: ' + new Date(item.lastSeen).toLocaleString('ru-RU') + '</div>' +
                                        '</div>' +
                                        '<div>' + action + '</div>' +
                                    '</div>';
                                }
                                suspiciousTable.innerHTML = suspiciousRows || '<div class="disk-empty">Подозрительных IP нет</div>';

                                var banButtons = suspiciousTable.querySelectorAll('[data-ban-ip]');
                                for (var bi = 0; bi < banButtons.length; bi++) {
                                    banButtons[bi].addEventListener('click', function(ev) {
                                        var ip = ev.currentTarget.getAttribute('data-ban-ip');
                                        banIP(ip)
                                            .then(fetchData)
                                            .catch(function(error) { window.RouterUI.announce(window.RouterUI.errorMessage(error),true); });
                                    });
                                }

                                var unbanButtons = suspiciousTable.querySelectorAll('[data-unban-ip]');
                                for (var ui = 0; ui < unbanButtons.length; ui++) {
                                    unbanButtons[ui].addEventListener('click', function(ev) {
                                        var ip = ev.currentTarget.getAttribute('data-unban-ip');
                                        unbanIP(ip)
                                            .then(fetchData)
                                            .catch(function(error) { window.RouterUI.announce(window.RouterUI.errorMessage(error),true); });
                                    });
                                }

                                var removeButtons = suspiciousTable.querySelectorAll('[data-remove-ip]');
                                for (var ri = 0; ri < removeButtons.length; ri++) {
                                    removeButtons[ri].addEventListener('click', function(ev) {
                                        var ip = ev.currentTarget.getAttribute('data-remove-ip');
                                        removeIP(ip)
                                            .then(fetchData)
                                            .catch(function(error) { window.RouterUI.announce(window.RouterUI.errorMessage(error),true); });
                                    });
                                }
                            }

                            if (data.autoBanned) {
                                var autoTable = document.getElementById('auto-banned-table');
                                var autoRows = '';
                                for (var ai = 0; ai < data.autoBanned.length; ai++) {
                                    var ban = data.autoBanned[ai];
                                    autoRows += '<div class="disk-row">' +
                                        '<div class="disk-main">' +
                                            '<div class="disk-title">' + escapeHTML(ban.ip) + '</div>' +
                                            '<div class="disk-subtitle">' + escapeHTML(reasonLabel(ban.reason)) + ' • запросов: ' + ban.count + '</div>' +
                                        '</div>' +
                                        '<div class="disk-metrics">' +
                                            '<div>Блокировка: ' + new Date(ban.bannedAt).toLocaleString('ru-RU') + '</div>' +
                                            '<div>До: ' + new Date(ban.banUntil).toLocaleString('ru-RU') + '</div>' +
                                        '</div>' +
                                        '<div><button class="btn" data-unban-ip="' + escapeHTML(ban.ip) + '">Разблокировать</button></div>' +
                                    '</div>';
                                }
                                autoTable.innerHTML = autoRows || '<div class="disk-empty">Нет автоматических блокировок</div>';

                                var autoUnbanButtons = autoTable.querySelectorAll('[data-unban-ip]');
                                for (var aui = 0; aui < autoUnbanButtons.length; aui++) {
                                    autoUnbanButtons[aui].addEventListener('click', function(ev) {
                                        var ip = ev.currentTarget.getAttribute('data-unban-ip');
                                        unbanIP(ip)
                                            .then(fetchData)
                                            .catch(function(error) { window.RouterUI.announce(window.RouterUI.errorMessage(error),true); });
                                    });
                                }
                            }
                        })
                        .catch(function(error) {
                            document.getElementById('stats-status').textContent = 'Не удалось обновить показатели. Повторная попытка автоматически.';
                            console.error('Error fetching stats data:', error);
                        }).finally(function() { fetching = false; });
                }

                var manualBanBtn = document.getElementById('manual-ban-btn');
                var manualBanInput = document.getElementById('manual-ban-ip');
                if (manualBanBtn && manualBanInput) {
                    manualBanBtn.addEventListener('click', function() {
                        var ip = manualBanInput.value.trim();
                        if (!ip) {
                            return;
                        }
                        banIP(ip)
                            .then(function() {
                                manualBanInput.value = '';
                                return fetchData();
                            })
                            .catch(function(error) { window.RouterUI.announce(window.RouterUI.errorMessage(error),true); });
                    });
                }

                setInterval(fetchData, 3000);
                fetchData();
            })();
