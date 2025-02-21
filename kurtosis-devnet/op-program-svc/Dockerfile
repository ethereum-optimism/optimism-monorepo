ARG BASE_VERSION=latest

FROM golang:1.22.7-alpine3.20 AS builder

COPY ./*.go /app/
WORKDIR /app

RUN go mod init op-program-svc
RUN go build -o op-program-svc .


FROM op-program-base:${BASE_VERSION}
COPY --from=builder /app/op-program-svc .
EXPOSE 8080
CMD ["./op-program-svc"]
