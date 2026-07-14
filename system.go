package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const commandTimeout = 4 * time.Second

type CPUStats struct {
	Cores        int     `json:"cores"`
	UsagePercent float64 `json:"usage_percent"`
}

type MemoryStats struct {
	TotalMB     int64   `json:"total_mb"`
	AvailableMB int64   `json:"available_mb"`
	UsedPercent float64 `json:"used_percent"`
	SwapTotalMB int64   `json:"swap_total_mb"`
	SwapUsedMB  int64   `json:"swap_used_mb"`
}

type BatteryStats struct {
	Percentage         int     `json:"percentage"`
	Status             string  `json:"status"`
	Health             string  `json:"health"`
	TemperatureCelsius float64 `json:"temperature_celsius"`
}

type StorageStats struct {
	TotalGB     float64 `json:"total_gb"`
	AvailableGB float64 `json:"available_gb"`
	UsedPercent float64 `json:"used_percent"`
}

type MetricFreshness struct {
	CPUMemory      *time.Time `json:"cpu_memory,omitempty"`
	BatteryStorage *time.Time `json:"battery_storage,omitempty"`
}

type SystemResponse struct {
	Status    string          `json:"status"`
	Timestamp time.Time       `json:"timestamp"`
	Uptime    string          `json:"uptime"`
	CPU       CPUStats        `json:"cpu"`
	Memory    MemoryStats     `json:"memory"`
	Battery   BatteryStats    `json:"battery"`
	Storage   StorageStats    `json:"storage"`
	Freshness MetricFreshness `json:"freshness"`
}

type systemCollector struct {
	mu sync.RWMutex

	uptime  string
	cpu     CPUStats
	memory  MemoryStats
	battery BatteryStats
	storage StorageStats

	fastUpdated *time.Time
	slowUpdated *time.Time
	fastHealthy bool
	slowHealthy bool

	run func(string, ...string) ([]byte, error)
}

func newSystemCollector() *systemCollector {
	return &systemCollector{run: runCommand}
}

func (c *systemCollector) start() {
	c.collectFast()
	c.collectSlow()

	go c.collectEvery(5*time.Second, c.collectFast)
	go c.collectEvery(30*time.Second, c.collectSlow)
}

func (c *systemCollector) collectEvery(interval time.Duration, collect func()) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		collect()
	}
}

func (c *systemCollector) collectFast() {
	cores := runtime.NumCPU()
	memOutput, memErr := c.run("cat", "/proc/meminfo")
	// Android blocks Termux from reading the system-wide /proc/stat counters. Two
	// samples let top calculate CPU use for the processes visible to Termux.
	topOutput, cpuErr := c.run("top", "-b", "-d", "1", "-n", "2")

	memory, memParseErr := parseMemory(memOutput)
	cpuUsage, cpuParseErr := parseCPU(topOutput, cores)
	healthy := memErr == nil && cpuErr == nil && memParseErr == nil && cpuParseErr == nil

	c.mu.Lock()
	defer c.mu.Unlock()
	c.fastHealthy = healthy
	if memErr == nil && memParseErr == nil {
		c.memory = memory
	}
	if cpuErr == nil && cpuParseErr == nil {
		c.cpu = CPUStats{Cores: cores, UsagePercent: cpuUsage}
	}
	if healthy {
		now := time.Now().UTC()
		c.fastUpdated = &now
	} else {
		log.Printf("system metrics fast collection degraded: memory=%v/%v cpu=%v/%v", memErr, memParseErr, cpuErr, cpuParseErr)
	}
}

func (c *systemCollector) collectSlow() {
	batteryOutput, batteryErr := c.run("termux-battery-status")
	home, homeErr := os.UserHomeDir()
	storageOutput, storageErr := c.run("df", "-k", home)
	uptimeOutput, uptimeErr := c.run("uptime", "-p")

	battery, batteryParseErr := parseBattery(batteryOutput)
	storage, storageParseErr := parseStorage(storageOutput)
	uptime, uptimeParseErr := parseUptime(uptimeOutput)
	healthy := batteryErr == nil && storageErr == nil && uptimeErr == nil && homeErr == nil &&
		batteryParseErr == nil && storageParseErr == nil && uptimeParseErr == nil

	c.mu.Lock()
	defer c.mu.Unlock()
	c.slowHealthy = healthy
	if batteryErr == nil && batteryParseErr == nil {
		c.battery = battery
	}
	if homeErr == nil && storageErr == nil && storageParseErr == nil {
		c.storage = storage
	}
	if uptimeErr == nil && uptimeParseErr == nil {
		c.uptime = uptime
	}
	if healthy {
		now := time.Now().UTC()
		c.slowUpdated = &now
	} else {
		log.Printf("system metrics slow collection degraded: battery=%v/%v storage=%v/%v uptime=%v/%v home=%v", batteryErr, batteryParseErr, storageErr, storageParseErr, uptimeErr, uptimeParseErr, homeErr)
	}
}

func (c *systemCollector) response() SystemResponse {
	c.mu.RLock()
	defer c.mu.RUnlock()

	status := "degraded"
	if c.fastHealthy && c.slowHealthy {
		status = "ok"
	}
	return SystemResponse{
		Status: status, Timestamp: time.Now().UTC(), Uptime: c.uptime,
		CPU: c.cpu, Memory: c.memory, Battery: c.battery, Storage: c.storage,
		Freshness: MetricFreshness{CPUMemory: cloneTime(c.fastUpdated), BatteryStorage: cloneTime(c.slowUpdated)},
	}
}

func (c *systemCollector) systemHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(c.response()); err != nil {
		log.Printf("failed to encode system response: %v", err)
	}
}

func runCommand(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	output, err := exec.CommandContext(ctx, name, args...).Output()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return nil, fmt.Errorf("%s timed out", name)
	}
	return output, err
}

func parseMemory(output []byte) (MemoryStats, error) {
	values := make(map[string]int64)
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		value, err := strconv.ParseInt(fields[1], 10, 64)
		if err == nil {
			values[key] = value
		}
	}
	if err := scanner.Err(); err != nil {
		return MemoryStats{}, err
	}
	total, totalOK := values["MemTotal"]
	available, availableOK := values["MemAvailable"]
	if !totalOK || !availableOK || total <= 0 {
		return MemoryStats{}, errors.New("missing required memory values")
	}
	swapTotal := values["SwapTotal"]
	swapFree := values["SwapFree"]
	return MemoryStats{
		TotalMB: total / 1024, AvailableMB: available / 1024,
		UsedPercent: round1(float64(total-available) * 100 / float64(total)),
		SwapTotalMB: swapTotal / 1024, SwapUsedMB: max(0, swapTotal-swapFree) / 1024,
	}, nil
}

func parseCPU(output []byte, cores int) (float64, error) {
	if cores <= 0 {
		return 0, errors.New("invalid CPU count")
	}
	lines := strings.Split(string(output), "\n")
	lastHeader := -1
	for index, line := range lines {
		if strings.Contains(line, "[%CPU]") {
			lastHeader = index
		}
	}
	if lastHeader < 0 {
		return 0, errors.New("CPU process table not found")
	}

	var totalProcessCPU float64
	for _, line := range lines[lastHeader+1:] {
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}
		processCPU, err := strconv.ParseFloat(fields[8], 64)
		if err == nil && processCPU >= 0 {
			totalProcessCPU += processCPU
		}
	}
	usage := totalProcessCPU / float64(cores)
	return round1(math.Max(0, math.Min(100, usage))), nil
}

func parseBattery(output []byte) (BatteryStats, error) {
	var raw struct {
		Percentage  int     `json:"percentage"`
		Status      string  `json:"status"`
		Health      string  `json:"health"`
		Temperature float64 `json:"temperature"`
	}
	if err := json.Unmarshal(output, &raw); err != nil {
		return BatteryStats{}, err
	}
	if raw.Percentage < 0 || raw.Percentage > 100 || raw.Status == "" {
		return BatteryStats{}, errors.New("invalid battery response")
	}
	return BatteryStats{Percentage: raw.Percentage, Status: raw.Status, Health: raw.Health, TemperatureCelsius: raw.Temperature}, nil
}

func parseStorage(output []byte) (StorageStats, error) {
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) < 2 {
		return StorageStats{}, errors.New("missing storage values")
	}
	fields := strings.Fields(lines[len(lines)-1])
	if len(fields) < 5 {
		return StorageStats{}, errors.New("invalid storage row")
	}
	totalKB, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil || totalKB <= 0 {
		return StorageStats{}, errors.New("invalid storage total")
	}
	availableKB, err := strconv.ParseInt(fields[3], 10, 64)
	if err != nil {
		return StorageStats{}, errors.New("invalid available storage")
	}
	usedPercent, err := strconv.ParseFloat(strings.TrimSuffix(fields[4], "%"), 64)
	if err != nil {
		return StorageStats{}, errors.New("invalid storage percentage")
	}
	const kbPerGB = 1024 * 1024
	return StorageStats{TotalGB: round1(float64(totalKB) / kbPerGB), AvailableGB: round1(float64(availableKB) / kbPerGB), UsedPercent: round1(usedPercent)}, nil
}

func parseUptime(output []byte) (string, error) {
	uptime := strings.TrimSpace(string(output))
	uptime = strings.TrimSpace(strings.TrimPrefix(uptime, "up"))
	if uptime == "" {
		return "", errors.New("empty uptime")
	}
	return uptime, nil
}

func round1(value float64) float64 { return math.Round(value*10) / 10 }

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
