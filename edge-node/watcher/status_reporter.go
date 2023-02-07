package watcher

import (
	"context"
	"ecos/edge-node/infos"
	"ecos/messenger"
	"github.com/rcrowley/go-metrics"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"runtime"
	"time"
)

type StatusReporter struct {
	watcher *Watcher
	ctx     context.Context

	lastReportTime   time.Time
	lastNetBytesSent uint64
	lastNetBytesRecv uint64
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

	// network speed
	netState, _ := net.IOCountersWithContext(s.ctx, false)
	deltaBytesSent := netState[0].BytesSent - s.lastNetBytesSent
	deltaBytesRecv := netState[0].BytesRecv - s.lastNetBytesRecv
	sentPerSec := float64(deltaBytesSent) / time.Since(s.lastReportTime).Seconds()
	recvPerSec := float64(deltaBytesRecv) / time.Since(s.lastReportTime).Seconds()
	metrics.GetOrRegisterMeter(messenger.MetricsSystemNetworkBytesSendPerSecond, nil).Mark(int64(sentPerSec))
	metrics.GetOrRegisterMeter(messenger.MetricsSystemNetworkBytesReceivedPerSecond, nil).Mark(int64(recvPerSec))
	s.lastReportTime = time.Now()
	s.lastNetBytesSent = netState[0].BytesSent
	s.lastNetBytesRecv = netState[0].BytesRecv
	status.NetworkSpeedSend = uint64(sentPerSec)
	status.NetworkSpeedRecv = uint64(recvPerSec)

	cpuState, _ := cpu.PercentWithContext(s.ctx, 0, false)
	status.CpuPercent = cpuState[0]
	status.GoroutineCount = uint64(runtime.NumGoroutine())

	// Get ecos metrics
	status.MetaPipelineCount = uint64(metrics.GetOrRegisterCounter(messenger.MetricsAlayaPipelineCount, nil).Count())
	status.MetaCount = uint64(metrics.GetOrRegisterCounter(messenger.MetricsAlayaMetaCount, nil).Count())
	status.BlockCount = uint64(metrics.GetOrRegisterCounter(messenger.MetricsGaiaBlockCount, nil).Count())

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
