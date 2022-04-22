package alaya

import (
	"ecos/edge-node/object"
	"ecos/utils/timestamp"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMemoryMetaStorage(t *testing.T) {
	storage := NewMemoryMetaStorage()
	for i := 0; i < 5; i++ {
		_ = storage.RecordMeta(&object.ObjectMeta{
			ObjId:      "",
			Status:     object.ObjectMeta_STATUS_OK,
			ObjSize:    100,
			UpdateTime: timestamp.Now(),
			ObjHash:    "",
			PgId:       1,
			Blocks:     nil,
			Term:       1,
			MetaData:   nil,
		})
	}
	snapshot, err := storage.CreateSnapshot()
	if err != nil {
		t.Error(err)
	}
	assert.NotNil(t, snapshot)
}
