package stats

import (
	"encoding/json"
	"fmt"
	"github.com/shirou/gopsutil/disk"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

type DiskSnapshot struct {
	CollectedAt time.Time   `json:"collectedAt"`
	Disks       []DiskUsage `json:"disks"`
	Unavailable []string    `json:"unavailable"`
}

func realFilesystem(p disk.PartitionStat) bool {
	if p.Mountpoint == "" || strings.HasPrefix(p.Device, "/dev/loop") {
		return false
	}
	switch p.Fstype {
	case "tmpfs", "devtmpfs", "proc", "sysfs", "squashfs", "iso9660", "cgroup", "cgroup2", "securityfs", "debugfs", "tracefs", "pstore", "efivarfs", "autofs", "mqueue", "hugetlbfs", "fusectl", "configfs", "devpts", "ramfs", "nsfs":
		return false
	case "overlay":
		return p.Mountpoint == "/"
	}
	return true
}

func collectDisks(partitions []disk.PartitionStat, extra []string, usageFor func(string) (*disk.UsageStat, error), now time.Time) ([]DiskUsage, []string) {
	selected := map[string]disk.PartitionStat{}
	for _, p := range partitions {
		if realFilesystem(p) {
			selected[p.Mountpoint] = p
		}
	}
	if runtime.GOOS != "windows" {
		if _, ok := selected["/"]; !ok {
			selected["/"] = disk.PartitionStat{Mountpoint: "/"}
		}
	}
	// Retain legacy explicit paths, but normal operation needs no configured paths.
	for _, path := range extra {
		if _, ok := selected[path]; ok {
			continue
		}
		best := disk.PartitionStat{Mountpoint: path}
		length := 0
		for _, p := range partitions {
			if realFilesystem(p) && (path == p.Mountpoint || strings.HasPrefix(strings.ReplaceAll(path, "\\", "/"), strings.TrimRight(strings.ReplaceAll(p.Mountpoint, "\\", "/"), "/")+"/")) && len(p.Mountpoint) > length {
				best.Device = p.Device
				best.Fstype = p.Fstype
				length = len(p.Mountpoint)
			}
		}
		selected[path] = best
	}
	paths := make([]string, 0, len(selected))
	for p := range selected {
		paths = append(paths, p)
	}
	sort.Slice(paths, func(i, j int) bool {
		if len(paths[i]) != len(paths[j]) {
			return len(paths[i]) < len(paths[j])
		}
		return paths[i] < paths[j]
	})
	entries := []DiskUsage{}
	unavailable := []string{}
	seen := map[string]bool{}
	for _, path := range paths {
		p := selected[path]
		key := p.Device + "|" + p.Fstype
		usage, err := usageFor(path)
		if err != nil || usage == nil || usage.Total == 0 {
			unavailable = append(unavailable, path)
			continue
		}
		if p.Device != "" && seen[key] {
			continue
		}
		if p.Fstype == "" {
			p.Fstype = usage.Fstype
		}
		entries = append(entries, DiskUsage{Time: now, Mountpoint: path, Device: p.Device, Fstype: p.Fstype, UsedPercent: usage.UsedPercent, Free: usage.Free, Total: usage.Total})
		if p.Device != "" {
			seen[key] = true
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Mountpoint < entries[j].Mountpoint })
	sort.Strings(unavailable)
	return entries, unavailable
}

func CollectDiskSnapshot(extra ...string) DiskSnapshot {
	now := time.Now()
	parts, err := disk.Partitions(false)
	entries, unavailable := collectDisks(parts, extra, disk.Usage, now)
	if err != nil {
		unavailable = append(unavailable, "Не удалось получить список дисков")
	}
	return DiskSnapshot{CollectedAt: now, Disks: entries, Unavailable: unavailable}
}

func WriteDiskSnapshot(path string) error {
	data, err := json.Marshal(CollectDiskSnapshot())
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".disks-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

func readDiskSnapshot(path string, now time.Time) (DiskSnapshot, error) {
	var snapshot DiskSnapshot
	f, err := os.Open(path)
	if err != nil {
		return snapshot, err
	}
	defer f.Close()
	if err = json.NewDecoder(io.LimitReader(f, 1<<20)).Decode(&snapshot); err != nil {
		return snapshot, err
	}
	if snapshot.CollectedAt.IsZero() || now.Sub(snapshot.CollectedAt) > 2*time.Minute || snapshot.CollectedAt.After(now.Add(5*time.Second)) {
		return DiskSnapshot{}, fmt.Errorf("disk snapshot is stale")
	}
	return snapshot, nil
}

func (s *Stats) RecordDisks(extra ...string) {
	var snapshot DiskSnapshot
	if path := os.Getenv("ROUTER_DISK_SNAPSHOT"); path != "" {
		var err error
		snapshot, err = readDiskSnapshot(path, time.Now())
		if err != nil {
			snapshot = DiskSnapshot{Unavailable: []string{"Сборщик дисков: нет свежих данных"}}
		}
	} else {
		snapshot = CollectDiskSnapshot(extra...)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.disks = snapshot.Disks
	s.diskUnavailable = snapshot.Unavailable
}
