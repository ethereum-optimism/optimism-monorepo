ARG BASE_IMAGE=op-program-base:latest

FROM golang:1.22.7-alpine3.20 AS builder

COPY ./*.go /app/
WORKDIR /app

RUN go mod init op-program-svc
RUN go build -o op-program-svc .


FROM ${BASE_IMAGE} AS svc

ARG GIT_COMMIT
ARG GIT_DATE

ARG CANNON_VERSION=v0.0.0
ARG OP_PROGRAM_VERSION=v0.0.0

ARG TARGETOS TARGETARCH

WORKDIR /app

# build cannon ahead of time
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build just \
        -d /app/op-program \
        -f /app/op-program/repro.justfile \
        GOOS="$TARGETOS" \
        GOARCH="$TARGETARCH" \
        GIT_COMMIT="$GIT_COMMIT" \
        GIT_DATE="$GIT_DATE" \
        CANNON_VERSION="$CANNON_VERSION" \
        OP_PROGRAM_VERSION="$OP_PROGRAM_VERSION" \
        cannon

COPY --from=builder /app/op-program-svc .
EXPOSE 8080
CMD ["./op-program-svc"]
