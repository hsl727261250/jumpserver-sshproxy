package jmsauth

import (
	"math"
	"os"
	"strconv"
	"strings"
	"syscall"
)

type hostStat struct {
	CPULoad    float64
	MemoryUsed float64
	DiskUsed   float64
}

func readHostStat() hostStat {
	var stat hostStat
	if content, err := os.ReadFile("/proc/loadavg"); err == nil {
		if load, err := ParseLoadavg(string(content)); err == nil {
			stat.CPULoad = load
		}
	}
	if content, err := os.ReadFile("/proc/meminfo"); err == nil {
		if used, err := ParseMeminfo(string(content)); err == nil {
			stat.MemoryUsed = used
		}
	}
	if used, err := DiskUsedPercent("/"); err == nil {
		stat.DiskUsed = used
	}
	return stat
}

func ParseLoadavg(text string) (float64, error) {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return 0, strconv.ErrSyntax
	}
	value, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, err
	}
	return round1(value), nil
}

func ParseMeminfo(text string) (float64, error) {
	var total, available uint64
	var hasTotal, hasAvailable, hasFree bool
	var free uint64
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		switch strings.TrimSuffix(fields[0], ":") {
		case "MemTotal":
			total, hasTotal = value, true
		case "MemAvailable":
			available, hasAvailable = value, true
		case "MemFree":
			free, hasFree = value, true
		}
	}
	if !hasTotal || total == 0 {
		return 0, strconv.ErrSyntax
	}
	if !hasAvailable {
		if !hasFree {
			return 0, strconv.ErrSyntax
		}
		available = free
	}
	if available > total {
		available = total
	}
	return round1(float64(total-available) / float64(total) * 100), nil
}

func DiskUsedPercent(path string) (float64, error) {
	var fs syscall.Statfs_t
	if err := syscall.Statfs(path, &fs); err != nil {
		return 0, err
	}
	if fs.Blocks == 0 || fs.Bfree > fs.Blocks {
		return 0, nil
	}
	used := fs.Blocks - fs.Bfree
	return round1(float64(used) / float64(fs.Blocks) * 100), nil
}

func round1(value float64) float64 {
	return math.Round(value*10) / 10
}
