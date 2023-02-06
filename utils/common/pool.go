package common

import (
	"ecos/utils/errno"
	"ecos/utils/logger"
	"io"
	"sync"
)

// Pool 管理一组可以安全地在多个goroutine间共享得到资源。被管理的资源必须实现 io.Closer 接口
type Pool struct {
	m         sync.Mutex
	resources chan io.Closer
	factory   func() (io.Closer, error)
	curCount  int
	maxCount  int
}

// NewPool 创建一个用来管理资源的池，这个池需要一个可以分配新资源的函数，并规定池的大小
func NewPool(fn func() (io.Closer, error), size uint, maxCount int) (*Pool, error) {
	if size == 0 {
		return nil, errno.ZeroSize
	}
	return &Pool{
		factory:   fn,
		resources: make(chan io.Closer, size),
		curCount:  0,
		maxCount:  maxCount,
	}, nil
}

// Acquire 从池中获取一个资源
func (p *Pool) Acquire() (io.Closer, error) {
	if p.curCount < p.maxCount && len(p.resources) < p.maxCount {
		res, err := p.factory()
		if err != nil {
			return nil, err
		}
		p.resources <- res
		p.curCount++
	}
	r, ok := <-p.resources //检查是否有空闲的资源
	logger.Tracef("Acquire: shared resource")
	if !ok {
		return nil, errno.PoolClosed
	}
	return r, nil
}

// AcquireMultiple 从池中获取多个资源
func (p *Pool) AcquireMultiple(count int) ([]io.Closer, error) {
	p.m.Lock()
	defer p.m.Unlock()
	if count == 0 {
		return nil, errno.ZeroSize
	}
	ret := make([]io.Closer, 0, count)
	for i := 0; i < count; i++ {
		res, err := p.Acquire()
		if err != nil {
			for _, r := range ret {
				p.Release(r)
			}
			return nil, err
		}
		ret = append(ret, res)
	}
	return ret, nil
}

// Release 将一个使用后的资源池释放回池里
func (p *Pool) Release(r io.Closer) {
	r.Close()
	p.resources <- r
}
