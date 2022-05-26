package io

import (
	"context"
	"ecos/edge-node/alaya"
	"ecos/edge-node/infos"
	"ecos/edge-node/object"
	"ecos/edge-node/pipeline"
	"ecos/edge-node/watcher"
	"ecos/utils/common"
	"ecos/utils/errno"
	"ecos/utils/logger"
	"ecos/utils/timestamp"
	"encoding/hex"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/rcrowley/go-metrics"
	"hash"
	"io"
	"strings"
	"time"
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
	ctx context.Context

	f *EcosIOFactory

	key    string
	Status BlockStatus

	chunks        *common.Pool
	curChunk      *localChunk
	reserveChunks []*localChunk
	chunkCount    int

	clusterInfo    infos.ClusterInfo
	pipes          pipeline.ClusterPipelines
	blocks         map[int]*Block
	curBlock       *Block
	blockCount     int
	finishedBlocks chan *Block

	writeSize uint64
	objHash   hash.Hash

	// for multipart upload
	partObject bool
	partIDs    []int32

	// For Go Metrics
	startTime time.Time
}

// getCurBlock ensures to return with a Block can put new Chunk in
func (w *EcosWriter) getCurBlock() *Block {
	blockSize := w.f.bucketInfo.Config.BlockSize
	if w.curBlock != nil && len(w.curBlock.chunks) ==
		int(blockSize/w.f.config.Object.ChunkSize) {
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
	nodes := w.pipes.GetBlockPGNodeID(b.PgId)
	serverInfo, _ := b.infoAgent.Get(infos.InfoType_NODE_INFO, nodes[0])
	client, err := NewGaiaClient(serverInfo.BaseInfo().GetNodeInfo(), w.f.config)
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
	for _, chunk := range w.reserveChunks {
		w.chunks.Release(chunk)
	}
	w.reserveChunks = nil
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
		if len(w.reserveChunks) == 0 {
			chunks, err := w.chunks.AcquireMultiple(int(w.f.bucketInfo.Config.BlockSize / w.f.config.Object.ChunkSize))
			if err != nil {
				return nil, err
			}
			for _, chunk := range chunks {
				w.reserveChunks = append(w.reserveChunks, chunk.(*localChunk))
			}
		}
		w.curChunk = w.reserveChunks[0]
		w.chunkCount++
		if len(w.reserveChunks) > 1 {
			w.reserveChunks = w.reserveChunks[1:]
		} else {
			w.reserveChunks = nil
		}
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
	if len(block.chunks) == int(w.f.bucketInfo.Config.BlockSize/w.f.config.Object.ChunkSize) {
		w.commitCurBlock()
	}
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
		writeSize := copy(chunk.data[w.f.config.Object.ChunkSize-chunk.freeSize:], p[offset:])
		chunk.freeSize -= uint64(writeSize)
		if chunk.freeSize == 0 {
			w.commitCurChunk()
		}
		offset += writeSize
		pending -= writeSize
	}
	// Update ObjectMeta of EcosWriter.meta.ObjHash
	w.writeSize += uint64(offset)
	if w.f.bucketInfo.Config.ObjectHashType != infos.BucketConfig_OFF {
		w.objHash.Write(p[:offset])
	}
	if offset != lenP {
		return offset, errno.IncompatibleSize
	}
	return lenP, nil
}

func (w *EcosWriter) genMeta(objectKey string) *object.ObjectMeta {
	pgID := object.GenObjPgID(w.f.bucketInfo, objectKey, 10)
	meta := &object.ObjectMeta{
		ObjId:      object.GenObjectId(w.f.bucketInfo, objectKey),
		ObjSize:    w.writeSize,
		UpdateTime: timestamp.Now(),
		PgId:       pgID,
		Blocks:     nil,
		Term:       w.clusterInfo.Term,
		MetaData:   nil,
	}
	if w.f.bucketInfo.Config.ObjectHashType != infos.BucketConfig_OFF {
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
	return w.uploadMeta(meta)
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
	metrics.GetOrRegisterTimer(watcher.MetricsClientPutTimer, nil).UpdateSince(w.startTime)
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
		// Abort Blocks
		go w.abortBlock(block)
	}
	return nil
}

// Copy will create from src meta to dst
func (w *EcosWriter) Copy(meta *object.ObjectMeta) (*string, error) {
	pgID := object.GenObjPgID(w.f.bucketInfo, w.key, 10)
	meta.ObjId = object.GenObjectId(w.f.bucketInfo, w.key)
	meta.PgId = pgID
	err := w.uploadMeta(meta)
	if err != nil {
		return nil, err
	}
	return &meta.ObjId, nil
}

/////////////////////////////////////////////////////////////////
//            EcosWriter Support for MultiPartUpload           //
/////////////////////////////////////////////////////////////////

// genPartialHash will generate partial hash from EcosWriter.blocks
// Will be used in EcosWriter.genPartialMeta
func (w *EcosWriter) genPartialHash() string {
	if w.f.bucketInfo.Config.ObjectHashType == infos.BucketConfig_OFF {
		return ""
	}
	w.objHash.Reset()
	for _, partID := range w.partIDs {
		block := w.blocks[int(partID)]
		w.objHash.Write([]byte(block.BlockInfo.BlockHash))
	}
	return hex.EncodeToString(w.objHash.Sum(nil))
}

// genPartialMeta will generate partial ObjectMeta for partial upload.
func (w *EcosWriter) genPartialMeta(objectKey string) *object.ObjectMeta {
	pgID := object.GenObjPgID(w.f.bucketInfo, objectKey, 10)
	var blocks []*object.BlockInfo
	objSize := uint64(0)
	for _, partID := range w.partIDs {
		block := w.blocks[int(partID)]
		blocks = append(blocks, &block.BlockInfo)
		objSize += block.BlockInfo.BlockSize
	}
	meta := &object.ObjectMeta{
		ObjId:      object.GenObjectId(w.f.bucketInfo, w.key),
		ObjSize:    objSize,
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
retry:
	meta := w.genPartialMeta(w.key)
	metaServerNode := w.getObjNodeByPg(meta.PgId)
	ctx, _ := alaya.SetTermToContext(w.ctx, w.f.infoAgent.GetCurClusterInfo().Term)
	metaClient, err := NewMetaClient(ctx, metaServerNode, w.f.config)
	if err != nil {
		logger.Errorf("Update Multipart Object Failed: %v", err)
		return err
	}
	result, err := metaClient.SubmitMeta(meta)
	if err != nil {
		if strings.Contains(err.Error(), errno.TermNotMatch.Error()) {
			logger.Warningf("Term not match, retry")
			err = w.f.infoAgent.UpdateCurClusterInfo()
			if err != nil {
				return err
			}
			goto retry
		}
		logger.Errorf("Update Multipart Object Failed: %v with Error %v", result, err)
		return err
	}
	logger.Infof("Update Multipart Object for %v: success", w.key)
	return nil
}

// addPartID will ordinal insert partID to EcosWriter.partIDs to form an increasing order
//
// Returns:
// 		true:  if partID NOT in EcosWriter.partIDs
// 		false: if partID IS  in EcosWriter.partIDs
func (w *EcosWriter) addPartID(partID int32) bool {
	for i, id := range w.partIDs {
		if partID == id {
			return false
		}
		if partID < id {
			w.partIDs = append(w.partIDs[:i], append([]int32{partID}, w.partIDs[i:]...)...)
			return true
		}
	}
	w.partIDs = append(w.partIDs, partID)
	return true
}

// WritePart will write data to EcosWriter.blocks[partID]
func (w *EcosWriter) WritePart(partID int32, reader io.Reader) (string, error) {
	if !w.partObject {
		return "", errno.MethodNotAllowed
	}
	if w.Status != READING {
		return "", errno.RepeatedClose
	}
	if partID < 1 || partID > 10000 {
		return "", errno.InvalidArgument
	}
	noDuplicate := w.addPartID(partID)
	if !noDuplicate {
		victim := w.blocks[int(partID)]
		logger.Tracef("Delete duplicate part %v", victim.PartId)
		node, err := w.f.infoAgent.Get(infos.InfoType_NODE_INFO, w.pipes.GetBlockPGNodeID(victim.PgId)[0])
		if err != nil {
			return "", err
		}
		client, err := NewGaiaClient(node.BaseInfo().GetNodeInfo(), w.f.config)
		if err != nil {
			return "", err
		}
		err = victim.Abort(client)
		if err != nil {
			return "", err
		}
		go w.abortBlock(victim)
		delete(w.blocks, int(partID))
	}
	w.blocks[int(partID)] = w.getNewBlock()
	w.blocks[int(partID)].blockCount = int(partID)
	w.blocks[int(partID)].BlockInfo.PartId = partID
	w.blocks[int(partID)].delFunc = func(self *Block) {
		self.chunks = nil
		w.finishedBlocks <- self
	}
	blockSize := uint64(0)
	for {
		var chunkData = make([]byte, w.f.config.Object.ChunkSize)
		readSize, err := reader.Read(chunkData)
		blockSize += uint64(readSize)
		chunk := &localChunk{
			freeSize: 0,
			data:     chunkData[:readSize],
		}
		w.blocks[int(partID)].chunks = append(w.blocks[int(partID)].chunks, chunk)
		w.objHash.Write(chunkData[:readSize])
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}
	}
	w.blocks[int(partID)].BlockInfo.BlockSize = blockSize
	w.blocks[int(partID)].status = UPLOADING
	err := w.blocks[int(partID)].updateBlockInfo()
	if err != nil {
		return "", err
	}
	go w.uploadBlock(int(partID), w.blocks[int(partID)])
	metrics.GetOrRegisterTimer(watcher.MetricsClientPartPutTimer, nil).UpdateSince(w.startTime)
	return w.blocks[int(partID)].BlockId, nil
}

// ListParts will return EcosWriter.partIDs
func (w *EcosWriter) ListParts() []types.Part {
	parts := make([]types.Part, 0, len(w.partIDs))
	for _, partID := range w.partIDs {
		block := w.blocks[int(partID)]
		part := types.Part{
			ETag:       &block.BlockId,
			PartNumber: partID,
			Size:       int64(block.BlockSize),
		}
		parts = append(parts, part)
	}
	return parts
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
	pendingMap := make(map[int32]bool)
	for _, part := range parts {
		if part.PartNumber < 1 || part.PartNumber > 10000 {
			return "", errno.InvalidArgument
		}
		if _, ok := w.blocks[int(part.PartNumber)]; !ok {
			return "", errno.InvalidPart
		}
		pendingMap[part.PartNumber] = true
	}
	pending := len(parts)
	for pending > 0 {
		block := <-w.finishedBlocks
		logger.Debugf("block closed: %v", block.BlockId)
		if w.blocks[int(block.PartId)] == block && pendingMap[block.PartId] {
			pending--
			delete(pendingMap, block.PartId)
		}
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
	var err error
	errBlocks := make([]int32, 0, len(w.partIDs))
	for _, partID := range w.partIDs {
		block := w.blocks[int(partID)]
		node, err1 := w.f.infoAgent.Get(infos.InfoType_NODE_INFO, w.pipes.GetBlockPGNodeID(block.PgId)[0])
		if err1 != nil {
			err = err1
			errBlocks = append(errBlocks, partID)
			continue
		}
		client, err2 := NewGaiaClient(node.BaseInfo().GetNodeInfo(), w.f.config)
		if err2 != nil {
			err = err2
			errBlocks = append(errBlocks, partID)
			continue
		}
		err3 := block.Abort(client)
		if err3 != nil {
			err = err3
			errBlocks = append(errBlocks, partID)
			continue
		}
		block.status = ABORTED
	}
	w.partIDs = errBlocks
	if err == nil {
		w.Status = ABORTED
	}
	return err
}

// GetPartBlockInfo will get BlockInfo of a part.
func (w *EcosWriter) GetPartBlockInfo(partID int32) (*object.BlockInfo, error) {
	if !w.partObject {
		return nil, errno.MethodNotAllowed
	}
	if w.Status != READING {
		return nil, errno.MethodNotAllowed
	}
	if partID < 0 || partID >= int32(len(w.partIDs)) {
		return nil, errno.InvalidArgument
	}
	targetBlock := w.blocks[int(partID)]
	if targetBlock == nil {
		return nil, errno.InvalidPart
	}
	if targetBlock.status != FINISHED {
		return nil, errno.InvalidObjectState
	}
	return &targetBlock.BlockInfo, nil
}

func (w *EcosWriter) getObjNodeByPg(pgID uint64) *infos.NodeInfo {
	nodeId := w.pipes.GetMetaPGNodeID(pgID)[0]
	logger.Infof("META PG: %v, NODE: %v", pgID, nodeId)
	nodeInfo, _ := w.f.infoAgent.Get(infos.InfoType_NODE_INFO, nodeId)
	return nodeInfo.BaseInfo().GetNodeInfo()
}

// getNewBlock will get a new Block with blank BlockInfo.
func (w *EcosWriter) getNewBlock() *Block {
	return &Block{
		BlockInfo:   object.BlockInfo{},
		status:      READING,
		chunks:      nil,
		key:         w.key,
		infoAgent:   w.f.infoAgent,
		clusterInfo: w.clusterInfo,
		blockCount:  w.blockCount,
		pipes:       w.pipes,
		uploadCount: 0,
		delFunc: func(self *Block) {
			// Release Chunks
			for _, chunk := range self.chunks {
				chunk.freeSize = w.f.config.Object.ChunkSize
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
	if err != nil {
		logger.Warningf("block close error: %v", err)
		return
	}
	client, err := w.getUploadStream(block)
	if err != nil {
		logger.Warningf("Failed to get upload stream: %v", err)
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

// uploadMeta will upload meta info of an object.
func (w *EcosWriter) uploadMeta(meta *object.ObjectMeta) error {
retry:
	metaServerNode := w.getObjNodeByPg(meta.PgId)
	ctx, _ := alaya.SetTermToContext(w.ctx, w.f.infoAgent.GetCurClusterInfo().Term)
	metaClient, err := NewMetaClient(ctx, metaServerNode, w.f.config)
	if err != nil {
		logger.Errorf("Upload Object Failed: %v", err)
		return err
	}
	result, err := metaClient.SubmitMeta(meta)
	if err != nil {
		if strings.Contains(err.Error(), errno.TermNotMatch.Error()) {
			logger.Warningf("Term not match, retry")
			err = w.f.infoAgent.UpdateCurClusterInfo()
			if err != nil {
				return err
			}
			goto retry
		}
		logger.Errorf("Upload Object Failed: %v with Error %v", result, err)
		return err
	}
	logger.Infof("Upload ObjectMeta for %v: success", w.key)
	return nil
}

func (w *EcosWriter) abortBlock(block *Block) {
	node, err := w.f.infoAgent.Get(infos.InfoType_NODE_INFO, w.pipes.GetBlockPGNodeID(block.PgId)[0])
	if err != nil {
		logger.Warningf("Abort Block: get node info failed: %v", err)
	}
	client, err := NewGaiaClient(node.BaseInfo().GetNodeInfo(), w.f.config)
	if err != nil {
		logger.Warningf("Abort Block: get node info failed: %v", err)
	}
	abortResult := make(chan error)
	go func() {
		abortResult <- block.Abort(client)
	}()
	select {
	case err := <-abortResult:
		if err != nil {
			logger.Warningf("Abort Block: %v", err)
		}
	case <-time.After(time.Second * 10):
		logger.Warningf("Abort Block: %v", errno.BlockOperationTimeout)
	}
}
