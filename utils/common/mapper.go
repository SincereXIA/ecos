package common

import (
	"ecos/utils/logger"
	"github.com/serialx/hashring"
	"strconv"
)

type Mapper struct {
	ring *hashring.HashRing
}

func NewMapper(pgNum uint64) Mapper {
	pgs := make([]string, 0, pgNum)
	for i := uint64(1); i <= pgNum; i++ {
		pgs = append(pgs, strconv.FormatUint(i, 10))
	}
	return Mapper{ring: hashring.New(pgs)}
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
