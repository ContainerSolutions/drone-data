FROM golang:1.5

RUN groupadd -r drone && useradd -r -g drone drone

COPY drone-test.go /go/src/github.com/containersolutions/drone-test/

RUN go get github.com/gorilla/websocket
RUN go install github.com/containersolutions/drone-test

EXPOSE 8000

USER drone
ENTRYPOINT ["/go/bin/drone-test"]
