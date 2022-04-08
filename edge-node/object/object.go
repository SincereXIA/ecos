package object

import (
	"ecos/edge-node/infos"
	utilsCommon "ecos/utils/common"
	"errors"
	"path"
	"strconv"
	"strings"
)

func GenSlotPgID(bucketID string, slot, pgNum int32) uint64 {
	str := bucketID + "/" + string(slot)
	pg := utilsCommon.NewMapper(uint64(pgNum)).MapIDtoPG(str)
	return pg
}

// CalculateSlot returns the slot number of the object
// 1 <= slot number <= slotNum
func CalculateSlot(objectKey string, slotNum int32) int32 {
	m := utilsCommon.NewMapper(uint64(slotNum))
	slot := m.MapIDtoPG(objectKey)
	return int32(slot)
}

func SplitID(objectID string) (volumeID, bucketID, key string, slotID int32, err error) {
	objectID = path.Clean(objectID)
	objectID = strings.TrimPrefix(objectID, "/")
	split := strings.SplitN(objectID, "/", 4)
	if len(split) != 4 {
		// TODO error
		return "", "", "", 0, errors.New("split objectID error")
	}
	var bucketName string
	volumeID, bucketName, key = split[0], split[1], split[3]
	bucketID = infos.GenBucketID(volumeID, bucketName)
	key = strings.TrimPrefix(key, "/")
	slot, err := strconv.Atoi(split[2])
	if err != nil {
		return "", "", "", 0, errors.New("split objectID error")
	}
	slotID = int32(slot)
	return
}

func GenObjPgID(bucketInfo *infos.BucketInfo, objectKey string, pgNum int32) (pgID uint64) {
	objectKey = strings.TrimPrefix(objectKey, "/")
	bucketID := bucketInfo.GetID()
	slotNum := bucketInfo.GetConfig().KeySlotNum
	pgID = GenSlotPgID(bucketID, CalculateSlot(objectKey, slotNum), pgNum)
	return pgID
}

// GenObjectId Generates ObjectId for a given object
func GenObjectId(bucketInfo *infos.BucketInfo, key string) string {
	prefix := bucketInfo.GetID()
	slot := CalculateSlot(key, bucketInfo.GetConfig().KeySlotNum)
	key = path.Clean(key)
	objID := path.Join(prefix, strconv.FormatInt(int64(slot), 10), key)
	return objID
}

// GenBlockPgID Generates Block PgID for a given block
func GenBlockPgID(blockID string, pgNum int32) uint64 {
	m := utilsCommon.NewMapper(uint64(pgNum))
	pg := m.MapIDtoPG(blockID)
	return pg
}
