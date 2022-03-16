package database

import gorocksdb "github.com/SUMStudio/grocksdb"

// InitRocksdb init Options, readOptions and writeOptions
func InitRocksdb() (*gorocksdb.Options, *gorocksdb.ReadOptions, *gorocksdb.WriteOptions) {
	opts := gorocksdb.NewDefaultOptions()
	readOptions := gorocksdb.NewDefaultReadOptions()
	writeOptions := gorocksdb.NewDefaultWriteOptions()
	setRocksdbOptions(opts)
	setRocksdbReadOptions(readOptions)
	setRocksWriteOptions(writeOptions)
	return opts, readOptions, writeOptions
}

func setRocksdbOptions(opts *gorocksdb.Options) {
	opts.SetCreateIfMissing(true)
}

func setRocksdbReadOptions(readOptions *gorocksdb.ReadOptions) {

}

func setRocksWriteOptions(writeOptions *gorocksdb.WriteOptions) {

}
