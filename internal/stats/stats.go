package stats

import (
	"context"
	"log"
	"net"
	"sort"
	"strconv"
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

type sshSessionState struct {
	RemoteIP    string
	RemotePort  uint32
	FirstSeen   time.Time
	LastSeen    time.Time
	CountryCode string
	CountryName string
	DeviceName  string
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
	sshSessions     map[string]sshSessionState
	deviceNames     map[string]string
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
		sshSessions:     make(map[string]sshSessionState),
		deviceNames:     make(map[string]string),
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
	v, _ := mem.VirtualMemory()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.memory = append(s.memory, Memory{
		Time:    time.Now(),
		Used:    v.Used / 1024 / 1024,
		Percent: v.UsedPercent,
	})

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

	s.cpu = append(s.cpu, CPU{Time: time.Now(), Percent: percent[0]})
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
		entries = append(entries, DiskUsage{Time: now, Mountpoint: partition.Mountpoint, Device: partition.Device, Fstype: partition.Fstype, UsedPercent: usage.UsedPercent, Free: usage.Free, Total: usage.Total})
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
	now := time.Now()
	if err != nil {
		log.Printf("[WARN] Unable to collect SSH connections: %v", err)
		s.appendSSHSample(0, remoteCounts)
		s.pruneSSHSessions(map[string]sshSessionState{})
		return
	}

	detected := make(map[string]sshSessionState)
	for _, conn := range conns {
		if conn.Laddr.Port != 22 || strings.ToUpper(conn.Status) != "ESTABLISHED" {
			continue
		}
		remoteIP := normalizeRemoteIP(conn.Raddr.IP)
		if remoteIP == "" {
			continue
		}
		remoteCounts[remoteIP]++
		key := sshSessionKey(remoteIP, conn.Raddr.Port)
		detected[key] = sshSessionState{RemoteIP: remoteIP, RemotePort: conn.Raddr.Port, LastSeen: now}
	}

	s.mergeSSHSessions(now, detected)
	s.appendSSHSample(len(detected), remoteCounts)
}

func sshSessionKey(remoteIP string, remotePort uint32) string {
	return remoteIP + ":" + strconv.FormatUint(uint64(remotePort), 10)
}

func (s *Stats) mergeSSHSessions(now time.Time, detected map[string]sshSessionState) {
	s.mu.RLock()
	existingSessions := make(map[string]sshSessionState, len(s.sshSessions))
	for key, session := range s.sshSessions {
		existingSessions[key] = session
	}
	deviceCache := make(map[string]string, len(s.deviceNames))
	for ip, name := range s.deviceNames {
		deviceCache[ip] = name
	}
	s.mu.RUnlock()

	for key, session := range detected {
		if old, ok := existingSessions[key]; ok {
			session.FirstSeen = old.FirstSeen
			session.CountryCode = old.CountryCode
			session.CountryName = old.CountryName
			session.DeviceName = old.DeviceName
		} else {
			session.FirstSeen = now
			countryCode := CountryFromIP(session.RemoteIP)
			session.CountryCode = countryCode
			session.CountryName = countryName(countryCode)
			if cached, ok := deviceCache[session.RemoteIP]; ok {
				session.DeviceName = cached
			} else {
				resolved := resolveDeviceName(session.RemoteIP)
				deviceCache[session.RemoteIP] = resolved
				session.DeviceName = resolved
			}
		}
		session.LastSeen = now
		detected[key] = session
	}

	s.pruneSSHSessions(detected)

	s.mu.Lock()
	defer s.mu.Unlock()
	for ip, name := range deviceCache {
		s.deviceNames[ip] = name
	}
}

func (s *Stats) pruneSSHSessions(active map[string]sshSessionState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sshSessions = active
}

func resolveDeviceName(ip string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	names, err := net.DefaultResolver.LookupAddr(ctx, ip)
	if err != nil || len(names) == 0 {
		return "Unknown device"
	}
	name := strings.TrimSuffix(strings.TrimSpace(names[0]), ".")
	if name == "" {
		return "Unknown device"
	}
	return name
}

func (s *Stats) appendSSHSample(established int, remoteCounts map[string]int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ssh = append(s.ssh, SSHConnections{Time: time.Now(), Established: established, ByRemoteIP: remoteCounts})
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

// GetSSHData returns only currently active SSH sessions and connection metadata.
func (s *Stats) GetSSHData() map[string]interface{} {
	s.mu.RLock()
	sessions := make([]sshSessionState, 0, len(s.sshSessions))
	for _, session := range s.sshSessions {
		sessions = append(sessions, session)
	}
	s.mu.RUnlock()

	sort.Slice(sessions, func(i, j int) bool {
		if sessions[i].FirstSeen.Equal(sessions[j].FirstSeen) {
			if sessions[i].RemoteIP == sessions[j].RemoteIP {
				return sessions[i].RemotePort < sessions[j].RemotePort
			}
			return sessions[i].RemoteIP < sessions[j].RemoteIP
		}
		return sessions[i].FirstSeen.Before(sessions[j].FirstSeen)
	})

	rows := make([]map[string]interface{}, 0, len(sessions))
	for _, session := range sessions {
		rows = append(rows, map[string]interface{}{
			"ip":          session.RemoteIP,
			"port":        session.RemotePort,
			"countryCode": session.CountryCode,
			"countryName": session.CountryName,
			"device":      session.DeviceName,
			"connectedAt": session.FirstSeen.Format("2006-01-02 15:04:05"),
			"date":        session.FirstSeen.Format("2006-01-02"),
			"time":        session.FirstSeen.Format("15:04:05"),
		})
	}

	return map[string]interface{}{
		"current":  len(rows),
		"sessions": rows,
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

	hourlyData := make(map[string]map[string]int)
	hostsMap := make(map[string]bool)

	for _, req := range s.requests {
		hour := req.Time.Format("2006-01-02 15:00")
		if _, ok := hourlyData[hour]; !ok {
			hourlyData[hour] = make(map[string]int)
		}
		hourlyData[hour][req.Host]++
		hostsMap[req.Host] = true
	}

	labels := make([]string, 0, len(hourlyData))
	for hour := range hourlyData {
		labels = append(labels, hour)
	}
	sort.Strings(labels)

	hosts := make([]string, 0, len(hostsMap))
	for host := range hostsMap {
		hosts = append(hosts, host)
	}
	sort.Strings(hosts)

	datasets := []map[string]interface{}{}
	for _, host := range hosts {
		data := make([]int, len(labels))
		for i, hour := range labels {
			if count, ok := hourlyData[hour][host]; ok {
				data[i] = count
			}
		}
		datasets = append(datasets, map[string]interface{}{
			"label": host,
			"data":  data,
		})
	}

	return map[string]interface{}{
		"labels":   labels,
		"datasets": datasets,
	}
}

// GetMemoryData returns labels, values, and percentages for memory usage
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

// GetCPUData returns labels and percentages for CPU usage
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
