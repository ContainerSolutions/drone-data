FROM golang:1.5

RUN groupadd -r drone && useradd -r -g drone drone

RUN go get github.com/mattbaird/elastigo/lib
RUN go get github.com/gorilla/websocket

#Moved later to keep cache
COPY drone-data.go /go/src/github.com/containersolutions/drone-data/
RUN go install github.com/containersolutions/drone-data

EXPOSE 8081

USER drone
ENTRYPOINT ["/go/bin/drone-data", "eshost=database"]