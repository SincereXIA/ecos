package rainbow

import (
	"ecos/utils/logger"
	"sync"
)

type Router struct {
	lock sync.RWMutex
	m    map[uint64]chan *Response
}

func NewRouter() *Router {
	return &Router{
		m: make(map[uint64]chan *Response),
	}
}

func (r *Router) Register(id uint64) <-chan *Response {
	r.lock.Lock()
	defer r.lock.Unlock()
	if _, ok := r.m[id]; !ok {
		r.m[id] = make(chan *Response, 1)
	} else {
		panic("dup id")
	}
	return r.m[id]
}

func (r *Router) Unregister(id uint64) {
	r.lock.Lock()
	defer r.lock.Unlock()
	close(r.m[id])
	delete(r.m, id)
}

func (r *Router) Send(id uint64, x *Response) {
	r.lock.RLock()
	ch := r.m[id]
	if ch == nil {
		logger.Errorf("router send response to channel err: no such id %d", id)
	}
	r.lock.RUnlock()
	if ch != nil {
		ch <- x
	}
}
