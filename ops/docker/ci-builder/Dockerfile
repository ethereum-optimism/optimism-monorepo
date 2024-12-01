###############################################################################
#                                   BUILDX                                    #
###############################################################################

FROM --platform=linux/amd64 docker AS buildx
COPY --from=docker/buildx-bin /buildx /usr/libexec/docker/cli-plugins/docker-buildx
RUN docker buildx version


###############################################################################
#                              CI BUILDER (BASE)                              #
###############################################################################

FROM --platform=linux/amd64 debian:bullseye-slim AS base-builder

# Use bash as the shell
SHELL ["/bin/bash", "-c"]
ENV SHELL=/bin/bash
ENV BASH=/bin/bash

# Copy mise configuration
COPY ./mise.toml ./mise.toml

# Set up mise environment
ENV PATH="/root/.local/share/mise/shims:$PATH"
ENV PATH="/root/.local/bin:${PATH}"

# Install dependencies
# We do this in one mega RUN command to avoid blowing up the size of the image
ENV DEBIAN_FRONTEND=noninteractive
RUN /bin/sh -c set -eux; \
  apt-get update; \
  apt-get install -y --no-install-recommends bash curl openssh-client git build-essential ca-certificates gnupg binutils-mips-linux-gnu clang libffi-dev; \
  mkdir -p /etc/apt/keyrings; \
  curl -fsSL https://download.docker.com/linux/debian/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg; \
  chmod a+r /etc/apt/keyrings/docker.gpg; \
  echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/debian $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null; \
  apt-get update; \
  apt-get install -y docker-ce-cli; \
  curl https://mise.run | sh; \
  mise trust ./mise.toml; \
  mise install; \
  curl -fLSs https://raw.githubusercontent.com/CircleCI-Public/circleci-cli/master/install.sh | bash; \
  pip install capstone pyelftools; \
  go env -w GOMODCACHE=/go/pkg/mod; \
  go env -w GOCACHE=/root/.cache/go-build; \
  ln -s /usr/local/go/bin/gofmt /usr/local/bin/gofmt; \
  apt-get purge -y --auto-remove -o APT::AutoRemove::RecommendsImportant=false; \
  go clean -cache -modcache -testcache; \
  rm -rf /var/lib/apt/lists/*; \
  rm -rf /root/.cache/pip; \
  rm -rf /root/.cache/uv; \
  rm -rf /root/.rustup;

# Install Solidity versions
RUN echo "installing Solidity versions" && \
  svm install 0.8.25 && \
  svm install 0.8.19 && \
  svm install 0.8.15

# Install Codecov uploader
RUN echo "downloading and verifying Codecov uploader" && \
  curl https://keybase.io/codecovsecurity/pgp_keys.asc | gpg --no-default-keyring --keyring trustedkeys.gpg --import && \
  curl -Os "https://uploader.codecov.io/latest/linux/codecov" && \
  curl -Os "https://uploader.codecov.io/latest/linux/codecov.SHA256SUM" && \
  curl -Os "https://uploader.codecov.io/latest/linux/codecov.SHA256SUM.sig" && \
  gpgv codecov.SHA256SUM.sig codecov.SHA256SUM && \
  shasum -a 256 -c codecov.SHA256SUM || sha256sum -c codecov.SHA256SUM && \
  cp codecov /usr/local/bin/codecov && \
  chmod +x /usr/local/bin/codecov  && \
  rm codecov

# Copy docker buildx
COPY --from=buildx /usr/libexec/docker/cli-plugins/docker-buildx /usr/libexec/docker/cli-plugins/docker-buildx

# Set up entrypoint
ENTRYPOINT ["/bin/bash", "-c"]


###############################################################################
#                              CI BUILDER (RUST)                              #
###############################################################################

FROM base-builder as rust-builder

# Install clang & lld
RUN apt-get update && apt-get install -y clang lld

# Install nightly toolchain
RUN rustup update nightly
