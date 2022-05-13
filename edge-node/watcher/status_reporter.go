package watcher

import (
	"context"
	"ecos/edge-node/infos"
	"github.com/rcrowley/go-metrics"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"runtime"
)

type StatusReporter struct {
	watcher *Watcher
	ctx     context.Context
}

func (s *StatusReporter) IsChanged() bool {
	return true
}

func (s *StatusReporter) GetReports() []Report {
	var status NodeStatus
	diskState, _ := disk.UsageWithContext(s.ctx, "/")
	status.DiskTotal = diskState.Total
	status.DiskAvailable = diskState.Free
	memState, _ := mem.VirtualMemoryWithContext(s.ctx)
	status.MemoryTotal = memState.Total
	status.MemoryUsage = memState.Used

	cpuState, _ := cpu.PercentWithContext(s.ctx, 0, false)
	status.CpuPercent = cpuState[0]
	status.GoroutineCount = uint64(runtime.NumGoroutine())

	// Get ecos metrics
	status.MetaPipelineCount = uint64(metrics.GetOrRegisterCounter(MetricsAlayaPipelineCount, nil).Count())
	status.MetaCount = uint64(metrics.GetOrRegisterCounter(MetricsAlayaMetaCount, nil).Count())
	status.BlockCount = uint64(metrics.GetOrRegisterCounter(MetricsGaiaBlockCount, nil).Count())

	return []Report{
		{
			ReportType: ReportTypeUPDATE,
			NodeReport: &NodeStatusReport{
				NodeId:    s.watcher.GetSelfInfo().RaftId,
				NodeUuid:  s.watcher.GetSelfInfo().Uuid,
				Timestamp: nil,
				Status:    &status,
				State:     infos.NodeState_ONLINE,
				Role:      0,
			},
			PipelineReport: nil,
		},
	}
}

func NewStatusReporter(ctx context.Context, watcher *Watcher) *StatusReporter {
	reporter := &StatusReporter{
		watcher: watcher,
		ctx:     ctx,
	}
	_ = watcher.Monitor.Register("status_reporter", reporter)
	return reporter
}
