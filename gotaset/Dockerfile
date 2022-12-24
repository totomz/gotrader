FROM scratch
ARG TARGETOS
ARG TARGETARCH

COPY ../bin/gotaset_${TARGETOS}_${TARGETARCH} /opt/gotaset
WORKDIR /data
VOLUME /data

ENTRYPOINT ["/opt/gotaset"]
#linux/amd64, linux/arm64, or darwin/amd64