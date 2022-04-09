package router

import (
	"bytes"
	"context"
	"ecos/edge-node/infos"
	"ecos/edge-node/moon"
	edgeNodeTest "ecos/edge-node/test"
	"ecos/utils/common"
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
		listObjectsOutput, err := client.ListObjects(context.TODO(), &s3.ListObjectsInput{
			Bucket: aws.String("bucket"),
		})
		assert.Error(t, err)
		listObjectsV2Output, err := client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
			Bucket: aws.String("bucket"),
		})
		assert.Error(t, err)

		// Right Bucket Name
		listObjectsOutput, err = client.ListObjects(context.TODO(), &s3.ListObjectsInput{
			Bucket: aws.String("default"),
		})
		assert.NoError(t, err)
		assert.Equal(t, 0, len(listObjectsOutput.Contents))

		listObjectsV2Output, err = client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
			Bucket: aws.String("default"),
		})
		assert.NoError(t, err)
		assert.Equal(t, 0, len(listObjectsV2Output.Contents))
	}

	test10MBBuffer := make([]byte, 1024*1024*10)
	reader := bytes.NewReader(test10MBBuffer) // 10 MB

	// Test PUT Object
	{
		putObjectOutput, err := client.PutObject(context.TODO(), &s3.PutObjectInput{
			Bucket: aws.String("default"),
			Key:    aws.String("testPutObject.obj"),
			Body:   reader,
		})
		assert.NoError(t, err)
		t.Logf("PutObject: %#v", putObjectOutput)
	}

	// Test POST Object
	{
		_, _ = reader.Seek(0, io.SeekStart)
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, err := writer.CreateFormFile("file", "testPostObject.obj")
		assert.NoError(t, err)
		_, err = io.Copy(part, reader)
		assert.NoError(t, err)
		assert.NoError(t, writer.WriteField("key", "testPostObject.obj"))
		assert.NoError(t, writer.Close())
		postObjectOutput, err := http.Post("http://localhost:3266/default", writer.FormDataContentType(), body)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, postObjectOutput.StatusCode)
	}

	// Test HEAD Object
	{
		headObjectOutput, err := client.HeadObject(context.TODO(), &s3.HeadObjectInput{
			Bucket: aws.String("default"),
			Key:    aws.String("testPutObject.obj"),
		})
		assert.NoError(t, err)
		t.Logf("HeadObject: %#v", headObjectOutput)
	}

	// Test GET Object
	{
		getObjectOutput, err := client.GetObject(context.TODO(), &s3.GetObjectInput{
			Bucket: aws.String("default"),
			Key:    aws.String("testPutObject.obj"),
		})
		assert.NoError(t, err)
		assert.Equal(t, int64(1024*1024*10), getObjectOutput.ContentLength)
		download, err := io.ReadAll(getObjectOutput.Body)
		assert.NoError(t, err)
		assert.Equal(t, test10MBBuffer, download)

		getObjectOutput, err = client.GetObject(context.TODO(), &s3.GetObjectInput{
			Bucket: aws.String("default"),
			Key:    aws.String("testPostObject.obj"),
		})
		assert.NoError(t, err)
		assert.Equal(t, int64(1024*1024*10), getObjectOutput.ContentLength)
		download, err = io.ReadAll(getObjectOutput.Body)
		assert.NoError(t, err)
		assert.Equal(t, test10MBBuffer, download)
	}

	// Test ListObject again
	{
		listObjectsOutput, err := client.ListObjects(context.TODO(), &s3.ListObjectsInput{
			Bucket: aws.String("default"),
		})
		assert.NoError(t, err)
		assert.Equal(t, 2, len(listObjectsOutput.Contents))

		listObjectsV2Output, err := client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
			Bucket: aws.String("default"),
		})
		assert.NoError(t, err)
		assert.Equal(t, 2, len(listObjectsV2Output.Contents))
	}

	// Upload another object
	{
		_, _ = reader.Seek(0, io.SeekStart)
		putObjectOutput, err := client.PutObject(context.TODO(), &s3.PutObjectInput{
			Bucket: aws.String("default"),
			Key:    aws.String("testPutObject2.obj"),
			Body:   reader,
		})
		assert.NoError(t, err)
		t.Logf("PutObject: %#v", putObjectOutput)
	}

	time.Sleep(time.Second)

	// Test DELETE Object
	{
		deleteObjectOutput, err := client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
			Bucket: aws.String("default"),
			Key:    aws.String("testPutObject2.obj"),
		})
		assert.NoError(t, err)
		t.Logf("DeleteObject: %#v", deleteObjectOutput)
	}

	// Test DeleteObjects
	{
		deleteObjectsOutput, err := client.DeleteObjects(context.TODO(), &s3.DeleteObjectsInput{
			Bucket: aws.String("default"),
			Delete: &types.Delete{
				Objects: []types.ObjectIdentifier{
					{
						Key: aws.String("testPutObject.obj"),
					},
					{
						Key: aws.String("testPostObject.obj"),
					},
				},
			},
		})
		assert.NoError(t, err)
		t.Logf("DeleteObjects: %#v", deleteObjectsOutput)
	}
}
