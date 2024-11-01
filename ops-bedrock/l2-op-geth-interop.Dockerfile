FROM us-docker.pkg.dev/oplabs-tools-artifacts/images/op-geth:v1.101411.1-rc.3
# Note: depend on dev-release for sequencer interop message checks

RUN apk add --no-cache jq

COPY l2-op-geth-entrypoint.sh /entrypoint.sh

VOLUME ["/db"]

ENTRYPOINT ["/bin/sh", "/entrypoint.sh"]
