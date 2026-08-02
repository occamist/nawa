package hoststats

import (
	"context"
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
)

type Stats struct {
	CPUPercent float64 `json:"cpu_percent"`
	MemUsed    uint64  `json:"mem_used"`
	MemTotal   uint64  `json:"mem_total"`
	DiskUsed   uint64  `json:"disk_used"`
	DiskTotal  uint64  `json:"disk_total"`
}

// Collect blocks for interval while sampling CPU usage, then reports current memory and disk usage for diskPath.
func Collect(ctx context.Context, diskPath string, interval time.Duration) (Stats, error) {
	percents, err := cpu.PercentWithContext(ctx, interval, false)
	if err != nil {
		return Stats{}, fmt.Errorf("failed to compute cpu percent: %w", err)
	}
	var cpuPercent float64
	if len(percents) > 0 {
		cpuPercent = percents[0]
	}

	vm, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return Stats{}, fmt.Errorf("failed to compute virtual memory: %w", err)
	}

	du, err := disk.UsageWithContext(ctx, diskPath)
	if err != nil {
		return Stats{}, fmt.Errorf("failed to compute disk usage: %w", err)
	}

	return Stats{
		CPUPercent: cpuPercent,
		MemUsed:    vm.Used,
		MemTotal:   vm.Total,
		DiskUsed:   du.Used,
		DiskTotal:  du.Total,
	}, nil
}
