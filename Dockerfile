FROM golang:1.26-alpine as builder
ARG version

WORKDIR /go/src/app
ADD . /go/src/app

# RUN go get -tool github.com/cilium/ebpf/cmd/bpf2go@latest
RUN mkdir /dist
RUN  CGO_ENABLED=0 \
       go build -ldflags "-s -w -X main.version=${version}" -o /dist ./cmd/*

FROM gcr.io/distroless/base-debian10

COPY --from=builder /dist /
ENTRYPOINT ["/conductor"]
