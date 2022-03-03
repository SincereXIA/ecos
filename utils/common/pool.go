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
	closed    bool
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
	select {
	case r, ok := <-p.resources: //检查是否有空闲的资源
		logger.Tracef("Acquire: shared resource")
		if !ok {
			return nil, errno.PoolClosed
		}
		return r, nil
	default:
		logger.Tracef("Acquire: New Resource")
		// 调用资源创建函数创建资源
		return p.factory()
	}
}

// Release 将一个使用后的资源池释放回池里
func (p *Pool) Release(r io.Closer) {
	p.m.Lock() //确保本操作和Close操作安全
	defer p.m.Unlock()
	if p.closed { //如果资源池已经关闭，销毁这个资源
		err := r.Close()
		if err != nil {
			logger.Warningf("Pool Release Err: %#v", err)
		}
		return
	}
	select {
	case p.resources <- r: //试图将这个资源放入队列
		logger.Tracef("Release: In Queue")
	default: //如果队列已满，则关闭这个资源
		logger.Tracef("Release: Closing")
		err := r.Close()
		if err != nil {
			logger.Warningf("Pool Release Err: %#v", err)
		}
	}
}

// Close 会让资源池停止工作，并关闭所有的资源
func (p *Pool) Close() {
	p.m.Lock() //确保本操作与release操作安全
	defer p.m.Unlock()
	if p.closed {
		return
	}
	p.closed = true    //将池关闭
	close(p.resources) //在清空通道里的资源前，将通道关闭，如果不这样做，会发生死锁
	//关闭资源
	for r := range p.resources {
		_ = r.Close()
	}
}
