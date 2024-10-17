FROM golang:1.23-alpine as builder
ARG version

WORKDIR /go/src/app
ADD . /go/src/app

RUN mkdir /dist
RUN  CGO_ENABLED=0 \
       go build -ldflags "-s -w -X main.version=${version}" -o /dist ./cmd/*

FROM gcr.io/distroless/base-debian10

COPY --from=builder /dist /
ENTRYPOINT ["/conductor"]
