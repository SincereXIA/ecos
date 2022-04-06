package common

import (
	"ecos/utils/logger"
	"github.com/serialx/hashring"
	"strconv"
	"sync"
)

type Mapper struct {
	ring *hashring.HashRing
}

var mapperCatch = sync.Map{}

func NewMapper(pgNum uint64) *Mapper {
	// Get from cache
	if m, ok := mapperCatch.Load(pgNum); ok {
		return m.(*Mapper)
	}
	// Create new
	pgs := make([]string, 0, pgNum)
	for i := uint64(1); i <= pgNum; i++ {
		pgs = append(pgs, strconv.FormatUint(i, 10))
	}
	m := &Mapper{ring: hashring.New(pgs)}
	mapperCatch.Store(pgNum, m)
	return m
}

func (m *Mapper) MapIDtoPG(id string) uint64 {
	ringID, ok := m.ring.GetNode(id)
	if !ok {
		logger.Fatalf("Mapper map id: %v to PG fail", id)
		return 0
	}
	pgID, err := strconv.ParseUint(ringID, 10, 64)
	if err != nil {
		logger.Fatalf("Mapper Parse ringID to pgID err: %v", ringID)
		return 0
	}
	return pgID
}
