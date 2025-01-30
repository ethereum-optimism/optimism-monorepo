ARG OP_STACK_GO_BUILDER=us-docker.pkg.dev/oplabs-tools-artifacts/images/op-stack-go:latest
FROM $OP_STACK_GO_BUILDER AS builder
# See "make golang-docker" and /ops/docker/op-stack-go

FROM alpine:3.20

COPY --from=builder /usr/local/bin/da-server /usr/local/bin/da-server

CMD ["da-server"]
