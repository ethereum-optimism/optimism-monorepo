FROM us-docker.pkg.dev/oplabs-tools-artifacts/images/op-geth:optimism

RUN apk add --no-cache jq

COPY l2-op-geth-entrypoint.sh /entrypoint.sh

VOLUME ["/db"]

ENTRYPOINT ["/bin/sh", "/entrypoint.sh"]
