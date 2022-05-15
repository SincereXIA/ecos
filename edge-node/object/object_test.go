package object

import (
	"ecos/edge-node/infos"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCalculateSlot(t *testing.T) {
	s1 := CalculateSlot("path/to/keyA", 5)
	s2 := CalculateSlot("/path/to/keyA", 5)
	assert.Equal(t, s1, s2)
	key := CleanObjectKey("/path//to/keyA")
	assert.Equal(t, "path/to/keyA", key)

	objID := GenObjectId(&infos.BucketInfo{
		VolumeId:   "root",
		BucketName: "default",
		UserId:     "root",
		GroupId:    "",
		Mode:       0,
		Config: &infos.BucketConfig{
			KeySlotNum:           5,
			BlockSize:            0,
			BlockHashEnable:      false,
			ObjectHashEnable:     false,
			HistoryVersionEnable: false,
			BlockHashType:        0,
			ObjectHashType:       0,
		},
	}, "path/to/keyA")
	assert.Equal(t, "root/default/1/path/to/keyA", objID)
}
