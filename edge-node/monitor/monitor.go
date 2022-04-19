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
	GetAllReports() []*NodeStatusReport
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
	watcher       *watcher.Watcher

	eventChannel chan *Event
}

func (m *NodeMonitor) Report(_ context.Context, report *NodeStatusReport) (*common.Result, error) {
	if !m.watcher.GetMoon().IsLeader() {
		// only leader can be runReport
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
			if m.eventChannel != nil { // 当 channel 初始化后才发送事件
				m.eventChannel <- &Event{
					Report: r,
				}
			}
		}))
	}
	m.nodeStatusMap.Store(report.NodeId, report)
	return &common.Result{}, nil
}

func (m *NodeMonitor) GetEventChannel() <-chan *Event {
	if m.eventChannel != nil {
		logger.Errorf("event channel has been created")
		return nil
	}
	m.eventChannel = make(chan *Event)
	return m.eventChannel
}

func (m *NodeMonitor) GetAllReports() []*NodeStatusReport {
	var nodeStatusList []*NodeStatusReport
	m.nodeStatusMap.Range(func(key, value interface{}) bool {
		nodeStatusList = append(nodeStatusList, value.(*NodeStatusReport))
		return true
	})
	return nodeStatusList
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
				logger.Warningf("get leader info: %v failed: %v", leaderID, err)
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

func NewMonitor(ctx context.Context, w *watcher.Watcher, rpcServer *messenger.RpcServer) Monitor {
	ctx, cancel := context.WithCancel(ctx)
	monitor := &NodeMonitor{
		ctx:           ctx,
		cancel:        cancel,
		nodeStatusMap: sync.Map{},
		selfStatus:    &NodeStatus{},
		watcher:       w,
	}
	RegisterMonitorServer(rpcServer, monitor)
	return monitor
}
