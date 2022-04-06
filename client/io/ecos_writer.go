package io

import (
	"ecos/client/config"
	agent "ecos/client/info-agent"
	"ecos/edge-node/infos"
	"ecos/edge-node/object"
	"ecos/edge-node/pipeline"
	"ecos/utils/common"
	"ecos/utils/errno"
	"ecos/utils/logger"
	"ecos/utils/timestamp"
	"encoding/hex"
	"hash"
	"strconv"
)

type localChunk struct {
	data     []byte
	freeSize uint64
}

// Close sets localChunk.data to nil
func (c *localChunk) Close() error {
	c.data = nil
	return nil
}

// EcosWriter When creating an object, use EcosWriter as a Writer
// EcosWriter.Write can take []byte as input to create Chunk and Block for Object
type EcosWriter struct {
	infoAgent   *agent.InfoAgent
	clusterInfo *infos.ClusterInfo
	bucketInfo  *infos.BucketInfo
	key         string
	config      *config.ClientConfig
	Status      BlockStatus

	chunks     *common.Pool
	curChunk   *localChunk
	chunkCount int

	blocks         map[int]*Block
	curBlock       *Block
	blockCount     int
	finishedBlocks chan *Block
	blockPipes     []*pipeline.Pipeline

	writeSize uint64
	objHash   hash.Hash
	objPipes  []*pipeline.Pipeline
}

// getCurBlock ensures to return with a Block can put new Chunk in
func (w *EcosWriter) getCurBlock() *Block {
	blockSize := w.bucketInfo.Config.BlockSize
	if w.curBlock != nil && len(w.curBlock.chunks) ==
		int(blockSize/w.config.Object.ChunkSize) {
		w.commitCurBlock()
	}
	if w.curBlock == nil {
		newBlock := &Block{
			BlockInfo:   object.BlockInfo{},
			status:      0,
			chunks:      nil,
			key:         w.key,
			infoAgent:   w.infoAgent,
			clusterInfo: w.clusterInfo,
			blockCount:  w.blockCount,
			needHash:    w.bucketInfo.Config.BlockHashEnable,
			blockPipes:  w.blockPipes,
			uploadCount: 0,
			delFunc: func(self *Block) {
				// Release Chunks
				for _, chunk := range self.chunks {
					chunk.freeSize = w.config.Object.ChunkSize
					w.chunks.Release(chunk)
				}
				self.chunks = nil
				w.finishedBlocks <- self
			},
		}
		w.blockCount++
		w.curBlock = newBlock
	}
	return w.curBlock
}

func (w *EcosWriter) getUploadStream(b *Block) (*UploadClient, error) {
	idString := strconv.FormatUint(b.blockPipes[b.BlockInfo.PgId-1].RaftId[0], 10)
	serverInfo, _ := b.infoAgent.Get(infos.InfoType_NODE_INFO, idString)
	client, err := NewGaiaClient(serverInfo.BaseInfo().GetNodeInfo(), w.config)
	if err != nil {
		logger.Errorf("Unable to start Gaia Client: %v", err)
		return nil, err
	}
	err = client.NewUploadStream()
	if err != nil {
		logger.Errorf("Unable to start upload stream: %v", err)
		return nil, err
	}
	return client, nil
}

// commitCurBlock submits curBlock to GaiaServer and save it to BlockMap.
//
// curBlock shall be nil after calling this.
func (w *EcosWriter) commitCurBlock() {
	// TODO: Upload And Retries?
	go func(i int, block *Block) {
		err := block.Close()
		client, err := w.getUploadStream(block)
		if err != nil {
			logger.Errorf("Failed to get upload stream: %v", err)
			return
		}
		// TODO (xiong): if upload is failed, now writer never can be close or writer won't know, we should fix it.
		defer block.delFunc(block)
		if client.cancel != nil {
			defer client.cancel()
		}
		err = block.Upload(client.stream)
		if err != nil {
			block.status = FAILED
			return
		} else {
			_, err = client.GetUploadResult()
			if err != nil {
				block.status = FAILED
				return
			}
			block.status = FINISHED
		}
		if err != nil {
			logger.Warningf("Err while Closing Block %v: %v", i, err)
		}
	}(w.blockCount, w.curBlock)
	w.blocks[w.blockCount] = w.curBlock
	w.curBlock = nil
}

// getCurChunk ensures to return with a writable localChunk
func (w *EcosWriter) getCurChunk() (*localChunk, error) {
	if w.curChunk == nil {
		chunk, err := w.chunks.Acquire()
		if err != nil {
			return nil, err
		}
		w.curChunk = chunk.(*localChunk)
		w.chunkCount++
	}
	return w.curChunk, nil
}

// commitCurChunk will send curChunk to curBlock for further operations.
//
// curChunk shall be nil after calling this.
func (w *EcosWriter) commitCurChunk() {
	if w.curChunk == nil {
		return
	}
	block := w.getCurBlock()
	block.chunks = append(block.chunks, w.curChunk)
	w.curChunk = nil
}

// Write Takes []byte to create localChunk and Block for upload
//
// Errors:
// IllegalStatus: Write called on a closed EcosWriter
// IncompatibleSize: Written Size NOT corresponded with param
func (w *EcosWriter) Write(p []byte) (int, error) {
	if w.Status != READING {
		return 0, errno.IllegalStatus
	}
	lenP := len(p)
	offset, pending := 0, lenP
	for pending > 0 {
		chunk, err := w.getCurChunk()
		if err != nil {
			return offset, err
		}
		writeSize := copy(chunk.data[w.config.Object.ChunkSize-chunk.freeSize:], p[offset:])
		chunk.freeSize -= uint64(writeSize)
		if chunk.freeSize == 0 {
			w.commitCurChunk()
		}
		offset += writeSize
		pending -= writeSize
	}
	// Update ObjectMeta of EcosWriter.meta.ObjHash
	w.writeSize += uint64(offset)
	if w.bucketInfo.Config.ObjectHashEnable {
		w.objHash.Write(p[:offset])
	}
	if offset != lenP {
		return offset, errno.IncompatibleSize
	}
	return lenP, nil
}

func (w *EcosWriter) genMeta(objectKey string) *object.ObjectMeta {
	pgID := object.GenObjPgID(w.bucketInfo, objectKey, 10)
	meta := &object.ObjectMeta{
		ObjId:      object.GenObjectId(w.bucketInfo, objectKey),
		ObjSize:    w.writeSize,
		UpdateTime: timestamp.Now(),
		//TODO: check if need hash
		ObjHash:  hex.EncodeToString(w.objHash.Sum(nil)),
		PgId:     pgID,
		Blocks:   nil,
		Term:     w.clusterInfo.Term,
		MetaData: nil,
	}
	// w.meta.Blocks is set here. The map and Block.Close ensures the Block Status
	logger.Debugf("commit blocks num: %v", len(w.blocks))
	for i := 1; i <= len(w.blocks); i++ {
		block := w.blocks[i]
		meta.Blocks = append(meta.Blocks, &block.BlockInfo)
	}
	return meta
}

// commitMeta will set EcosWriter collect info of Block s and generate ObjectMeta.
// Then these infos will be sent to AlayaServer
func (w *EcosWriter) commitMeta() error {
	meta := w.genMeta(w.key)
	metaServerNode := w.getObjNodeByPg(meta.PgId)
	metaClient, err := NewMetaClient(metaServerNode, w.config)
	if err != nil {
		logger.Errorf("Upload Object Failed: %v", err)
		return err
	}
	result, err := metaClient.SubmitMeta(meta)
	if err != nil {
		logger.Errorf("Upload Object Failed: %v with Error %v", result, err)
		return err
	}
	logger.Infof("Upload ObjectMeta for %v: success", w.key)
	return nil
}

// Close will change EcosWriter.Status: READING -> UPLOADING -> FINISHED
//
// Close should close EcosWriter from subsequent writing to EcosWriter
// and WAIT until ALL BLOCK FINISHED uploading
//
// Call Close with a closed EcosWriter will produce errno.RepeatedClose
func (w *EcosWriter) Close() error {
	if w.Status != READING {
		return errno.RepeatedClose
	}
	w.commitCurChunk()
	w.commitCurBlock()
	w.Status = UPLOADING
	for i := 0; i < w.blockCount; i++ {
		block := <-w.finishedBlocks
		logger.Debugf("block closed: %v", block.BlockId)
	}
	err := w.commitMeta()
	if err != nil {
		return err
	}
	w.Status = FINISHED
	return nil
}

func (w *EcosWriter) getObjNodeByPg(pgID uint64) *infos.NodeInfo {
	logger.Infof("META PG: %v, NODE: %v", pgID, w.objPipes[pgID-1].RaftId)
	idString := strconv.FormatUint(w.objPipes[pgID-1].RaftId[0], 10)
	nodeInfo, _ := w.infoAgent.Get(infos.InfoType_NODE_INFO, idString)
	return nodeInfo.BaseInfo().GetNodeInfo()
}
