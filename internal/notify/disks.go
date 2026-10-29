package notify

import (
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	"router/internal/clog"
	"router/internal/stats"
)

type diskAlertKey struct {
	device, mount string
	chat          int64
}
type diskAlertState struct {
	level      int
	sent, seen time.Time
}

// DiskMonitor is independent of HTTP traffic: closing the panel does not stop it.
// Delivery state is per chat; a failing recipient does not spam successful ones.
type DiskMonitor struct {
	mu       sync.Mutex
	notifier *TelegramNotifier
	states   map[diskAlertKey]diskAlertState
	host     string
	statusMu sync.RWMutex
	status   DiskMonitorStatus
}

type DiskMonitorStatus struct {
	Reason    string    `json:"reason"`
	LastCheck time.Time `json:"lastCheck"`
	LastSent  time.Time `json:"lastSent"`
	LastError string    `json:"lastError"`
	Timezone  string    `json:"timezone"`
}

func (m *DiskMonitor) Status(now time.Time) DiskMonitorStatus {
	m.statusMu.RLock()
	status := m.status
	m.statusMu.RUnlock()
	cfg := m.notifier.store.Get()
	status.Timezone = now.Format("MST (UTC-07:00)")
	switch {
	case !cfg.Enabled:
		status.Reason = "Отправка уведомлений выключена"
	case !cfg.Events["disk_usage"]:
		status.Reason = "Уведомления о заполнении выключены"
	case cfg.Token == "":
		status.Reason = "Не указан токен Telegram-бота"
	case len(targetChatIDs(cfg)) == 0:
		status.Reason = "Не выбраны получатели"
	case inQuietHours(now, cfg.QuietHoursOn, cfg.QuietHoursStart, cfg.QuietHoursEnd):
		status.Reason = "Отправка приостановлена на тихие часы"
	default:
		status.Reason = "Контроль заполнения включён"
	}
	return status
}

// Explicit test bypasses thresholds and quiet hours, without changing alert state.
func (n *TelegramNotifier) TestDiskMessage(samples []stats.DiskUsage) error {
	cfg := n.store.Get()
	chats := targetChatIDs(cfg)
	if cfg.Token == "" || len(chats) == 0 {
		return fmt.Errorf("Укажите токен бота и хотя бы одного получателя")
	}
	lines := []string{"Router: тест уведомлений о дисках (не авария)"}
	for _, sample := range samples {
		if sample.Total == 0 || time.Since(sample.Time) > 2*time.Minute {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s: занято %.1f%%, свободно %.2f ГиБ", sample.Mountpoint, sample.UsedPercent, float64(sample.Free)/(1<<30)))
	}
	if len(lines) == 1 {
		return fmt.Errorf("Нет свежих данных о дисках. Проверьте пути и дождитесь обновления показателей")
	}
	message := strings.Join(lines, "\n")
	var failed int
	for _, chat := range chats {
		if err := n.SendMessageToChat(chat, message); err != nil {
			failed++
		}
	}
	if failed > 0 {
		return fmt.Errorf("Тест не доставлен получателям: %d из %d. Проверьте токен, ID чатов и доступ бота", failed, len(chats))
	}
	return nil
}

func NewDiskMonitor(n *TelegramNotifier) *DiskMonitor {
	host, _ := os.Hostname()
	return &DiskMonitor{notifier: n, states: make(map[diskAlertKey]diskAlertState), host: host}
}

func diskLevel(percent float64, previous, warning, critical int) int {
	if percent >= float64(critical) || previous == 2 && percent >= float64(critical-3) {
		return 2
	}
	if percent >= float64(warning) || previous > 0 && percent >= float64(warning-5) {
		return 1
	}
	return 0
}

func (m *DiskMonitor) Check(now time.Time, samples []stats.DiskUsage) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cfg := m.notifier.store.Get()
	m.statusMu.Lock()
	m.status.LastCheck = now
	m.statusMu.Unlock()
	deliveryErrors := []string{}
	defer func() {
		m.statusMu.Lock()
		m.status.LastError = strings.Join(deliveryErrors, "; ")
		m.statusMu.Unlock()
	}()
	for key, state := range m.states {
		if now.Sub(state.seen) > 24*time.Hour {
			delete(m.states, key)
		}
	}
	if !cfg.Enabled || !cfg.Events["disk_usage"] || cfg.Token == "" || inQuietHours(now, cfg.QuietHoursOn, cfg.QuietHoursStart, cfg.QuietHoursEnd) {
		return
	}
	for _, sample := range samples {
		if sample.Total == 0 || sample.Mountpoint == "" || now.Sub(sample.Time) > 2*time.Minute || sample.Time.After(now) || math.IsNaN(sample.UsedPercent) || math.IsInf(sample.UsedPercent, 0) || sample.UsedPercent < 0 || sample.UsedPercent > 100 {
			continue
		}
		for _, chat := range targetChatIDs(cfg) {
			key := diskAlertKey{sample.Device, sample.Mountpoint, chat}
			state := m.states[key]
			level := diskLevel(sample.UsedPercent, state.level, cfg.DiskWarning, cfg.DiskCritical)
			state.seen = now
			m.states[key] = state
			if level == 0 && state.level == 0 {
				continue
			}
			if level == state.level && now.Sub(state.sent) < time.Duration(cfg.DiskReminderHours)*time.Hour {
				continue
			}
			title := "Мало места на диске"
			if level == 2 {
				title = "Критически мало места на диске"
			}
			if level == 0 {
				title = "Место на диске освобождено"
			}
			message := fmt.Sprintf("Router: %s\nСервер: %s\nПуть: %s\nЗанято: %.1f%%\nСвободно: %.2f ГиБ из %.2f ГиБ", title, m.host, sample.Mountpoint, sample.UsedPercent, float64(sample.Free)/(1<<30), float64(sample.Total)/(1<<30))
			if err := m.notifier.SendMessageToChat(chat, message); err != nil {
				deliveryErrors = append(deliveryErrors, fmt.Sprintf("Чат %d: %s", chat, err))
				clog.Warnf("disk alert delivery failed: %v", err)
				continue
			}
			m.states[key] = diskAlertState{level: level, sent: now, seen: now}
			m.statusMu.Lock()
			m.status.LastSent = now
			m.statusMu.Unlock()
		}
	}
}
