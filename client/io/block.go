package io

import (
	"context"
	"ecos/client/config"
	agent "ecos/client/info-agent"
	"ecos/edge-node/gaia"
	"ecos/edge-node/infos"
	"ecos/edge-node/object"
	"ecos/edge-node/pipeline"
	"ecos/messenger"
	messengerCommon "ecos/messenger/common"
	"ecos/utils/common"
	"ecos/utils/errno"
	"ecos/utils/logger"
	"encoding/hex"
	"errors"
	"github.com/google/uuid"
	"github.com/minio/sha256-simd"
	"github.com/twmb/murmur3"
	"hash"
	"io"
)

type BlockStatus int

const (
	READING BlockStatus = iota
	UPLOADING
	FAILED
	FINISHED
	ABORTED
)

type Block struct {
	ctx context.Context
	object.BlockInfo
	conf *config.ClientConfig

	// status describes current status of block
	status BlockStatus

	chunks []*localChunk

	// These infos are for BlockInfo
	key           string
	clusterInfo   infos.ClusterInfo
	blockCount    int
	blockHashType infos.BucketConfig_HashType
	pipes         pipeline.ClusterPipelines

	// These are for Upload and Release
	infoAgent   *agent.InfoAgent
	uploadCount int
	delFunc     func(*Block)
}

func (b *Block) Upload() error {
	switch b.conf.ConnectType {
	case config.ConnectCloud:
		//return b.uploadByCloud()
	}
	return b.uploadByEdge()
}

func (b *Block) uploadByCloud() error {
	conn, err := messenger.GetRpcConn(b.conf.CloudAddr, b.conf.CloudPort)
	if err != nil {
		logger.Errorf("Get cloud connection failed: %v", err)
		return err
	}
	client := gaia.NewGaiaClient(conn)
	stream, err := client.UploadBlockData(b.ctx)
	if err != nil {
		logger.Errorf("Upload block data failed: %v", err)
		return err
	}
	return b.uploadByStream(stream)
}

func (b *Block) uploadByEdge() error {
	nodes := b.pipes.GetBlockPGNodeID(b.PgId)
	for _, node := range nodes {
		serverInfo, err := b.infoAgent.Get(infos.InfoType_NODE_INFO, node)
		conn, err := messenger.GetRpcConnByNodeInfo(serverInfo.BaseInfo().GetNodeInfo())
		if err != nil {
			logger.Warningf("Get node info failed: %v", err)
			continue
		}
		client := gaia.NewGaiaClient(conn)
		stream, err := client.UploadBlockData(b.ctx)
		if err != nil {
			logger.Warningf("Upload block data failed: %v", err)
			continue
		}
		err = b.uploadByStream(stream)
		if err != nil {
			logger.Warningf("Upload block data failed: %v", err)
			continue
		}
		return nil
	}
	return errno.RemoteGaiaFail
}

// Upload provides a way to upload self to a given stream
func (b *Block) uploadByStream(stream gaia.Gaia_UploadBlockDataClient) error {
	// Start Upload by ControlMessage with Code BEGIN
	pipe := b.pipes.GetBlockPipeline(b.PgId)
	start := &gaia.UploadBlockRequest{
		Payload: &gaia.UploadBlockRequest_Message{
			Message: &gaia.ControlMessage{
				Code:     gaia.ControlMessage_BEGIN,
				Block:    &b.BlockInfo,
				Pipeline: pipe,
				Term:     b.clusterInfo.Term,
			},
		},
	}
	err := stream.Send(start)
	if err != nil && err != io.EOF {
		logger.Errorf("uploadBlock stream send control msg error: %v", err)
		return err
	}
	byteCount := uint64(0)
	for _, chunk := range b.chunks {
		err = stream.Send(&gaia.UploadBlockRequest{
			Payload: &gaia.UploadBlockRequest_Chunk{
				Chunk: &gaia.Chunk{Content: chunk.data[:uint64(len(chunk.data))-chunk.freeSize]},
			},
		})
		if err != nil && err != io.EOF {
			logger.Errorf("uploadBlock send chunk error: %v", err)
			return err
		}
		byteCount += uint64(len(chunk.data)) - chunk.freeSize
	}
	if byteCount != b.BlockSize {
		logger.Errorf("Incompatible size: Chunks: %v, Block: %v", byteCount, b.BlockSize)
		return errors.New("incompatible upload size")
	}
	logger.Infof("Sent %v bytes in block: %v ", byteCount, b.BlockInfo.BlockId)
	// End Upload by ControlMessage with Code EOF
	end := &gaia.UploadBlockRequest{
		Payload: &gaia.UploadBlockRequest_Message{
			Message: &gaia.ControlMessage{
				Code:     gaia.ControlMessage_EOF,
				Block:    &b.BlockInfo,
				Pipeline: pipe,
				Term:     b.clusterInfo.Term,
			},
		},
	}
	logger.Infof("PG: %v, NODE: %v", b.PgId, pipe.RaftId)
	err = stream.Send(end)
	if err != nil && err != io.EOF {
		logger.Errorf("uploadBlock send EOF error: %v", err)
		return err
	}
	_, err = stream.CloseAndRecv()
	if err != nil {
		logger.Errorf("Unable to get result form server: %v", err)
		return err
	} else {
		logger.Infof("Block upload success")
	}
	return nil
}

// genBlockHash Block.genBlockHash uses SHA256 to calc the hash value of block content
func (b *Block) genBlockHash() error {
	if b.status != UPLOADING {
		return errno.IllegalStatus
	}
	if b.blockHashType == infos.BucketConfig_OFF {
		b.BlockHash = ""
		return nil
	}
	var h hash.Hash
	switch b.blockHashType {
	case infos.BucketConfig_SHA256:
		h = sha256.New()
	case infos.BucketConfig_MURMUR3_128:
		h = murmur3.New128()
	case infos.BucketConfig_MURMUR3_32:
		h = murmur3.New32()
	}
	for _, chunk := range b.chunks {
		h.Write(chunk.data)
	}
	b.BlockHash = hex.EncodeToString(h.Sum(nil))
	return nil
}

func (b *Block) updateBlockInfo() error {
	if b.PartId == 0 { // Not Multipart Block
		b.BlockInfo.BlockSize = 0
		for _, chunk := range b.chunks {
			b.BlockInfo.BlockSize += uint64(len(chunk.data)) - chunk.freeSize
		}
	}
	err := b.genBlockHash()
	if err != nil {
		return err
	}
	if b.blockHashType != infos.BucketConfig_OFF {
		b.BlockInfo.BlockId = b.BlockInfo.BlockHash
	} else {
		b.BlockInfo.BlockId = GenBlockId()
	}
	logger.Debugf("Gen block info success: %v", &b.BlockInfo)
	b.BlockInfo.PgId = GenBlockPG(&b.BlockInfo)
	return nil
}

func (b *Block) Close() error {
	if b.PartId != 0 { // Not Multipart Block
		return nil
	}
	if len(b.chunks) == 0 {
		return nil // Temp fix by zhang
	}
	if b.status != READING {
		return errno.RepeatedClose
	}
	b.status = UPLOADING
	err := b.updateBlockInfo()
	if err != nil {
		return err
	}
	return nil
}

func (b *Block) Abort(uc *UploadClient) error {
	res, err := uc.client.DeleteBlock(context.Background(), &gaia.DeleteBlockRequest{
		BlockId:  b.BlockId,
		Pipeline: b.pipes.GetBlockPipeline(b.PgId),
		Term:     b.clusterInfo.Term,
	})
	if err != nil {
		logger.Errorf("Abort block error: %v", err)
		return err
	}
	if res.Status == messengerCommon.Result_FAIL {
		logger.Errorf("Abort block error: %v", res.Message)
		return errors.New(res.Message)
	}
	return nil
}

// GenBlockId Generates BlockId for the `i` th block of a specific object
//
// This method ensures the global unique with UUID!
//
// ONLY CALL THIS WHEN BLOCK_HASH IS NOT VALID
func GenBlockId() string {
	return uuid.New().String()
}

// These const are for PgNum calc
const (
	blockPgNum = 100
)

var (
	blockMapper = common.NewMapper(blockPgNum)
)

// GenBlockPG Generates PgId for Block
// PgId of Block depends on `BlockId` of Block
// While BlockId depends on `Block.BlockHash`
func GenBlockPG(block *object.BlockInfo) uint64 {
	return blockMapper.MapIDtoPG(block.BlockId)
}
