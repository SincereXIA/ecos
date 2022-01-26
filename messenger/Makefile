GO_OPT_FLAG=--go_opt=paths=source_relative
GRPC_OPT_FLAG=--go-grpc_opt=paths=source_relative
PROC_INC=-Iproto
TIDE_DIR=./tide

.PHONY:	clean object tide

clean:
	rm *.pb.go


object.pb.go: proto/object.proto
	protoc --go_out=. ${GO_OPT_FLAG} ${PROC_INC} proto/object.proto

tide:	proto/tide.proto object.pb.go
	protoc --go_out=${TIDE_DIR} --go-grpc_out=${TIDE_DIR} ${GO_OPT_FLAG} ${GRPC_OPT_FLAG} ${PROC_INC} \
		proto/tide.proto