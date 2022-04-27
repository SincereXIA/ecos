package watcher

import (
	"context"
	"ecos/edge-node/infos"
	"ecos/messenger"
	"ecos/messenger/common"
	"ecos/utils/logger"
	"errors"
	"github.com/rcrowley/go-metrics"
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
	GetAllReports() []*NodeStatusReport
	GetReport(nodeId uint64) *NodeStatusReport
	GetEventChannel() <-chan *Event
	Stop()
}

type Event struct {
	Report *NodeStatusReport
}

type NodeMonitor struct {
	UnimplementedMonitorServer
	ctx    context.Context
	cancel context.CancelFunc
	timer  *time.Ticker

	nodeStatusMap sync.Map
	reportTimers  sync.Map
	selfStatus    *NodeStatus
	watcher       *Watcher

	eventChannel chan *Event
}

// Report is a rpc func to get the node status report.
// it called by all node (include leader self).
func (m *NodeMonitor) Report(_ context.Context, report *NodeStatusReport) (*common.Result, error) {
	if !m.watcher.GetMoon().IsLeader() {
		// only leader can be runReport
		logger.Infof("node %v is not leader, can't runReport", m.watcher.GetSelfInfo().GetID())
		return nil, errors.New("not leader")
	}
	if val, ok := m.reportTimers.Load(report.NodeId); ok {
		logger.Tracef("reset timer for node %v", report.NodeId)
		t := val.(*time.Timer)
		if !t.Stop() {
			<-t.C
		}
		t.Reset(time.Second * 3)
	} else {
		logger.Debugf("create timer for node %v", report.NodeId)
		m.reportTimers.Store(report.NodeId, time.AfterFunc(time.Second*3, func() {
			logger.Warningf("get node status timeout, nodeId: %v", report.NodeId)
			v, _ := m.nodeStatusMap.Load(report.NodeId)
			r := v.(*NodeStatusReport)
			r.State = infos.NodeState_OFFLINE
			m.nodeStatusMap.Store(report.NodeId, r)
			m.eventChannel <- &Event{
				Report: r,
			}
		}))
	}
	if _, ok := m.nodeStatusMap.Load(report.NodeId); !ok {
		// first time online
		m.eventChannel <- &Event{
			Report: report,
		}
	}
	m.nodeStatusMap.Store(report.NodeId, report)
	return &common.Result{}, nil
}

// GetEventChannel returns the event channel.
// Event channel will send event when the node status changed. (like node online, offline, etc.)
func (m *NodeMonitor) GetEventChannel() <-chan *Event {
	return m.eventChannel
}

// GetAllReports returns all node status reports.
func (m *NodeMonitor) GetAllReports() []*NodeStatusReport {
	var nodeStatusList []*NodeStatusReport
	m.nodeStatusMap.Range(func(key, value interface{}) bool {
		nodeStatusList = append(nodeStatusList, value.(*NodeStatusReport))
		return true
	})
	return nodeStatusList
}

func (m *NodeMonitor) GetReport(nodeID uint64) *NodeStatusReport {
	if val, ok := m.nodeStatusMap.Load(nodeID); ok {
		return val.(*NodeStatusReport)
	}
	return nil
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

	// Get ecos metrics
	status.MetaPipelineCount = uint64(metrics.GetOrRegisterCounter(MetricsAlayaPipelineCount, nil).Count())
	status.MetaCount = uint64(metrics.GetOrRegisterCounter(MetricsAlayaMetaCount, nil).Count())
	status.BlockCount = uint64(metrics.GetOrRegisterCounter(MetricsGaiaBlockCount, nil).Count())

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
			leaderInfo, err := m.watcher.GetMoon().GetInfoDirect(infos.InfoType_NODE_INFO, strconv.FormatUint(leaderID, 10))
			if err != nil || leaderInfo.BaseInfo().GetNodeInfo() == nil {
				logger.Warningf("node: %v get leader info: %v failed: %v", m.watcher.GetSelfInfo().GetID(), leaderID, err)
				continue
			}
			conn, _ := messenger.GetRpcConnByNodeInfo(leaderInfo.BaseInfo().GetNodeInfo())
			client := NewMonitorClient(conn)
			_, err = client.Report(m.ctx, &NodeStatusReport{
				NodeId:    m.watcher.GetSelfInfo().RaftId,
				NodeUuid:  m.watcher.GetSelfInfo().Uuid,
				Timestamp: nil,
				Status:    status,
				State:     infos.NodeState_ONLINE,
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

func (m *NodeMonitor) Stop() {
	m.cancel()
	m.timer.Stop()
}

func NewMonitor(ctx context.Context, w *Watcher, rpcServer *messenger.RpcServer) Monitor {
	ctx, cancel := context.WithCancel(ctx)
	monitor := &NodeMonitor{
		ctx:           ctx,
		cancel:        cancel,
		nodeStatusMap: sync.Map{},
		selfStatus:    &NodeStatus{},
		watcher:       w,
		eventChannel:  make(chan *Event),
	}
	RegisterMonitorServer(rpcServer, monitor)
	return monitor
}
