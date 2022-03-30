package database

import gorocksdb "github.com/SUMStudio/grocksdb"

var (
	Opts      *gorocksdb.Options
	ReadOpts  *gorocksdb.ReadOptions
	WriteOpts *gorocksdb.WriteOptions
)

// InitRocksdb init Options, readOptions and writeOptions
func InitRocksdb() {
	Opts = gorocksdb.NewDefaultOptions()
	ReadOpts = gorocksdb.NewDefaultReadOptions()
	WriteOpts = gorocksdb.NewDefaultWriteOptions()
	setRocksdbOptions(Opts)
	setRocksdbReadOptions(ReadOpts)
	setRocksWriteOptions(WriteOpts)
}

func setRocksdbOptions(opts *gorocksdb.Options) {
	opts.SetCreateIfMissing(true)
	opts.SetCreateIfMissingColumnFamilies(true)
}

func setRocksdbReadOptions(readOptions *gorocksdb.ReadOptions) {

}

func setRocksWriteOptions(writeOptions *gorocksdb.WriteOptions) {

}

func init() {
	InitRocksdb()
}
