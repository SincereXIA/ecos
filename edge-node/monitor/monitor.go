package monitor

import (
	"context"
	"ecos/edge-node/infos"
	"ecos/edge-node/watcher"
	"ecos/messenger"
	"ecos/messenger/common"
	"ecos/utils/logger"
	"errors"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"runtime"
	"strconv"
	"sync"
	"time"
)

type Monitor interface {
	MonitorServer
	Run()
}

type NodeMonitor struct {
	UnimplementedMonitorServer
	ctx    context.Context
	cancel context.CancelFunc
	timer  *time.Ticker

	nodeStateMap sync.Map
	selfStatus   *NodeStatus
	watcher      *watcher.Watcher
}

func (m *NodeMonitor) Report(_ context.Context, report *NodeStateReport) (*common.Result, error) {
	if !m.watcher.GetMoon().IsLeader() {
		// only leader can be runReport
		return nil, errors.New("not leader")
	}
	m.nodeStateMap.Store(report.NodeId, report)
	return &common.Result{}, nil
}

func (m *NodeMonitor) genSelfState() *NodeStatus {
	var status NodeStatus
	diskState, _ := disk.UsageWithContext(m.ctx, "/")
	status.DiskTotal = diskState.Total
	status.DiskAvailable = diskState.Free
	memState, _ := mem.VirtualMemoryWithContext(m.ctx)
	status.MemoryTotal = memState.Total
	status.MemoryUsage = memState.Used

	cpuState, _ := cpu.PercentWithContext(m.ctx, 0, false)
	status.CpuPercent = cpuState[0]
	status.GoroutineCount = uint64(runtime.NumGoroutine())
	return &status
}

func (m *NodeMonitor) getSelfStateChan() <-chan *NodeStatus {
	stateChan := make(chan *NodeStatus)
	go func() {
		for {
			select {
			case <-m.ctx.Done():
				close(stateChan)
				return
			default:
				<-m.timer.C
				stateChan <- m.genSelfState()
			}
		}
	}()
	return stateChan
}

func (m *NodeMonitor) runReport(nodeStatusChan <-chan *NodeStatus) {
	for {
		select {
		case <-m.ctx.Done():
			return
		case status := <-nodeStatusChan:
			m.selfStatus = status
			leaderID := m.watcher.GetMoon().GetLeaderID()
			if leaderID == 0 {
				continue
			}
			leaderInfo, err := m.watcher.GetMoon().GetInfoDirect(infos.InfoType_NODE_INFO,
				strconv.FormatUint(leaderID, 10))
			if err != nil {
				logger.Errorf("get leader info failed: %v", err)
				continue
			}
			conn, _ := messenger.GetRpcConnByNodeInfo(leaderInfo.BaseInfo().GetNodeInfo())
			client := NewMonitorClient(conn)
			_, err = client.Report(m.ctx, &NodeStateReport{
				NodeId:    m.watcher.GetSelfInfo().RaftId,
				NodeUuid:  m.watcher.GetSelfInfo().Uuid,
				Timestamp: nil,
				Status:    status,
			})
			if err != nil {
				logger.Errorf("runReport node status failed: %v", err)
			}
		}
	}
}

func (m *NodeMonitor) Run() {
	m.timer = time.NewTicker(time.Second * 1)
	m.runReport(m.getSelfStateChan())
}

func NewMonitor(ctx context.Context, w *watcher.Watcher, rpcServer *messenger.RpcServer) Monitor {
	ctx, cancel := context.WithCancel(ctx)
	monitor := &NodeMonitor{
		ctx:          ctx,
		cancel:       cancel,
		nodeStateMap: sync.Map{},
		selfStatus:   &NodeStatus{},
		watcher:      w,
	}
	RegisterMonitorServer(rpcServer, monitor)
	return monitor
}
