# DO NOT EDIT! Generated Dockerfile for test/cmd/watch-cm.
FROM registry.ci.openshift.org/openshift/release:golang-1.18 as builder

COPY . .

RUN mkdir -p /var/run/ko && \
    mkdir -p test/cmd/watch-cm/kodata && \
    go build -o /usr/bin/main ./test/cmd/watch-cm && \
    cp -r test/cmd/watch-cm/kodata /var/run/ko

FROM registry.access.redhat.com/ubi8/ubi-minimal
USER 65532

COPY --from=builder /usr/bin/main /usr/bin/main
COPY --from=builder /var/run/ko /var/run/ko
ENTRYPOINT ["/usr/bin/main"]
