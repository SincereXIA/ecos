FROM golang:1.17 as builder

WORKDIR /ecos

COPY go.mod ./

RUN GOPROXY=https://goproxy.io,direct go mod download

COPY . ./
RUN GOPROXY=https://goproxy.io,direct go mod tidy

# Build
RUN CGO_ENABLED=0 GOOS=linux GO111MODULE=on go build -o ecos

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM alpine:3.15.0
WORKDIR /ecos
COPY --from=builder /ecos/ecos .

ENTRYPOINT ["./ecos"]
EXPOSE 3267