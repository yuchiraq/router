package stats

import (
	"encoding/json"
	"github.com/shirou/gopsutil/disk"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAutomaticDiskSelection(t *testing.T) {
	parts := []disk.PartitionStat{{Device: "/dev/root", Mountpoint: "/", Fstype: "ext4"}, {Device: "/dev/root", Mountpoint: "/var/lib/bind", Fstype: "ext4"}, {Device: "/dev/sdb1", Mountpoint: "/mnt/data", Fstype: "ext4"}, {Device: "/dev/sdc1", Mountpoint: "/home", Fstype: "btrfs"}, {Device: "tmpfs", Mountpoint: "/run", Fstype: "tmpfs"}, {Device: "/dev/loop1", Mountpoint: "/snap/app", Fstype: "squashfs"}}
	usage := func(p string) (*disk.UsageStat, error) {
		return &disk.UsageStat{Total: 100, Free: 20, UsedPercent: 80}, nil
	}
	entries, _ := collectDisks(parts, nil, usage, time.Now())
	if len(entries) != 3 {
		t.Fatalf("wrong devices: %+v", entries)
	}
	entries, _ = collectDisks(parts[:2], nil, usage, time.Now())
	if len(entries) != 1 {
		t.Fatal("unplugged disks retained", entries)
	}
}

func TestDiskSnapshotFreshness(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snapshot.json")
	now := time.Now()
	write := func(at time.Time) {
		b, _ := json.Marshal(DiskSnapshot{CollectedAt: at, Disks: []DiskUsage{{Time: at, Mountpoint: "/", Total: 100}}})
		if err := os.WriteFile(path, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(now)
	t.Setenv("ROUTER_DISK_SNAPSHOT", path)
	s := New()
	s.RecordDisks()
	if len(s.LatestDisks()) != 1 {
		t.Fatal("fresh snapshot not read")
	}
	write(now.Add(-3 * time.Minute))
	s.RecordDisks()
	if len(s.LatestDisks()) != 0 || len(s.UnavailableDisks()) == 0 {
		t.Fatal("stale metrics used for alerts")
	}
	write(now.Add(time.Hour))
	if _, err := readDiskSnapshot(path, now); err == nil {
		t.Fatal("future snapshot accepted")
	}
}
