package router

import (
	"bytes"
	"context"
	"ecos/edge-node/infos"
	edgeNodeTest "ecos/shared"
	"ecos/shared/moon"
	"ecos/utils/common"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/assert"
	"io"
	"math/rand"
	"mime/multipart"
	"net/http"
	"os"
	"testing"
)

func TestGateway(t *testing.T) {
	// Prepare test environment
	{
		ctx, cancel := context.WithCancel(context.Background())
		basePath := "./ecos-data/"

		t.Cleanup(func() {
			cancel()
			_ = os.RemoveAll(basePath)
		})
		_ = common.InitAndClearPath(basePath)
		watchers, _, _ := edgeNodeTest.RunTestEdgeNodeCluster(t, ctx, true, basePath, 9)

		// Add a test bucket first
		bucketName := "default"
		bucketInfo := infos.GenBucketInfo("root", bucketName, "root")

		_, err := watchers[0].GetMoon().ProposeInfo(ctx, &moon.ProposeInfoRequest{
			Head:     nil,
			Operate:  moon.ProposeInfoRequest_ADD,
			Id:       bucketInfo.GetID(),
			BaseInfo: bucketInfo.BaseInfo(),
		})
		if err != nil {
			t.Errorf("Failed to add bucket: %v", err)
		}

		go func() {
			nodeInfo := watchers[0].GetSelfInfo()
			DefaultConfig.Host = nodeInfo.IpAddr
			DefaultConfig.Port = nodeInfo.RpcPort
			gateway := NewRouter(DefaultConfig)
			_ = gateway.Run(":3266")
		}()
	}

	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			PartitionID:       "ecos",
			URL:               "http://localhost:3266",
			SigningRegion:     "cn",
			HostnameImmutable: true,
		}, nil
	})
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithEndpointResolverWithOptions(customResolver))
	if err != nil {
		t.Errorf("Failed to init AWS SDK config: %v", err)
	}
	client := s3.NewFromConfig(cfg)

	test10MBBuffer := genTestData(10 << 20)
	reader := bytes.NewReader(test10MBBuffer)

	// Test ListAllBuckets
	t.Run("ListAllBuckets", func(t *testing.T) {
		testListAllBuckets(t, client, 1) // ["default"]
	})

	// Create a test bucket
	t.Run("CreateBucket", func(t *testing.T) {
		bucketName := "test"
		testCreateBucket(t, client, bucketName, false)
	})

	// Test Head Bucket
	t.Run("HeadBucket", func(t *testing.T) {
		testHeadBucket(t, client, "wrongBucket", true)
		testHeadBucket(t, client, "test", false)
	})

	// Test List Bucket
	t.Run("ListBucket", func(t *testing.T) {
		// Wrong Bucket Name
		wrongBucket := "wrongBucket"
		testListObjects(t, client, wrongBucket, true, 0)
		testListObjectsV2(t, client, wrongBucket, true, 0)
		// Right Bucket Name
		trueBucket := "test-list-bucket"
		testCreateBucket(t, client, trueBucket, false)
		testListObjects(t, client, trueBucket, false, 0)
		testListObjectsV2(t, client, trueBucket, false, 0)
		_, _ = reader.Seek(0, io.SeekStart)
		testPutObject(t, client, trueBucket, "1.obj", reader)
		testListObjects(t, client, trueBucket, false, 1)
		testListObjectsV2(t, client, trueBucket, false, 1)
	})

	// Test PUT Object
	t.Run("PutObject", func(t *testing.T) {
		// Normal Data
		testPutObject(t, client, "default", "testPUTObject.obj", reader)
		// Blank Data
		testPutObject(t, client, "default", "testPUTEmptyObject.obj", nil)
	})

	// Test POST Object
	t.Run("PostObject", func(t *testing.T) {
		_, _ = reader.Seek(0, io.SeekStart)
		// Normal Data
		testPostObject(t, client, "default", "testPOSTObject.obj", reader)
		// Blank Data
		testPostObject(t, client, "default", "testPOSTEmptyObject.obj", nil)
	})

	// Test HEAD Object
	t.Run("HeadObject", func(t *testing.T) {
		testHeadObject(t, client, "default", "testPUTObject.obj", false)
		testHeadObject(t, client, "default", "testPOSTObject.obj", false)
	})

	// Test GET Object
	t.Run("GetObject", func(t *testing.T) {
		_, _ = reader.Seek(0, io.SeekStart)
		testPutObject(t, client, "default", "testGETObject_PUT.obj", reader)
		testPostObject(t, client, "default", "testGETObject_POST.obj", reader)
		obj := testGetObject(t, client, "default", "testGETObject_PUT.obj", false)
		content, err := io.ReadAll(obj)
		assert.NoError(t, err)
		assert.Equal(t, string(test10MBBuffer), string(content))
		obj = testGetObject(t, client, "default", "testGETObject_POST.obj", false)
		content, err = io.ReadAll(obj)
		assert.NoError(t, err)
		if !bytes.Equal(content, nil) {
			t.Errorf("GetObject: content is not equal")
		}
	})

	// Test DELETE Object
	t.Run("DeleteObject", func(t *testing.T) {
		bucketName := "test-delete-bucket"
		testCreateBucket(t, client, bucketName, false)
		_, _ = reader.Seek(0, io.SeekStart)
		testPutObject(t, client, bucketName, "testDeleteObject.obj", reader)
		testListObjects(t, client, bucketName, false, 1)
		// Normal Delete
		testDeleteObject(t, client, bucketName, "testDeleteObject.obj", false)
		testListObjects(t, client, bucketName, false, 0)
		testHeadObject(t, client, bucketName, "testDeleteObject.obj", true)
		// Delete non-exist object
		// testDeleteObject(t, client, "default", "testPUTObject2.obj", false)
		testListObjects(t, client, bucketName, false, 0)
	})

	// Test DeleteObjects
	t.Run("DeleteObjects", func(t *testing.T) {
		bucketName := "test-delete-objects-bucket"
		testCreateBucket(t, client, bucketName, false)
		_, _ = reader.Seek(0, io.SeekStart)
		testPutObject(t, client, bucketName, "testDeleteObjects1.obj", reader)
		_, _ = reader.Seek(0, io.SeekStart)
		testPutObject(t, client, bucketName, "testDeleteObjects2.obj", reader)
		// Normal Delete
		testDeleteObjects(t, client, bucketName, []string{"testDeleteObjects1.obj", "testDeleteObjects2.obj"}, false)
		testListObjects(t, client, bucketName, false, 0)
		// Delete non-exist objects
		// testDeleteObjects(t, client, "default", []string{"testPUTObject.obj", "testPOSTObject.obj", "testPUTEmptyObject.obj", "testPOSTEmptyObject.obj"}, true)
		testListObjects(t, client, bucketName, false, 0)
	})

	// Test Multipart Upload
	t.Run("MultipartUpload", func(t *testing.T) {
		bucketName := "test-multipart-upload-bucket"
		testCreateBucket(t, client, bucketName, false)
		testListMultipartUploads(t, client, bucketName, false, 0)

		uploadId := testCreateMultipartUpload(t, client, bucketName, "testMultipartUpload.obj", false)
		testListMultipartUploads(t, client, bucketName, false, 1)
		var partFile []io.ReadSeeker
		for i := 0; i < 5; i++ {
			reader := bytes.NewReader(genTestData(10 << 20))
			partFile = append(partFile, reader)
		}
		testListParts(t, client, bucketName, "testMultipartUpload.obj", uploadId, false, 0)
		testUploadPart(t, client, bucketName, "testMultipartUpload.obj", uploadId, 1, partFile[0], false)
		testUploadPart(t, client, bucketName, "testMultipartUpload.obj", uploadId, 2, partFile[1], false)
		testUploadPart(t, client, bucketName, "testMultipartUpload.obj", uploadId, 100, partFile[4], false)
		testUploadPart(t, client, bucketName, "testMultipartUpload.obj", uploadId, 50, partFile[3], false)
		testUploadPart(t, client, bucketName, "testMultipartUpload.obj", uploadId, 20, partFile[2], false)
		testListParts(t, client, bucketName, "testMultipartUpload.obj", uploadId, false, 5)
		_, _ = partFile[0].Seek(0, io.SeekStart)
		testUploadPart(t, client, bucketName, "testMultipartUpload.obj", uploadId, 100, partFile[0], false)
		testListParts(t, client, bucketName, "testMultipartUpload.obj", uploadId, false, 5)
		parts := []types.CompletedPart{
			{
				PartNumber: 1,
			},
			{
				PartNumber: 2,
			},
			{
				PartNumber: 20,
			},
			{
				PartNumber: 50,
			},
			{
				PartNumber: 100,
			},
		}
		testCompleteMultipartUpload(t, client, bucketName, "testMultipartUpload.obj", uploadId, parts, false)
		testListMultipartUploads(t, client, bucketName, false, 0)
		testListObjects(t, client, bucketName, false, 1)
		obj := testGetObject(t, client, bucketName, "testMultipartUpload.obj", false)
		content, err := io.ReadAll(obj)
		assert.NoError(t, err)
		var fullContent []byte
		for i := 0; i < 4; i++ {
			_, _ = partFile[i].Seek(0, io.SeekStart)
			content, _ := io.ReadAll(partFile[i])
			fullContent = append(fullContent, content...)
		}
		_, _ = partFile[0].Seek(0, io.SeekStart)
		lastContent, _ := io.ReadAll(partFile[0])
		fullContent = append(fullContent, lastContent...)
		result := bytes.Compare(fullContent, content)
		assert.Zero(t, result)
		if result != 0 {
			t.Errorf("GetObject: content is not equal")
			t.Logf("expected size: %d, actual size: %d", len(fullContent), len(content))
		}

		uploadId = testCreateMultipartUpload(t, client, bucketName, "testMultipartUpload2.obj", false)
		_, _ = reader.Seek(0, io.SeekStart)
		testUploadPart(t, client, bucketName, "testMultipartUpload2.obj", uploadId, 1, reader, false)
		testAbortMultipartUpload(t, client, bucketName, "testMultipartUpload2.obj", uploadId, false)
		testListMultipartUploads(t, client, bucketName, false, 0)
		testListObjects(t, client, bucketName, false, 1)
	})
}

func testListAllBuckets(t *testing.T, client *s3.Client, wantLength int) {
	listBucketsOutput, err := client.ListBuckets(context.TODO(), nil)
	if err != nil {
		t.Errorf("Failed to list buckets: %v", err)
	}
	assert.Equal(t, wantLength, len(listBucketsOutput.Buckets))
	t.Log("ListAllBuckets:")
	for _, bucket := range listBucketsOutput.Buckets {
		t.Logf("\tBucket: %s, CreationDate: %s", *bucket.Name, bucket.CreationDate)
	}
}

func testCreateBucket(t *testing.T, client *s3.Client, bucketName string, wantErr bool) {
	createBucketOutput, err := client.CreateBucket(context.TODO(), &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	if wantErr {
		assert.Error(t, err)
		return
	}
	assert.NoError(t, err)
	t.Logf("CreateBucketOutput: %v", createBucketOutput)
}

func testHeadBucket(t *testing.T, client *s3.Client, bucketName string, wantErr bool) {
	headBucketOutput, err := client.HeadBucket(context.TODO(), &s3.HeadBucketInput{
		Bucket: aws.String(bucketName),
	})
	if wantErr {
		assert.Error(t, err)
		return
	}
	assert.NoError(t, err)
	t.Logf("HeadBucketOutput: %v", headBucketOutput)
}

func testListObjects(t *testing.T, client *s3.Client, bucketName string, wantErr bool, wantLength int) {
	listObjectsOutput, err := client.ListObjects(context.TODO(), &s3.ListObjectsInput{
		Bucket: aws.String(bucketName),
	})
	if wantErr {
		assert.Error(t, err)
		return
	}
	assert.NoError(t, err)
	assert.Equal(t, wantLength, len(listObjectsOutput.Contents))
	t.Logf("ListObjects: %#v", listObjectsOutput)
	for _, obj := range listObjectsOutput.Contents {
		t.Logf("Object: {Key: {%s}\t Size: {%v}}", *obj.Key, obj.Size)
	}
}

func testListObjectsV2(t *testing.T, client *s3.Client, bucketName string, wantErr bool, wantLength int) {
	listObjectsV2Output, err := client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
	})
	if wantErr {
		assert.Error(t, err)
		return
	}
	assert.NoError(t, err)
	assert.Equal(t, wantLength, len(listObjectsV2Output.Contents))
	t.Logf("ListObjects: %#v", listObjectsV2Output)
	for _, obj := range listObjectsV2Output.Contents {
		t.Logf("Object: {Key: {%s}\t Size: {%v}}", *obj.Key, obj.Size)
	}
}

func testPutObject(t *testing.T, client *s3.Client, bucketName string, key string, data io.Reader) {
	putObjectOutput, err := client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
		Body:   data,
	})
	assert.NoError(t, err)
	t.Logf("PutObject: %#v", putObjectOutput)
}

func testPostObject(t *testing.T, _ *s3.Client, bucketName string, key string, data io.Reader) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", key)
	assert.NoError(t, err)
	if data != nil {
		_, err = io.Copy(part, data)
		assert.NoError(t, err)
	}
	assert.NoError(t, writer.WriteField("key", key))
	assert.NoError(t, writer.Close())
	postObjectOutput, err := http.Post(fmt.Sprintf("http://localhost:3266/%s", bucketName),
		writer.FormDataContentType(), body)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, postObjectOutput.StatusCode)
}

func testHeadObject(t *testing.T, client *s3.Client, bucketName string, key string, wantErr bool) {
	headObjectOutput, err := client.HeadObject(context.TODO(), &s3.HeadObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})
	if wantErr {
		assert.Error(t, err)
		return
	}
	assert.NoError(t, err)
	t.Logf("HeadObject: %#v", headObjectOutput)
}

func testGetObject(t *testing.T, client *s3.Client, bucketName string, key string, wantErr bool) io.Reader {
	getObjectOutput, err := client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})
	if wantErr {
		assert.Error(t, err)
		return nil
	}
	assert.NoError(t, err)
	t.Logf("GetObject: %#v", getObjectOutput)
	return getObjectOutput.Body
}

func testDeleteObject(t *testing.T, client *s3.Client, bucketName string, key string, wantErr bool) {
	deleteObjectOutput, err := client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})
	if wantErr {
		assert.Error(t, err)
		return
	}
	assert.NoError(t, err)
	t.Logf("DeleteObject: %#v", deleteObjectOutput)
}

func testDeleteObjects(t *testing.T, client *s3.Client, bucketName string, keys []string, wantErr bool) {
	objs := make([]types.ObjectIdentifier, len(keys))
	for i, key := range keys {
		objs[i] = types.ObjectIdentifier{
			Key: aws.String(key),
		}
	}
	deleteObjectsOutput, err := client.DeleteObjects(context.TODO(), &s3.DeleteObjectsInput{
		Bucket: aws.String(bucketName),
		Delete: &types.Delete{
			Objects: objs,
		},
	})
	if wantErr {
		assert.Error(t, err)
		return
	}
	assert.NoError(t, err)
	t.Logf("DeleteObjects: %#v", deleteObjectsOutput)
}

func testCreateMultipartUpload(t *testing.T, client *s3.Client, bucketName string, key string, wantErr bool) string {
	createMultipartUploadOutput, err := client.CreateMultipartUpload(context.TODO(), &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})
	if wantErr {
		assert.Error(t, err)
		return ""
	}
	assert.NoError(t, err)
	t.Logf("CreateMultipartUpload: %#v", createMultipartUploadOutput)
	if err != nil {
		return ""
	}
	return *createMultipartUploadOutput.UploadId
}

func testUploadPart(t *testing.T, client *s3.Client, bucketName string, key string, uploadId string, partNumber int32, data io.Reader, wantErr bool) {
	uploadPartOutput, err := client.UploadPart(context.TODO(), &s3.UploadPartInput{
		Bucket:     aws.String(bucketName),
		Key:        aws.String(key),
		UploadId:   aws.String(uploadId),
		PartNumber: partNumber,
		Body:       data,
	})
	if wantErr {
		assert.Error(t, err)
		return
	}
	assert.NoError(t, err)
	t.Logf("UploadPart: %#v", uploadPartOutput)
}

func testCompleteMultipartUpload(t *testing.T, client *s3.Client, bucketName string, key string, uploadId string, parts []types.CompletedPart, wantErr bool) {
	completeMultipartUploadOutput, err := client.CompleteMultipartUpload(context.TODO(), &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(bucketName),
		Key:      aws.String(key),
		UploadId: aws.String(uploadId),
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: parts,
		},
	})
	if wantErr {
		assert.Error(t, err)
		return
	}
	assert.NoError(t, err)
	t.Logf("CompleteMultipartUpload: %#v", completeMultipartUploadOutput)
}

func testAbortMultipartUpload(t *testing.T, client *s3.Client, bucketName string, key string, uploadId string, wantErr bool) {
	abortMultipartUploadOutput, err := client.AbortMultipartUpload(context.TODO(), &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(bucketName),
		Key:      aws.String(key),
		UploadId: aws.String(uploadId),
	})
	if wantErr {
		assert.Error(t, err)
		return
	}
	assert.NoError(t, err)
	if err != nil {
		t.Errorf("AbortMultipartUpload Error: %v", err)
	}
	t.Logf("AbortMultipartUpload: %#v", abortMultipartUploadOutput)
}

func testListMultipartUploads(t *testing.T, client *s3.Client, bucketName string, wantErr bool, wantLength int) {
	listMultipartUploadsOutput, err := client.ListMultipartUploads(context.TODO(), &s3.ListMultipartUploadsInput{
		Bucket: aws.String(bucketName),
	})
	if wantErr {
		assert.Error(t, err)
		return
	}
	assert.NoError(t, err)
	t.Logf("ListMultipartUploads: %#v", listMultipartUploadsOutput)
	if err != nil {
		return
	}
	if wantLength > 0 {
		assert.Equal(t, wantLength, len(listMultipartUploadsOutput.Uploads))
	}
}

func testListParts(t *testing.T, client *s3.Client, bucketName string, key string, uploadId string, wantErr bool, wantLength int) {
	listPartsOutput, err := client.ListParts(context.TODO(), &s3.ListPartsInput{
		Bucket:   aws.String(bucketName),
		Key:      aws.String(key),
		UploadId: aws.String(uploadId),
	})
	if wantErr {
		assert.Error(t, err)
		return
	}
	assert.NoError(t, err)
	t.Logf("ListParts: %#v", listPartsOutput)
	if err != nil {
		return
	}
	if wantLength > 0 {
		assert.Equal(t, len(listPartsOutput.Parts), wantLength)
	}
}

func genTestData(size int) []byte {
	data := make([]byte, size)
	for idx := range data {
		if idx%100 == 0 {
			data[idx] = '\n'
		} else {
			data[idx] = byte(rand.Intn(26) + 97)
		}
	}
	return data
}
