package object

import (
	"context"
	"crypto/sha256"
	"ecos/client/config"
	"ecos/edge-node/moon"
	"ecos/edge-node/node"
	"ecos/edge-node/object"
	"ecos/messenger"
	timestamp "ecos/messenger/timestamppb"
	"ecos/utils/common"
	"ecos/utils/errno"
	"ecos/utils/logger"
	"encoding/hex"
	"hash"
	"io"
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
	groupInfo *node.GroupInfo
	key       string
	config    *config.ClientConfig
	Status    BlockStatus

	chunks     *common.Pool
	curChunk   *localChunk
	chunkCount int

	blocks         map[int]*Block
	curBlock       *Block
	blockCount     int
	finishedBlocks chan *Block

	meta    *object.ObjectMeta
	objHash hash.Hash
}

// getCurBlock ensures to return with a Block can put new Chunk in
func (w *EcosWriter) getCurBlock() *Block {
	if w.curBlock != nil && len(w.curBlock.chunks) ==
		int(w.config.Object.BlockSize/w.config.Object.ChunkSize) {
		w.commitCurBlock()
	}
	if w.curBlock == nil {
		newBlock := &Block{
			BlockInfo:   object.BlockInfo{},
			status:      0,
			chunks:      nil,
			key:         w.key,
			groupInfo:   w.groupInfo,
			blockCount:  w.blockCount,
			needHash:    w.config.Object.BlockHash,
			uploadCount: 0,
			delFunc: func(self *Block) {
				// Release Chunks
				for _, chunk := range self.chunks {
					w.chunks.Release(chunk)
				}
				self.chunks = nil
				w.finishedBlocks <- self
			},
		}
		w.curBlock = newBlock
	}
	w.blockCount++
	return w.curBlock
}

// commitCurBlock submits curBlock to GaiaServer and save it to BlockMap.
//
// curBlock shall be nil after calling this.
func (w *EcosWriter) commitCurBlock() {
	// TODO: Upload And Retries?
	go func(i int, block *Block) {
		err := block.Close()
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
		writeSize := minSize(int(chunk.freeSize), pending)
		chunk.data = append(chunk.data, p[offset:writeSize]...)
		chunk.freeSize -= uint64(writeSize)
		if chunk.freeSize == 0 {
			w.commitCurChunk()
		}
		offset += writeSize
		pending -= writeSize
	}
	// Update ObjectMeta of EcosWriter.meta.ObjHash
	w.meta.ObjSize += uint64(offset)
	if w.config.Object.ObjectHash {
		w.objHash.Write(p[:offset])
	}
	if offset != lenP {
		return offset, errno.IncompatibleSize
	}
	return lenP, nil
}

// commitMeta will set EcosWriter collect info of Block s and generate ObjectMeta.
// Then these infos will be sent to AlayaServer
func (w *EcosWriter) commitMeta() error {
	w.meta.ObjId = GenObjectId(w.key)
	// w.meta.ObjSize has been set in EcosWriter.Write
	w.meta.UpdateTime = timestamp.Now()
	if w.config.Object.ObjectHash {
		w.meta.ObjHash = hex.EncodeToString(w.objHash.Sum(nil))
	} else {
		w.meta.ObjHash = ""
	}
	w.meta.PgId = GenObjectPG(w.key)
	// w.meta.Blocks is set here. The map and Block.Close ensures the Block Status
	for i := 0; i < w.blockCount; i++ {
		w.meta.Blocks = append(w.meta.Blocks, &w.blocks[i].BlockInfo)
	}
	// TODO: Add PG Calc for ObjectMeta
	metaClient, err := NewMetaClient("")
	if err != nil {
		logger.Errorf("Upload Object Failed: %v", err)
		return err
	}
	result, err := metaClient.SubmitMeta(w.meta)
	if err != nil {
		logger.Errorf("Upload Object Failed: %v with Error %v", result, err)
		return err
	}
	logger.Infof("Upload ObjectMeta for %v: %v", w.key, result)
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
		select {
		case <-w.finishedBlocks:
			break
		}
	}
	// TODO: w.commitMeta()
	w.Status = FINISHED
	return nil
}

// EcosWriterFactory Generates EcosWriter with ClientConfig
type EcosWriterFactory struct {
	config    *config.ClientConfig
	groupInfo *node.GroupInfo
}

// NewEcosWriterFactory Constructor for EcosWriterFactory
//
// nodeAddr shall provide the address to get GroupInfo from
func NewEcosWriterFactory(config *config.ClientConfig, nodeAddr string, port int) *EcosWriterFactory {
	conn, err := messenger.GetRpcConn(nodeAddr, uint64(port))
	if err != nil {
		return nil
	}
	moonClient := moon.NewMoonClient(conn)
	groupInfo, err := moonClient.GetGroupInfo(context.Background(), &node.Term{Term: 0})
	if err != nil {
		return nil
	}
	// TODO: Retry?
	ret := &EcosWriterFactory{
		groupInfo: groupInfo,
		config:    config,
	}
	return ret
}

func (f *EcosWriterFactory) newLocalChunk() (io.Closer, error) {
	return &localChunk{freeSize: f.config.Object.ChunkSize}, nil
}

// GetEcosWriter provide a EcosWriter for object associated with key
func (f *EcosWriterFactory) GetEcosWriter(key string) EcosWriter {
	maxChunk := uint(f.config.UploadBuffer / f.config.Object.ChunkSize)
	chunkPool, _ := common.NewPool(f.newLocalChunk, maxChunk, int(maxChunk))
	return EcosWriter{
		groupInfo: f.groupInfo,
		key:       key,
		config:    f.config,
		Status:    READING,
		chunks:    chunkPool,
		blocks:    map[int]*Block{},
		meta:      &object.ObjectMeta{},
		objHash:   sha256.New(),
	}
}
