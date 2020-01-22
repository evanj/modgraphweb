FROM golang:1.13.6-buster AS builder
COPY go.* modgraphweb.go /go/src/modgraphweb/
WORKDIR /go/src/modgraphweb
RUN go build --mod=readonly modgraphweb.go && go install --mod=readonly golang.org/x/exp/cmd/modgraphviz


# Extract graphviz and dependencies
FROM golang:1.13.6-buster AS deb_extractor
RUN cd /tmp && \
    apt-get update && apt-get download \
        graphviz libgvc6 libcgraph6 libltdl7 libxdot4 libcdt5 libpathplan4 libexpat1 zlib1g && \
    mkdir /dpkg && \
    for deb in *.deb; do dpkg --extract $deb /dpkg || exit 10; done


FROM gcr.io/distroless/base-debian10:latest AS run
COPY --from=builder /go/src/modgraphweb/modgraphweb /modgraphweb
COPY --from=builder /go/bin/modgraphviz /usr/bin/modgraphviz
COPY --from=deb_extractor /dpkg /
# Configure dot plugins
RUN ["dot", "-c"]

# Use a non-root user: slightly more secure (defense in depth)
USER nobody
WORKDIR /
EXPOSE 8080
ENTRYPOINT ["/modgraphweb"]
