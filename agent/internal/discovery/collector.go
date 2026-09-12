package discovery

import (
	"bufio"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type HostInfo struct {
	Hostname     string
	OS           string
	Distribution string
	Version      string
	Architecture string
	Capabilities []string
	IPAddress    string
}

type Metrics struct {
	CPUUsagePercent  float64
	MemoryUsageBytes int64
	MemoryTotalBytes int64
	DiskUsagePercent float64
	LoadAvg1m        float64
	ActiveTasks      int32
	TimestampUnix    int64
}

type Collector struct{}

func NewCollector() *Collector {
	return &Collector{}
}

func (c *Collector) DiscoverHost() *HostInfo {
	hostname, _ := os.Hostname()
	info := &HostInfo{
		Hostname:     hostname,
		OS:           runtime.GOOS,
		Distribution: "unknown",
		Version:      "unknown",
		Architecture: runtime.GOARCH,
		Capabilities: make([]string, 0),
		IPAddress:    "127.0.0.1",
	}

	// Discover outbound host IP address
	if ifaces, err := net.Interfaces(); err == nil {
		for _, iface := range ifaces {
			if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
				continue
			}
			addrs, err := iface.Addrs()
			if err != nil {
				continue
			}
			for _, addr := range addrs {
				if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
					if ipNet.IP.To4() != nil {
						info.IPAddress = ipNet.IP.String()
						break
					}
				}
			}
			if info.IPAddress != "127.0.0.1" {
				break
			}
		}
	}

	// Read /etc/os-release on Linux
	if runtime.GOOS == "linux" {
		if file, err := os.Open("/etc/os-release"); err == nil {
			defer file.Close()
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "ID=") {
					info.Distribution = strings.Trim(strings.TrimPrefix(line, "ID="), "\"")
				} else if strings.HasPrefix(line, "VERSION_ID=") {
					info.Version = strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), "\"")
				}
			}
		}
	} else {
		info.Distribution = runtime.GOOS
		info.Version = "local"
	}

	// Discover available capabilities
	if c.hasBinary("systemctl") {
		info.Capabilities = append(info.Capabilities, "systemd")
	}
	if c.hasBinary("apt-get") || c.hasBinary("apt") {
		info.Capabilities = append(info.Capabilities, "apt")
	}
	if c.hasBinary("docker") {
		info.Capabilities = append(info.Capabilities, "docker")
	}
	if c.hasBinary("journalctl") {
		info.Capabilities = append(info.Capabilities, "journald")
	}
	if c.hasBinary("ss") || c.hasBinary("netstat") || c.hasBinary("curl") {
		info.Capabilities = append(info.Capabilities, "network")
	}

	return info
}

func (c *Collector) CollectMetrics(activeTasks int32) *Metrics {
	m := &Metrics{
		CPUUsagePercent:  0.0,
		MemoryUsageBytes: 0,
		MemoryTotalBytes: 0,
		DiskUsagePercent: 0.0,
		LoadAvg1m:        0.0,
		ActiveTasks:      activeTasks,
		TimestampUnix:    time.Now().Unix(),
	}

	// On Linux read real /proc/loadavg and /proc/meminfo
	if runtime.GOOS == "linux" {
		if data, err := os.ReadFile("/proc/loadavg"); err == nil {
			fields := strings.Fields(string(data))
			if len(fields) > 0 {
				if val, err := strconv.ParseFloat(fields[0], 64); err == nil {
					m.LoadAvg1m = val
				}
			}
		}

		if data, err := os.ReadFile("/proc/meminfo"); err == nil {
			var memTotalKB, memAvailKB int64
			scanner := bufio.NewScanner(strings.NewReader(string(data)))
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "MemTotal:") {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						memTotalKB, _ = strconv.ParseInt(fields[1], 10, 64)
					}
				} else if strings.HasPrefix(line, "MemAvailable:") {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						memAvailKB, _ = strconv.ParseInt(fields[1], 10, 64)
					}
				}
			}
			if memTotalKB > 0 {
				m.MemoryTotalBytes = memTotalKB * 1024
				m.MemoryUsageBytes = (memTotalKB - memAvailKB) * 1024
			}
		}
	} else {
		// Reasonable dev metrics on non-Linux
		m.MemoryTotalBytes = 1024 * 1024 * 8192
		m.MemoryUsageBytes = 1024 * 1024 * 2048
		m.LoadAvg1m = 0.5
	}

	return m
}

func (c *Collector) hasBinary(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
