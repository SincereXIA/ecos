package gaia

import (
	"ecos/utils/database"
	gorocksdb "github.com/SUMStudio/grocksdb"
)

type StaticStorage struct {
	db          *gorocksdb.DB
	samplingNum int
	lruPoolSize int
}

func (s *StaticStorage) Visit(key string) {
	it := s.db.NewIterator(database.ReadOpts)
	it.Seek([]byte(key))
	for it.Next(); it.Valid(); it.Next() {

	}
}
