package stats

import (
	"log"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
	netutil "github.com/shirou/gopsutil/net"
)

// Request represents a single request entry with its host
type Request struct {
	Time    time.Time
	Host    string
	Country string
}

// Memory represents a single memory usage entry
type Memory struct {
	Time    time.Time
	Used    uint64  // Used memory in MB
	Percent float64 // Used memory in percentage
}

// CPU represents a single CPU usage entry
type CPU struct {
	Time    time.Time
	Percent float64 // Used CPU in percentage
}

// DiskUsage represents a single disk usage entry
type DiskUsage struct {
	Time        time.Time
	Mountpoint  string
	Device      string
	Fstype      string
	UsedPercent float64
	Free        uint64
	Total       uint64
}

// SSHConnections represents a snapshot of SSH connections.
type SSHConnections struct {
	Time        time.Time
	Established int
	ByRemoteIP  map[string]int
}

type connectionFetcher func(kind string) ([]netutil.ConnectionStat, error)

// Stats holds the collected statistics
type Stats struct {
	mu              sync.RWMutex
	requests        []Request
	memory          []Memory
	cpu             []CPU
	disks           []DiskUsage
	ssh             []SSHConnections
	countryStats    map[string]int
	listConnections connectionFetcher
}

// New creates a new Stats instance
func New() *Stats {
	return &Stats{
		requests:        make([]Request, 0, 10000), // Pre-allocate for performance
		memory:          make([]Memory, 0, 1000),   // Pre-allocate
		cpu:             make([]CPU, 0, 1000),      // Pre-allocate
		disks:           make([]DiskUsage, 0, 4000),
		ssh:             make([]SSHConnections, 0, 1000),
		countryStats:    make(map[string]int),
		listConnections: netutil.Connections,
	}
}

// AddRequest adds a new request to the stats, including host and country.
func (s *Stats) AddRequest(host, country string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	country = NormalizeCountry(country)
	s.requests = append(s.requests, Request{Time: time.Now(), Host: host, Country: country})
	s.countryStats[country]++
}

// RecordMemory records the current memory usage (both absolute and percentage)
func (s *Stats) RecordMemory() {
	// Get system memory stats
	v, _ := mem.VirtualMemory()

	s.mu.Lock()
	defer s.mu.Unlock()

	// Store total system memory usage
	s.memory = append(s.memory, Memory{
		Time:    time.Now(),
		Used:    v.Used / 1024 / 1024, // Total system memory used in MB
		Percent: v.UsedPercent,        // Total system memory used percentage
	})

	// Optional: Keep memory slice from growing indefinitely
	if len(s.memory) > 1000 {
		s.memory = s.memory[len(s.memory)-1000:]
	}
}

// RecordCPU records the current CPU usage
func (s *Stats) RecordCPU() {
	percent, err := cpu.Percent(0, false)
	if err != nil || len(percent) == 0 {
		percent, err = cpu.Percent(time.Second, false)
		if err != nil || len(percent) == 0 {
			return
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.cpu = append(s.cpu, CPU{
		Time:    time.Now(),
		Percent: percent[0],
	})

	// Optional: Keep CPU slice from growing indefinitely
	if len(s.cpu) > 1000 {
		s.cpu = s.cpu[len(s.cpu)-1000:]
	}
}

// RecordDisks records usage for all mounted disks.
func (s *Stats) RecordDisks() {
	partitions, err := disk.Partitions(false)
	if err != nil {
		return
	}

	now := time.Now()
	entries := make([]DiskUsage, 0, len(partitions))
	for _, partition := range partitions {
		if partition.Mountpoint == "" || partition.Device == "" {
			continue
		}
		if partition.Mountpoint != "/" && !strings.HasPrefix(partition.Mountpoint, "/media/") {
			continue
		}
		usage, err := disk.Usage(partition.Mountpoint)
		if err != nil {
			continue
		}
		entries = append(entries, DiskUsage{
			Time:        now,
			Mountpoint:  partition.Mountpoint,
			Device:      partition.Device,
			Fstype:      partition.Fstype,
			UsedPercent: usage.UsedPercent,
			Free:        usage.Free,
			Total:       usage.Total,
		})
	}

	if len(entries) == 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.disks = append(s.disks, entries...)
	if len(s.disks) > 4000 {
		s.disks = s.disks[len(s.disks)-4000:]
	}
}

// RecordSSHConnections records current established SSH sessions on port 22.
func (s *Stats) RecordSSHConnections() {
	listConnections := s.listConnections
	if listConnections == nil {
		listConnections = netutil.Connections
	}

	conns, err := listConnections("tcp")
	remoteCounts := make(map[string]int)
	established := 0
	if err != nil {
		log.Printf("[WARN] Unable to collect SSH connections: %v", err)
		s.appendSSHSample(0, remoteCounts)
		return
	}

	for _, conn := range conns {
		if conn.Laddr.Port != 22 {
			continue
		}
		if strings.ToUpper(conn.Status) != "ESTABLISHED" {
			continue
		}

		established++
		remoteIP := normalizeRemoteIP(conn.Raddr.IP)
		if remoteIP != "" {
			remoteCounts[remoteIP]++
		}
	}

	s.appendSSHSample(established, remoteCounts)
}

func (s *Stats) appendSSHSample(established int, remoteCounts map[string]int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ssh = append(s.ssh, SSHConnections{
		Time:        time.Now(),
		Established: established,
		ByRemoteIP:  remoteCounts,
	})

	if len(s.ssh) > 1000 {
		s.ssh = s.ssh[len(s.ssh)-1000:]
	}
}

func normalizeRemoteIP(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	ip := net.ParseIP(raw)
	if ip == nil {
		return raw
	}
	return ip.String()
}

// GetSSHData returns SSH connection history and current remote IP table.
func (s *Stats) GetSSHData() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	labels := make([]string, len(s.ssh))
	values := make([]int, len(s.ssh))

	latestIPs := make(map[string]int)
	if len(s.ssh) > 0 {
		for ip, cnt := range s.ssh[len(s.ssh)-1].ByRemoteIP {
			latestIPs[ip] = cnt
		}
	}

	for i, sample := range s.ssh {
		labels[i] = sample.Time.Format("15:04:05")
		values[i] = sample.Established
	}

	ipRows := make([]map[string]interface{}, 0, len(latestIPs))
	for ip, cnt := range latestIPs {
		ipRows = append(ipRows, map[string]interface{}{
			"ip":    ip,
			"count": cnt,
		})
	}

	sort.Slice(ipRows, func(i, j int) bool {
		ci := ipRows[i]["count"].(int)
		cj := ipRows[j]["count"].(int)
		if ci == cj {
			return ipRows[i]["ip"].(string) < ipRows[j]["ip"].(string)
		}
		return ci > cj
	})

	current := 0
	if len(values) > 0 {
		current = values[len(values)-1]
	}

	return map[string]interface{}{
		"labels":  labels,
		"values":  values,
		"current": current,
		"clients": ipRows,
	}
}

// GetDiskData returns latest disk usage for tracked mountpoints.
func (s *Stats) GetDiskData() []map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	latestByMount := make(map[string]DiskUsage)

	for _, entry := range s.disks {
		latestByMount[entry.Mountpoint] = entry
	}

	latest := []map[string]interface{}{}
	for mountpoint, entry := range latestByMount {
		latest = append(latest, map[string]interface{}{
			"mountpoint":  mountpoint,
			"device":      entry.Device,
			"fstype":      entry.Fstype,
			"usedPercent": entry.UsedPercent,
			"freeGB":      float64(entry.Free) / 1024 / 1024 / 1024,
			"totalGB":     float64(entry.Total) / 1024 / 1024 / 1024,
		})
	}

	return latest
}

// GetRequestData returns request data grouped by host for charting
func (s *Stats) GetRequestData() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	datasets := make(map[string]map[int]int)
	hosts := []string{}
	now := time.Now()

	// Aggregate requests by host and hour
	for _, r := range s.requests {
		if r.Time.After(now.Add(-24 * time.Hour)) {
			if _, ok := datasets[r.Host]; !ok {
				datasets[r.Host] = make(map[int]int)
				hosts = append(hosts, r.Host)
			}
			datasets[r.Host][r.Time.Hour()]++
		}
	}

	// Prepare data for Chart.js
	chartData := make(map[string]interface{})
	labels := []string{}
	for i := 0; i < 24; i++ {
		hour := (now.Hour() - (23 - i) + 24) % 24
		labels = append(labels, time.Date(0, 0, 0, hour, 0, 0, 0, time.UTC).Format("15:04"))
	}
	chartData["labels"] = labels

	chartDatasets := []interface{}{}
	for _, host := range hosts {
		values := []int{}
		for i := 0; i < 24; i++ {
			hour := (now.Hour() - (23 - i) + 24) % 24
			values = append(values, datasets[host][hour])
		}
		chartDatasets = append(chartDatasets, map[string]interface{}{
			"label": host,
			"data":  values,
		})
	}
	chartData["datasets"] = chartDatasets

	return chartData
}

// GetMemoryData returns memory data for charting (absolute and percentage)
func (s *Stats) GetMemoryData() ([]string, []uint64, []float64) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	labels := make([]string, len(s.memory))
	values := make([]uint64, len(s.memory))
	percents := make([]float64, len(s.memory))

	for i, m := range s.memory {
		labels[i] = m.Time.Format("15:04:05")
		values[i] = m.Used
		percents[i] = m.Percent
	}

	return labels, values, percents
}

// GetCPUData returns CPU data for charting
func (s *Stats) GetCPUData() ([]string, []float64) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	labels := make([]string, len(s.cpu))
	percents := make([]float64, len(s.cpu))

	for i, c := range s.cpu {
		labels[i] = c.Time.Format("15:04:05")
		percents[i] = c.Percent
	}

	return labels, percents
}
