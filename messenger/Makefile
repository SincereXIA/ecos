GOMODCACHE=$(go env GOMODCACHE)

.PHONY:	clean object tide

clean:
	rm *.pb.go


all:
	protoc --go_out=.. --go-grpc_out=..  messenger/proto/*.proto -I messenger/proto -I ${GOMODCACHE}\go.etcd.io\etcd\raft\v3@v3.5.2 -I ${GOMODCACHE}\github.com\gogo\protobuf@v1.3.2 --go_opt=Mraftpb/raft.proto=go.etcd.io/etcd/raft/v3/raftpb

