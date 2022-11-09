package benchmark

import (
	"context"
	"ecos/client"
	"ecos/client/config"
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
	c   *client.Client
	g   *Generator
	ctx context.Context
}

func NewEcosTester(conf *config.ClientConfig) *EcosTester {
	c, err := client.New(conf)
	if err != nil {
		return nil
	}
	return &EcosTester{
		c: c,
		g: &Generator{},
	}
}

func (e *EcosTester) PutObject(objectName string, size uint64) error {
	f, _ := e.c.GetIOFactory("default")
	w := f.GetEcosWriter(objectName)
	_, err := w.Write(e.g.Generate(size))
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
	for {
		select {
		case <-e.ctx.Done():
			return
		default:
		}
		_ = e.PutObject("test", 1024*1024*10)

	}
}
