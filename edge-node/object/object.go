package object

import (
	"ecos/edge-node/infos"
	utilsCommon "ecos/utils/common"
	"errors"
	"path"
	"strings"
)

func genPgID(bucketID string, slot, pgNum int32) uint64 {
	str := bucketID + "/" + string(slot)
	pg := utilsCommon.NewMapper(uint64(pgNum)).MapIDtoPG(str)
	return pg
}

func CalculateSlot(objectKey string, slotNum int32) int32 {
	m := utilsCommon.NewMapper(uint64(slotNum))
	slot := m.MapIDtoPG(objectKey)
	return int32(slot)
}

func SplitID(objectID string) (volumeID, bucketID, key string, err error) {
	objectID = path.Clean(objectID)
	objectID = strings.TrimPrefix(objectID, "/")
	split := strings.SplitN(objectID, "/", 3)
	if len(split) != 3 {
		// TODO error
		return "", "", "", errors.New("split objectID error")
	}
	volumeID, bucketID, key = split[0], split[1], split[2]
	bucketID = path.Join(volumeID, bucketID)
	key = strings.TrimPrefix(key, "/")
	return
}

func GenObjPgID(bucketInfo *infos.BucketInfo, objectKey string, pgNum int32) (pgID uint64) {
	objectKey = strings.TrimPrefix(objectKey, "/")
	bucketID := bucketInfo.GetID()
	slotNum := bucketInfo.GetConfig().KeySlotNum
	pgID = genPgID(bucketID, CalculateSlot(objectKey, slotNum), pgNum)
	return pgID
}

// GenObjectId Generates ObjectId for a given object
func GenObjectId(bucketInfo *infos.BucketInfo, key string) string {
	prefix := bucketInfo.GetID()
	key = path.Clean(key)
	objID := path.Join(prefix, key)
	return objID
}
