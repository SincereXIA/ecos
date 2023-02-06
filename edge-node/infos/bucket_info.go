package infos

import (
	"ecos/utils/logger"
	"path"
	"strings"
)

func (m *BucketInfo) GetInfoType() InfoType {
	return InfoType_BUCKET_INFO
}

func (m *BucketInfo) BaseInfo() *BaseInfo {
	return &BaseInfo{Info: &BaseInfo_BucketInfo{BucketInfo: m}}
}

func (m *BucketInfo) GetID() string {
	return GenBucketID(m.VolumeId, m.BucketName)
}

func GenBucketID(volumeId, bucketName string) string {
	return path.Join("/", volumeId, bucketName)
}

// GenBucketInfo generates a new BucketInfo from a volume and a bucket name.
// It is usually used to create a new Bucket.
func GenBucketInfo(volumeID, bucketName, userID string) *BucketInfo {
	volumeID = path.Clean(volumeID)
	bucketName = path.Clean(bucketName)
	if strings.Contains(bucketName, "/") || strings.Contains(volumeID, "/") {
		logger.Errorf("bucketName %s contains /", bucketName)
		return nil
	}
	return &BucketInfo{
		VolumeId:   volumeID,
		BucketName: bucketName,
		UserId:     userID,
		Config: &BucketConfig{
			KeySlotNum:           5,
			BlockSize:            4 * 1024 * 1024, // 4MB default
			HistoryVersionEnable: false,
			BlockHashType:        BucketConfig_MURMUR3_128,
			ObjectHashType:       BucketConfig_MURMUR3_128,
			MaxExtraDataSize:     1024,
		},
	}
}

// GetBucketID returns the bucket ID by volume ID and bucket name.
func GetBucketID(volumeID, bucketName string) string {
	volumeID = path.Clean(volumeID)
	bucketName = path.Clean(bucketName)
	if strings.Contains(bucketName, "/") || strings.Contains(volumeID, "/") {
		logger.Errorf("bucketName %s contains /", bucketName)
		return ""
	}
	return GenBucketID(volumeID, bucketName)
}
