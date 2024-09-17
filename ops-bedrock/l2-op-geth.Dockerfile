FROM us-docker.pkg.dev/oplabs-tools-artifacts/images/op-geth:v1.101408.1-dev.1

RUN apk add --no-cache jq

COPY l2-op-geth-entrypoint.sh /entrypoint.sh

VOLUME ["/db"]

ENTRYPOINT ["/bin/sh", "/entrypoint.sh"]
