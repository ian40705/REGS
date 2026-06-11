FROM ubuntu:24.04

RUN apt-get update && apt-get install -y --no-install-recommends \
    bash \
    cmake \
    ninja-build \
    build-essential \
    g++ \
    clang \
    diffutils \
    coreutils \
    ca-certificates \
    dos2unix \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /workspace

COPY judge.sh /workspace/judge.sh

RUN dos2unix /workspace/judge.sh \
    && chmod +x /workspace/judge.sh

ENTRYPOINT ["bash"]