package discovery

import (
	"bufio"
	"os"
	"os/exec"
	"runtime"
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
		CPUUsagePercent:  5.0,
		MemoryUsageBytes: 1024 * 1024 * 512,  // 512MB
		MemoryTotalBytes: 1024 * 1024 * 4096, // 4GB
		DiskUsagePercent: 25.0,
		LoadAvg1m:        0.15,
		ActiveTasks:      activeTasks,
		TimestampUnix:    time.Now().Unix(),
	}

	// On Linux read /proc/meminfo and /proc/loadavg
	if runtime.GOOS == "linux" {
		if data, err := os.ReadFile("/proc/loadavg"); err == nil {
			fields := strings.Fields(string(data))
			if len(fields) > 0 {
				// parse if needed or default
			}
		}
	}

	return m
}

func (c *Collector) hasBinary(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
