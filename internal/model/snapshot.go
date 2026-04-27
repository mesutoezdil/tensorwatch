package model

import "time"

type Snapshot struct {
	Taken    time.Time `json:"taken"`
	Host     Host      `json:"host"`
	CPU      CPU       `json:"cpu"`
	Memory   Memory    `json:"memory"`
	GPUs     []GPU     `json:"gpus"`
	Warnings []string  `json:"warnings,omitempty"`
}

type Host struct {
	Hostname     string  `json:"hostname"`
	OS           string  `json:"os"`
	Kernel       string  `json:"kernel"`
	Arch         string  `json:"arch"`
	UptimeSec    uint64  `json:"uptime_sec"`
	Load1        float64 `json:"load1"`
	Load5        float64 `json:"load5"`
	Load15       float64 `json:"load15"`
	BootTimeUnix int64   `json:"boot_time_unix"`
}

type CPU struct {
	LogicalCores  int       `json:"logical_cores"`
	PhysicalCores int       `json:"physical_cores"`
	ModelName     string    `json:"model"`
	UsageOverall  float64   `json:"usage_overall_pct"`
	UsagePerCore  []float64 `json:"usage_per_core_pct"`
	FreqMHz       float64   `json:"freq_mhz"`
	TempCelsius   float64   `json:"temp_c,omitempty"`
}

type Memory struct {
	TotalBytes     uint64  `json:"total_bytes"`
	AvailableBytes uint64  `json:"available_bytes"`
	UsedBytes      uint64  `json:"used_bytes"`
	UsedPct        float64 `json:"used_pct"`
	BufCacheBytes  uint64  `json:"buf_cache_bytes"`
	SwapTotal      uint64  `json:"swap_total_bytes"`
	SwapUsed       uint64  `json:"swap_used_bytes"`
}

type GPU struct {
	Index       int     `json:"index"`
	Vendor      string  `json:"vendor"`
	Name        string  `json:"name"`
	UUID        string  `json:"uuid,omitempty"`
	UtilGPU     float64 `json:"util_gpu_pct"`
	UtilMemory  float64 `json:"util_mem_pct"`
	MemTotal    uint64  `json:"mem_total_bytes"`
	MemUsed     uint64  `json:"mem_used_bytes"`
	TempCelsius float64 `json:"temp_c"`
	PowerWatts  float64 `json:"power_w"`
	PowerLimitW float64 `json:"power_limit_w,omitempty"`
	ClockCore   uint32  `json:"clock_core_mhz,omitempty"`
	ClockMem    uint32  `json:"clock_mem_mhz,omitempty"`
	FanPercent  float64 `json:"fan_pct,omitempty"`
	EncoderPct  float64 `json:"encoder_pct,omitempty"`
	DecoderPct  float64 `json:"decoder_pct,omitempty"`
	Processes   []GPUProcess `json:"processes,omitempty"`
}

type GPUProcess struct {
	PID         int32  `json:"pid"`
	Name        string `json:"name"`
	MemoryBytes uint64 `json:"memory_bytes"`
}
