ARG GIT_COMMIT
ARG GIT_DATE

ARG CHALLENGER_VERSION
ARG KONA_VERSION

FROM --platform=$BUILDPLATFORM us-docker.pkg.dev/oplabs-tools-artifacts/images/op-challenger:$CHALLENGER_VERSION AS challenger
FROM --platform=$BUILDPLATFORM ghcr.io/anton-rs/kona/kona-fpp-asterisc:$KONA_VERSION AS kona

FROM --platform=$BUILDPLATFORM ubuntu:22.04 AS proofs-tools
RUN apt-get update && apt-get install -y --no-install-recommends musl openssl ca-certificates
COPY --from=challenger /usr/local/bin/op-challenger /usr/local/bin/
COPY --from=challenger /usr/local/bin/cannon /usr/local/bin/
ENV OP_CHALLENGER_CANNON_BIN /usr/local/bin/cannon
COPY --from=challenger /usr/local/bin/op-program /usr/local/bin/
ENV OP_CHALLENGER_CANNON_SERVER=/usr/local/bin/op-program

COPY --from=kona /kona-host /usr/local/bin/
ENV OP_CHALLENGER_ASTERISC_KONA_SERVER=/usr/local/bin/kona-host
COPY --from=kona /asterisc /usr/local/bin/
ENV OP_CHALLENGER_ASTERISC_BIN=/usr/local/bin/asterisc

CMD /usr/local/bin/op-challenger
