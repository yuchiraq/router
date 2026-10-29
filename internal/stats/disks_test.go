package stats

import (
	"errors"
	"github.com/shirou/gopsutil/disk"
	"reflect"
	"testing"
	"time"
)

func TestExplicitDiskPaths(t *testing.T) {
	parts := []disk.PartitionStat{{Mountpoint: "/", Device: "root"}, {Mountpoint: "/home", Device: "home"}, {Mountpoint: "/var", Device: "var"}, {Mountpoint: "/media/backup", Device: "backup"}}
	usage := func(p string) (*disk.UsageStat, error) {
		if p == "/missing" {
			return nil, errors.New("missing")
		}
		return &disk.UsageStat{Total: 100, Free: 15, UsedPercent: 85}, nil
	}
	entries, missing := collectDisks(parts, nil, usage, time.Now())
	if len(entries) != 2 || len(missing) != 0 {
		t.Fatal("default selection expanded", entries, missing)
	}
	entries, missing = collectDisks(parts, []string{"/mnt/archive", "/home/user", "/missing", "/mnt/archive"}, usage, time.Now())
	if len(entries) != 4 || !reflect.DeepEqual(missing, []string{"/missing"}) {
		t.Fatal("explicit paths not respected", entries, missing)
	}
	s := New()
	s.disks = entries
	copy := s.LatestDisks()
	copy[0].Mountpoint = "changed"
	if s.LatestDisks()[0].Mountpoint == "changed" {
		t.Fatal("snapshot alias")
	}
}
