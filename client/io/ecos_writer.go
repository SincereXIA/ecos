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
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"hash"
	"io"
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

	// for multipart upload
	partObject bool
	partIDs    []int
}

// getCurBlock ensures to return with a Block can put new Chunk in
func (w *EcosWriter) getCurBlock() *Block {
	blockSize := w.bucketInfo.Config.BlockSize
	if w.curBlock != nil && len(w.curBlock.chunks) ==
		int(blockSize/w.config.Object.ChunkSize) {
		w.commitCurBlock()
	}
	if w.curBlock == nil {
		newBlock := w.getNewBlock()
		w.blockCount++
		w.curBlock = newBlock
	}
	return w.curBlock
}

// getUploadStream returns a new upload stream
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
	if w.curBlock == nil {
		return
	}
	// TODO: Upload And Retries?
	go w.uploadBlock(w.blockCount, w.curBlock)
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
	if w.partObject {
		return 0, errno.MethodNotAllowed
	}
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
	if w.bucketInfo.Config.ObjectHashType != infos.BucketConfig_OFF {
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
		PgId:       pgID,
		Blocks:     nil,
		Term:       w.clusterInfo.Term,
		MetaData:   nil,
	}
	if w.bucketInfo.Config.ObjectHashType != infos.BucketConfig_OFF {
		meta.ObjHash = hex.EncodeToString(w.objHash.Sum(nil))
	} else {
		meta.ObjHash = ""
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

// Abort will change EcosWriter.Status: READING -> ABORTED
//
// Abort should close EcosWriter from subsequent writing to EcosWriter
// and REMOVE blocks that had benn already uploaded
//
// Call Abort with a closed EcosWriter will produce errno.RepeatedAbort
func (w *EcosWriter) Abort() error {
	if w.Status != READING {
		return errno.RepeatedClose
	}
	w.Status = ABORTED
	for i := 0; i < w.blockCount; i++ {
		block := <-w.finishedBlocks
		logger.Debugf("block aborted: %v", block.BlockId)
		// TODO: Delete block from block server
	}
	return nil
}

/////////////////////////////////////////////////////////////////
//            EcosWriter Support for MultiPartUpload           //
/////////////////////////////////////////////////////////////////

// genPartialHash will generate partial hash from EcosWriter.blocks
// Will be used in EcosWriter.genPartialMeta
func (w *EcosWriter) genPartialHash() string {
	if w.bucketInfo.Config.ObjectHashType == infos.BucketConfig_OFF {
		return ""
	}
	w.objHash.Reset()
	for _, partID := range w.partIDs {
		block := w.blocks[partID]
		w.objHash.Write([]byte(block.BlockInfo.BlockHash))
	}
	return hex.EncodeToString(w.objHash.Sum(nil))
}

// genPartialMeta will generate partial ObjectMeta for partial upload.
func (w *EcosWriter) genPartialMeta(objectKey string) *object.ObjectMeta {
	pgID := object.GenObjPgID(w.bucketInfo, objectKey, 10)
	var blocks []*object.BlockInfo
	for _, partID := range w.partIDs {
		block := w.blocks[partID]
		blocks = append(blocks, &block.BlockInfo)
	}
	meta := &object.ObjectMeta{
		ObjId:      object.GenObjectId(w.bucketInfo, w.key),
		ObjSize:    w.writeSize,
		UpdateTime: timestamp.Now(),
		PgId:       pgID,
		Blocks:     blocks,
		Term:       w.clusterInfo.Term,
		MetaData:   nil,
	}
	meta.ObjHash = w.genPartialHash()
	// w.meta.Blocks is set here. The map and Block.Close ensures the Block Status
	logger.Debugf("commit blocks num: %v", len(blocks))
	return meta
}

// CommitPartialMeta will set EcosWriter collect info of Block s and generate ObjectMeta.
func (w *EcosWriter) CommitPartialMeta() error {
	if !w.partObject {
		return errno.MethodNotAllowed
	}
	meta := w.genPartialMeta(w.key)
	metaServerNode := w.getObjNodeByPg(meta.PgId)
	metaClient, err := NewMetaClient(metaServerNode, w.config)
	if err != nil {
		logger.Errorf("Update Multipart Object Failed: %v", err)
		return err
	}
	result, err := metaClient.SubmitMeta(meta)
	if err != nil {
		logger.Errorf("Update Multipart Object Failed: %v with Error %v", result, err)
		return err
	}
	logger.Infof("Update Multipart Object for %v: success", w.key)
	return nil
}

// addPartID will ordinal insert partID to EcosWriter.partIDs
//
// Returns:
// 		true:  if partID NOT in EcosWriter.partIDs
// 		false: if partID IS  in EcosWriter.partIDs
func (w *EcosWriter) addPartID(partID int) bool {
	for i, id := range w.partIDs {
		if id == partID {
			return false
		}
		if id < partID {
			w.partIDs = append(w.partIDs[:i], append([]int{partID}, w.partIDs[i:]...)...)
			return true
		}
	}
	w.partIDs = append(w.partIDs, partID)
	return true
}

// WritePart will write data to EcosWriter.blocks[partID]
func (w *EcosWriter) WritePart(partID int, reader io.Reader) (string, error) {
	if !w.partObject {
		return "", errno.MethodNotAllowed
	}
	if w.Status != READING {
		return "", errno.RepeatedClose
	}
	if partID < 1 || partID > 10000 {
		return "", errno.InvalidArgument
	}
	// TODO: Delete duplicate part
	noDuplicate := w.addPartID(partID)
	if !noDuplicate {
		victim := w.blocks[partID]
		logger.Tracef("Delete duplicate part %v", victim.PartId)
	}
	w.blocks[partID] = w.getNewBlock()
	w.blocks[partID].BlockInfo.PartId = int32(partID)
	w.blocks[partID].delFunc = func(self *Block) {
		self.chunks = nil
		w.finishedBlocks <- self
	}
	blockSize := uint64(0)
	for {
		var chunkData = make([]byte, w.config.Object.ChunkSize)
		readSize, err := reader.Read(chunkData)
		blockSize += uint64(readSize)
		chunk := &localChunk{
			freeSize: 0,
			data:     chunkData[:readSize],
		}
		w.blocks[partID].chunks = append(w.blocks[partID].chunks, chunk)
		w.writeSize += uint64(readSize)
		w.objHash.Write(chunkData[:readSize])
		if err != nil {
			if err == io.EOF {
				break
			}
			// TODO: Abort
			return "", err
		}
	}
	w.blocks[partID].BlockInfo.BlockSize = blockSize
	w.blocks[partID].status = UPLOADING
	err := w.blocks[partID].updateBlockInfo()
	if err != nil {
		return "", err
	}
	go w.uploadBlock(partID, w.blocks[partID])
	return w.blocks[partID].BlockId, nil
}

// CloseMultiPart will close EcosWriter for multi-part upload..
func (w *EcosWriter) CloseMultiPart(parts ...types.CompletedPart) (string, error) {
	if !w.partObject {
		return "", errno.MethodNotAllowed
	}
	if w.Status != READING {
		return "", errno.RepeatedClose
	}
	w.Status = UPLOADING
	// TODO: Check completed parts in args
	for i := 0; i < w.blockCount; i++ {
		block := <-w.finishedBlocks
		logger.Debugf("block closed: %v", block.BlockId)
	}
	err := w.CommitPartialMeta()
	if err != nil {
		return "", err
	}
	w.Status = FINISHED
	return hex.EncodeToString(w.objHash.Sum(nil)), nil
}

// AbortMultiPart will abort EcosWriter for multipart upload.
func (w *EcosWriter) AbortMultiPart() error {
	if !w.partObject {
		return errno.MethodNotAllowed
	}
	if w.Status != READING {
		return errno.RepeatedClose
	}
	w.Status = ABORTED
	w.blocks = nil
	w.partIDs = nil
	return nil
}

func (w *EcosWriter) getObjNodeByPg(pgID uint64) *infos.NodeInfo {
	logger.Infof("META PG: %v, NODE: %v", pgID, w.objPipes[pgID-1].RaftId)
	idString := strconv.FormatUint(w.objPipes[pgID-1].RaftId[0], 10)
	nodeInfo, _ := w.infoAgent.Get(infos.InfoType_NODE_INFO, idString)
	return nodeInfo.BaseInfo().GetNodeInfo()
}

// getNewBlock will get a new Block with blank BlockInfo.
func (w *EcosWriter) getNewBlock() *Block {
	return &Block{
		BlockInfo:   object.BlockInfo{},
		status:      READING,
		chunks:      nil,
		key:         w.key,
		infoAgent:   w.infoAgent,
		clusterInfo: w.clusterInfo,
		blockCount:  w.blockCount,
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
}

// uploadBlock will upload a Block to Object Storage.
func (w *EcosWriter) uploadBlock(i int, block *Block) {
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
}
