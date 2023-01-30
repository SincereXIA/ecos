package benchmark

import (
	"bytes"
	"context"
	"ecos/client"
	"ecos/client/config"
	io2 "ecos/client/io"
	"ecos/edge-node/object"
	"ecos/utils/logger"
	prometheusmetrics "github.com/deathowl/go-metrics-prometheus"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
	"github.com/rcrowley/go-metrics"
	"github.com/shirou/gopsutil/v3/net"
	"io"
	"math/rand"
	"sync"
	"time"
)

var NetWorkTimeInterval = 5 * time.Second // seconds

type Generator struct {
}

func (g *Generator) Generate(size uint64) []byte {
	// Generate a random byte array of size `size`
	return genTestData(int(size))
}

func (g *Generator) FillRandom(data []byte) {
	directSize := 1024

	for i := 0; i < directSize && i < len(data); i++ {
		if i%100 == 0 {
			data[i] = '\n'
		} else {
			data[i] = byte(rand.Intn(26) + 97)
		}
	}

	for i := directSize; i < len(data); i += directSize {
		for j := 0; j < directSize && i+j < len(data); j++ {
			data[i+j] = data[i-directSize+j]
		}
	}
	return
}

func genTestData(size int) []byte {
	directSize := 1024
	if size < directSize {
		data := make([]byte, size)
		for idx := range data {
			if idx%100 == 0 && idx != 0 {
				data[idx] = '\n'
			} else {
				data[idx] = byte(rand.Intn(26) + 97)
			}
		}
		return data
	}
	d := make([]byte, directSize)
	data := make([]byte, 0, size)
	rand.Read(d)
	for size-directSize > 0 {
		data = append(data, d...)
		size = size - directSize
	}
	data = append(data, d[0:size]...)
	return data
}

type Connector interface {
	PutObject(objectName string, data []byte) error
	ListObjects() ([]string, error)
	GetObject(objectName string) (size uint64, err error)
	Clear() error
}

type MinioConnector struct {
	ctx context.Context
	c   *minio.Client
}

func NewMinioConnector(ctx context.Context) *MinioConnector {
	//endpoint := "minio.sums.top"
	//accessKeyID := "kubesphere"
	//secretAccessKey := "mKKwuN6Y!G9"
	endpoint := "192.168.7.141:45530"
	accessKeyID := "VLQ24N15T0PYZL9CHQD6"
	secretAccessKey := "DlG8CCPP7vKnskmHoY4aJBzUg5MWGUdaUIzFh7id"
	c, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: false,
	})
	if err != nil {
		logger.Errorf("create minio client failed: %v", err)
		return nil
	}
	c.MakeBucket(ctx, "test", minio.MakeBucketOptions{})
	return &MinioConnector{
		c:   c,
		ctx: ctx,
	}
}

func (c *MinioConnector) PutObject(objectName string, data []byte) error {
	_, err := c.c.PutObject(c.ctx, "test", objectName, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{})
	return err
}

func (c *MinioConnector) ListObjects() ([]string, error) {
	objects := c.c.ListObjects(c.ctx, "test", minio.ListObjectsOptions{})
	var objectNames []string
	for object := range objects {
		objectNames = append(objectNames, object.Key)
	}
	return objectNames, nil
}

func (c *MinioConnector) GetObject(objectName string) (size uint64, err error) {
	obj, err := c.c.GetObject(c.ctx, "test", objectName, minio.GetObjectOptions{})
	if err != nil {
		return
	}
	data, err := io.ReadAll(obj)
	return uint64(len(data)), err
}

func (c *MinioConnector) Clear() error {
	objects, err := c.ListObjects()
	if err != nil {
		return err
	}
	for _, object := range objects {
		err := c.c.RemoveObject(c.ctx, "test", object, minio.RemoveObjectOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

type EcosConnector struct {
	c         *client.Client
	ctx       context.Context
	cancel    context.CancelFunc
	ioFactory *io2.EcosIOFactory
}

func NewEcosConnector(ctx context.Context, conf *config.ClientConfig) *EcosConnector {
	ctx, cancel := context.WithCancel(ctx)
	c, err := client.New(conf)
	if err != nil {
		defer cancel()
		return nil
	}
	f, err := c.GetIOFactory("default")

	return &EcosConnector{
		c:         c,
		ctx:       ctx,
		cancel:    cancel,
		ioFactory: f,
	}
}

func (e *EcosConnector) PutObject(objectName string, data []byte) error {
	writer := e.ioFactory.GetEcosWriter(objectName)
	_, err := writer.Write(data)
	writer.Close()
	if err != nil {
		return err
	}
	return nil
}

func (e *EcosConnector) ListObjects() ([]string, error) {
	metas, err := e.c.ListObjects(e.ctx, "default", "")
	if err != nil {
		return nil, err
	}
	var objectNames []string
	for _, meta := range metas {
		_, _, key, _, _ := object.SplitID(meta.ObjId)
		objectNames = append(objectNames, key)
	}
	return objectNames, nil
}

func (e *EcosConnector) GetObject(objectName string) (uint64, error) {
	reader := e.ioFactory.GetEcosReader(objectName)
	data, err := io.ReadAll(reader)
	return uint64(len(data)), err
}

func (e *EcosConnector) Clear() error {
	objectNames, err := e.ListObjects()
	if err != nil {
		return err
	}
	for _, objectName := range objectNames {
		bucket, _ := e.c.GetVolumeOperator().Get("default")
		err = bucket.Remove(objectName)
		logger.Infof("remove object %v", objectName)
		if err != nil {
			return err
		}
	}
	return nil
}

type EcosTester struct {
	g     *Generator
	c     *Connector
	timer *time.Ticker
}

type Tester struct {
	ctx            context.Context
	g              *Generator
	c              Connector
	cancel         context.CancelFunc
	timer          *time.Ticker
	register       *prometheus.Registry
	registry       metrics.Registry
	sample         metrics.Sample
	eachObjectSame bool

	bytesWrittenUpdateTime time.Time
	bytesReadUpdateTime    time.Time

	bytesSentLast uint64
	bytesRecvLast uint64
	lastTime      time.Time

	networkSpeedTimer *time.Ticker
}

func NewTester(ctx context.Context, c Connector, same bool) *Tester {
	ctx, cancel := context.WithCancel(ctx)

	tester := &Tester{
		ctx:               ctx,
		c:                 c,
		cancel:            cancel,
		g:                 &Generator{},
		timer:             time.NewTicker(1 * time.Second),
		sample:            metrics.NewUniformSample(1000000),
		networkSpeedTimer: time.NewTicker(NetWorkTimeInterval),
		eachObjectSame:    same,
		register:          prometheus.NewRegistry(),
		registry:          metrics.NewRegistry(),
	}
	go tester.pushToPrometheus()
	go tester.monitorNetwork()
	return tester
}

func (t *Tester) monitorNetwork() {
	for {
		select {
		case <-t.ctx.Done():
			return
		case <-t.networkSpeedTimer.C:
			netStats, err := net.IOCountersWithContext(t.ctx, true)
			if err != nil {
				logger.Errorf("get network stats failed: %v", err)
				continue
			}
			for _, stat := range netStats {
				if stat.Name != "eth0" && stat.Name != "tun0" {
					continue
				}
				sentDelta := stat.BytesSent - t.bytesSentLast
				recvDelta := stat.BytesRecv - t.bytesRecvLast
				valid := t.bytesSentLast != 0 || t.bytesRecvLast != 0
				t.bytesSentLast = stat.BytesSent
				t.bytesRecvLast = stat.BytesRecv
				sendSpeed := float64(sentDelta) / NetWorkTimeInterval.Seconds()
				recvSpeed := float64(recvDelta) / NetWorkTimeInterval.Seconds()
				if valid {
					metrics.GetOrRegisterGauge("uploadSpeed", t.registry).Update(int64(sendSpeed))
					metrics.GetOrRegisterGauge("downloadSpeed", t.registry).Update(int64(recvSpeed))
					logger.Infof("network speed: %v kB/s %v kB/s", sendSpeed/1024, recvSpeed/1024)
				}
			}
		}
	}
}

func (t *Tester) pushToPrometheus() {
	prometheusClient := prometheusmetrics.NewPrometheusProvider(
		t.registry, "exp",
		"client",
		t.register, 1*time.Second)
	go prometheusClient.UpdatePrometheusMetrics()

	for {
		select {
		case <-t.ctx.Done():
			return
		case <-t.timer.C:
			err := push.New("http://gateway.prometheus.sums.top", "exp").
				Gatherer(t.register).Grouping("node", "client").Push()
			if err != nil {
				logger.Warningf("push to prometheus failed: %v", err)
			}
		}
	}
}

func (t *Tester) GetWriteBytes() int64 {
	return metrics.GetOrRegisterCounter("TotalWriteBytes", t.registry).Count()
}

func (t *Tester) TestWritePerformance(size uint64, threadNum int) {
	logger.Infof("!! start write performance test, size: %v, threadNum: %v", size, threadNum)
	data := make([]byte, size)
	t.g.FillRandom(data)
	taskPoolSize := threadNum
	wg := sync.WaitGroup{}
	f := func() {
		defer wg.Done()
		for {
			select {
			case <-t.ctx.Done():
				return
			default:
			}
			// 生成随机对象名
			logger.Infof("start write performance test")
			objectName := "test" + string(genTestData(10))
			// 生成随机数据
			start := time.Now()
			if t.eachObjectSame == false {
				t.g.FillRandom(data)
				logger.Infof("fill random data spend %v", time.Since(start))
			}
			start = time.Now()

			_ = t.c.PutObject(objectName, data)
			t.bytesWrittenUpdateTime = time.Now()
			spendTime := time.Since(start)
			speed := float64(size) / spendTime.Seconds()
			metrics.GetOrRegisterGaugeFloat64("ObjPutSpeed", t.registry).Update(speed)
			metrics.GetOrRegisterHistogram("ObjPutSpeedHistogram", t.registry, t.sample).Update(int64(speed))
			metrics.GetOrRegisterGaugeFloat64("ObjPutTime", t.registry).Update(float64(spendTime.Milliseconds()))
			metrics.GetOrRegisterCounter("ObjPutCount", t.registry).Inc(1)
			metrics.GetOrRegisterCounter("TotalWriteBytes", t.registry).Inc(int64(size))
			logger.Infof("write object %v spend %v", objectName, spendTime)
		}
	}
	for i := 0; i < taskPoolSize; i++ {
		go f()
		time.Sleep(time.Second)
		wg.Add(1)
	}
	wg.Wait()
}

func (t *Tester) TestReadPerformance() {
	objectNames, err := t.c.ListObjects()
	if err != nil {
		logger.Errorf("list objects failed: %v", err)
		return
	}
	if len(objectNames) == 0 {
		logger.Errorf("no object found")
		return
	}
	for {
		select {
		case <-t.ctx.Done():
			return
		default:
		}
		// 随机选择一个对象名
		objectName := objectNames[rand.Intn(len(objectNames))]
		start := time.Now()
		size, _ := t.c.GetObject(objectName)
		spendTime := time.Since(start)
		speed := float64(size) / spendTime.Seconds()
		metrics.GetOrRegisterGaugeFloat64("ObjGetSpeed", t.registry).Update(speed)
		metrics.GetOrRegisterHistogram("ObjGetSpeedHistogram", t.registry, t.sample).Update(int64(speed))
		metrics.GetOrRegisterGaugeFloat64("ObjGetTime", t.registry).Update(float64(spendTime.Milliseconds()))
		metrics.GetOrRegisterCounter("ObjGetCount", t.registry).Inc(1)
		metrics.GetOrRegisterCounter("TotalReadBytes", t.registry).Inc(int64(size))
		logger.Infof("read object %v spend %v", objectName, spendTime)
	}
}

func (t *Tester) Stop() {
	t.cancel()
	mean := t.sample.Mean()
	logger.Infof("mean: %v", mean)
}

func (t *Tester) GetMean() float64 {
	return t.sample.Mean()
}
