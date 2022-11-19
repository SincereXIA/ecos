package benchmark

import (
	"context"
	"ecos/client"
	"ecos/client/config"
	io2 "ecos/client/io"
	"ecos/utils/logger"
	prometheusmetrics "github.com/deathowl/go-metrics-prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
	"github.com/rcrowley/go-metrics"
	"io"
	"math/rand"
	"time"
)

type Generator struct {
}

func (g *Generator) Generate(size uint64) []byte {
	// Generate a random byte array of size `size`
	return genTestData(int(size))
}

func genTestData(size int) []byte {
	rand.Seed(time.Now().Unix())
	directSize := 1024 * 1024 * 10
	if size < directSize {
		data := make([]byte, size)
		for idx := range data {
			if idx%100 == 0 {
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

type Tester interface {
	PutObject(objectName string, size uint64) error
	GetObject(objectName string) error
	DeleteObject(objectName string) error
	ListObjects() error
}

type EcosTester struct {
	c      *client.Client
	g      *Generator
	ctx    context.Context
	cancel context.CancelFunc

	timer    *time.Ticker
	register *prometheus.Registry

	ioFactory *io2.EcosIOFactory
}

func NewEcosTester(ctx context.Context, conf *config.ClientConfig) *EcosTester {
	ctx, cancel := context.WithCancel(ctx)
	c, err := client.New(conf)
	if err != nil {
		defer cancel()
		return nil
	}
	f, err := c.GetIOFactory("default")

	return &EcosTester{
		c:         c,
		ctx:       ctx,
		cancel:    cancel,
		g:         &Generator{},
		ioFactory: f,
	}
}

func (e *EcosTester) PutObject(objectName string, size uint64) error {
	testData := e.g.Generate(size)
	startAt := time.Now()
	w := e.ioFactory.GetEcosWriter(objectName)
	_, err := w.Write(testData)
	w.Close()
	spendTime := time.Since(startAt)

	metrics.GetOrRegisterGaugeFloat64("ObjPutSpeed", nil).Update(float64(size) / float64(spendTime.Milliseconds()) * 1000)
	metrics.GetOrRegisterGaugeFloat64("ObjPutTime", nil).Update(float64(spendTime.Milliseconds()))
	metrics.GetOrRegisterCounter("ObjPutCount", nil).Inc(int64(size))
	return err
}

func (e *EcosTester) GetObject(objectName string) error {
	f, _ := e.c.GetIOFactory("default")
	r := f.GetEcosReader(objectName)
	for {
		_, err := r.Read(make([]byte, 1024))
		if err == io.EOF {
			break
		}
	}
	return nil
}

func (e *EcosTester) TestPerformance() {
	// 启动推送网关
	go e.pushToPrometheus()
	e.timer = time.NewTicker(1 * time.Second)
	for {
		select {
		case <-e.ctx.Done():
			return
		default:
		}
		// 生成随机对象名
		objectName := "test" + string(genTestData(10))
		_ = e.PutObject(objectName, 1024*10)
	}
}

func (e *EcosTester) Stop() {
	e.cancel()
}

func (e *EcosTester) pushToPrometheus() {
	e.register = prometheus.NewRegistry()
	prometheusClient := prometheusmetrics.NewPrometheusProvider(
		metrics.DefaultRegistry, "exp",
		"client",
		e.register, 1*time.Second)
	go prometheusClient.UpdatePrometheusMetrics()

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-e.timer.C:
			err := push.New("http://gateway.prometheus.sums.top", "exp").
				Gatherer(e.register).Grouping("node", "client").Push()
			if err != nil {
				logger.Warningf("push to prometheus failed: %v", err)
			}
		}
	}
}
