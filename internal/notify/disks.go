package notify

import (
	"fmt"
	"math"
	"os"
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
				clog.Warnf("disk alert delivery failed: %v", err)
				continue
			}
			m.states[key] = diskAlertState{level: level, sent: now, seen: now}
		}
	}
}
