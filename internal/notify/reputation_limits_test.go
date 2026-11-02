package notify

import (
	"fmt"
	"testing"
	"time"
)

func TestNotificationDedupeIsBounded(t *testing.T) {
	n := &TelegramNotifier{lastSentKey: map[string]time.Time{}, cooldown: time.Minute}
	for i := 0; i < 5500; i++ {
		n.shouldSkip(fmt.Sprint(i))
	}
	if len(n.lastSentKey) > 5000 {
		t.Fatal("unbounded attacker-controlled notification keys")
	}
	for key := range n.lastSentKey {
		n.lastSentKey[key] = time.Now().Add(-time.Hour)
	}
	if n.shouldSkip("new") || len(n.lastSentKey) != 1 {
		t.Fatal("expired notification keys not reclaimed")
	}
}
