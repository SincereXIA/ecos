# Reference for Amazon S3 Gateway APIs

https://github.com/minio/minio/blob/master/cmd/api-router.go

## Todo list

| Scope   | Description               | Method   | Status  | Reference                                                                                                                                                                                    |
|---------|---------------------------|----------|---------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Service | List all Buckets          | `GET`    | `DELAY` | [`ListBuckets`](https://docs.aws.amazon.com/zh_cn/AmazonS3/latest/API/API_ListBuckets.html)                                                                                                  |
| Bucket  | Create Bucket             | `PUT`    | `WAIT`  | [`CreateBucket`](https://docs.aws.amazon.com/zh_cn/AmazonS3/latest/API/API_CreateBucket.html)                                                                                                |
| Bucket  | Delete Bucket             | `DELETE` | `WAIT`  | [`DeleteBucket`](https://docs.aws.amazon.com/zh_cn/AmazonS3/latest/API/API_DeleteBucket.html)                                                                                                |
| Bucket  | List Bucket's Objects     | `GET`    | `WAIT`  | [`ListObjects`](https://docs.aws.amazon.com/zh_cn/AmazonS3/latest/API/API_ListObjects.html), [`ListObjectsV2`](https://docs.aws.amazon.com/zh_cn/AmazonS3/latest/API/API_ListObjectsV2.html) |
| Bucket  | Show Bucket Stat          | `HEAD`   | `WAIT`  | [`HeadBucket`](https://docs.aws.amazon.com/zh_cn/AmazonS3/latest/API/API_HeadBucket.html)                                                                                                    |
| Bucket  | List Multipart Uploads    | `GET`    | `WAIT`  | [`ListMultipartUploads`](https://docs.aws.amazon.com/zh_cn/AmazonS3/latest/API/API_ListMultipartUploads.html)                                                                                | 
| Object  | Create Object             | `PUT`    | `TODO`  | [`PutObject`](https://docs.aws.amazon.com/zh_cn/AmazonS3/latest/API/API_PutObject.html)                                                                                                      |
| Object  | Create Object via browser | `POST`   | `TODO`  | [`PostObject`](https://docs.aws.amazon.com/zh_cn/AmazonS3/latest/API/RESTObjectPOST.html)                                                                                                    |
| Object  | Delete Object             | `DELETE` | `WAIT`  | [`DeleteObject`](https://docs.aws.amazon.com/zh_cn/AmazonS3/latest/API/API_DeleteObject.html)                                                                                                |
| Object  | Delete Objects            | `POST`   | `WAIT`  | [`DeleteMultipleObjects`](https://docs.aws.amazon.com/zh_cn/AmazonS3/latest/API/API_DeleteObjects.html)                                                                                      |
| Object  | Get Object                | `GET`    | `TODO`  | [`GetObject`](https://docs.aws.amazon.com/zh_cn/AmazonS3/latest/API/API_GetObject.html)                                                                                                      |
| Object  | Get Object Meta           | `HEAD`   | `TODO`  | [`HeadObject`](https://docs.aws.amazon.com/zh_cn/AmazonS3/latest/API/API_HeadObject.html)                                                                                                    |
| Object  | Copy Object               | `PUT`    | `WAIT`  | [`CopyObject`](https://docs.aws.amazon.com/zh_cn/AmazonS3/latest/API/API_CopyObject.html)                                                                                                    |
| Object  | Create Multipart Upload   | `POST`   | `TODO`  | [`CreateMultipartUpload`](https://docs.aws.amazon.com/zh_cn/AmazonS3/latest/API/API_CreateMultipartUpload.html)                                                                              |
| Object  | Upload Part               | `PUT`    | `TODO`  | [`UploadPart`](https://docs.aws.amazon.com/zh_cn/AmazonS3/latest/API/API_UploadPart.html)                                                                                                    |
| Object  | Complete Multipart Upload | `POST`   | `TODO`  | [`CompleteMultipartUpload`](https://docs.aws.amazon.com/zh_cn/AmazonS3/latest/API/API_CompleteMultipartUpload.html)                                                                          |
| Object  | Abort Multipart Upload    | `DELETE` | `TODO`  | [`AbortMultipartUpload`](https://docs.aws.amazon.com/zh_cn/AmazonS3/latest/API/API_AbortMultipartUpload.html)                                                                                |
| Object  | List Parts                | `GET`    | `TODO`  | [`ListParts`](https://docs.aws.amazon.com/zh_cn/AmazonS3/latest/API/API_ListParts.html)                                                                                                      |

