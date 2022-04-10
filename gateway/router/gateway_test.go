package router

import (
	"bytes"
	"context"
	"ecos/edge-node/infos"
	"ecos/edge-node/moon"
	edgeNodeTest "ecos/edge-node/test"
	"ecos/utils/common"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/assert"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"testing"
	"time"
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
		watchers, _ := edgeNodeTest.RunTestEdgeNodeCluster(t, ctx, true, basePath, 9)

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

	// Test List Bucket
	{
		// Wrong Bucket Name
		wrongBucket := "wrongBucket"
		testListObjects(t, client, wrongBucket, true, 0)
		testListObjectsV2(t, client, wrongBucket, true, 0)
		// Right Bucket Name
		testListObjects(t, client, "default", false, 0)
		testListObjectsV2(t, client, "default", false, 0)
	}

	test10MBBuffer := make([]byte, 1024*1024*10)
	reader := bytes.NewReader(test10MBBuffer) // 10 MB

	// Test PUT Object
	{
		// Normal Data
		testPutObject(t, client, "default", "testPUTObject.obj", reader)
		// Blank Data
		testPutObject(t, client, "default", "testPUTEmptyObject.obj", nil)
	}

	// Test POST Object
	{
		_, _ = reader.Seek(0, io.SeekStart)
		// Normal Data
		testPostObject(t, client, "default", "testPOSTObject.obj", reader)
		// Blank Data
		testPostObject(t, client, "default", "testPOSTEmptyObject.obj", nil)
	}

	// Test HEAD Object
	{
		testHeadObject(t, client, "default", "testPUTObject.obj", false)
		testHeadObject(t, client, "default", "testPOSTObject.obj", false)
	}

	// Test GET Object
	{
		testGetObject(t, client, "default", "testPUTObject.obj", false)
		testGetObject(t, client, "default", "testPOSTObject.obj", false)
	}

	// Test ListObject again
	{
		testListObjects(t, client, "default", false, 4)
		testListObjectsV2(t, client, "default", false, 4)
	}

	// Upload another object
	{
		_, _ = reader.Seek(0, io.SeekStart)
		testPutObject(t, client, "default", "testPUTObject2.obj", reader)
	}

	time.Sleep(time.Second)

	// Test DELETE Object
	{
		// Normal Delete
		testDeleteObject(t, client, "default", "testPUTObject2.obj", false)
		testListObjects(t, client, "default", false, 3)
		// Delete non-exist object
		testDeleteObject(t, client, "default", "testPUTObject2.obj", true)
		testListObjects(t, client, "default", false, 3)
	}

	// Test DeleteObjects
	{
		// Normal Delete
		testDeleteObjects(t, client, "default", []string{"testPUTObject.obj", "testPOSTObject.obj", "testPUTEmptyObject.obj", "testPOSTEmptyObject.obj"}, false)
		testListObjects(t, client, "default", false, 0)
		// Delete non-exist objects
		testDeleteObjects(t, client, "default", []string{"testPUTObject.obj", "testPOSTObject.obj", "testPUTEmptyObject.obj", "testPOSTEmptyObject.obj"}, true)
		testListObjects(t, client, "default", false, 0)
	}
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

func testGetObject(t *testing.T, client *s3.Client, bucketName string, key string, wantErr bool) {
	getObjectOutput, err := client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})
	if wantErr {
		assert.Error(t, err)
		return
	}
	assert.NoError(t, err)
	t.Logf("GetObject: %#v", getObjectOutput)
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
