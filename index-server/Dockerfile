FROM golang:1.9.1 as builder
LABEL builder=true
ENV srcDir="/go/src/github.com/yglcode/index-server/"
RUN mkdir -p $srcDir
COPY . $srcDir/
WORKDIR $srcDir
RUN go test -v
RUN CGO_ENABLED=0 go build -a --installsuffix cgo --ldflags="-s" -o index-server

FROM busybox:latest
ENV srcDir="/go/src/github.com/yglcode/index-server"
COPY --from=builder $srcDir/index-server .
ENV port=8080
EXPOSE $port
HEALTHCHECK --interval=5s CMD echo 'STATUS||' | nc localhost $port | [ $(wc -m) -gt 6 ] || exit 1
CMD ["/index-server"]
