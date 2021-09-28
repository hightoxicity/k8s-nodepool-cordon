FROM golang:1.17.1-alpine
RUN mkdir -p /go/src/github.com/hightoxicity/k8s-nodepool-cordon
WORKDIR /go/src/github.com/hightoxicity/k8s-nodepool-cordon
COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-w -s -v -extldflags -static" -a main.go

FROM scratch
COPY --from=0 /go/src/github.com/hightoxicity/k8s-nodepool-cordon/main /k8s-nodepool-cordon
ENTRYPOINT ["/k8s-nodepool-cordon"]
CMD ["-help"]
