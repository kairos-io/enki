ARG LUET_VERSION=0.35.2
ARG GO_VERSION=1.22.5-alpine

FROM quay.io/luet/base:$LUET_VERSION AS luet
FROM golang:$GO_VERSION AS builder

WORKDIR /build
COPY . .

ENV CGO_ENABLED=0
RUN go mod download
# Set arg/env after go mod download, otherwise we invalidate the cached layers due to the commit changing easily
ARG ENKI_VERSION
ARG ENKI_COMMIT
ENV ENKI_VERSION=${ENKI_VERSION}
ENV ENKI_COMMIT=${ENKI_COMMIT}
RUN go build \
    -ldflags "-w -s \
    -X github.com/kairos-io/enki/internal/version.VERSION=$ENKI_VERSION \
    -X github.com/kairos-io/enki/internal/version.gitCommit=$ENKI_COMMIT" \
    -o /enki

# specify the fedora version, otherwise we migth get beta versions!
FROM fedora:40 as tools-image
COPY --from=luet /usr/bin/luet /usr/bin/luet
ENV LUET_NOLOCK=true
ENV TMPDIR=/tmp
ARG TARGETARCH
# copy both arches
COPY luet-arm64.yaml /tmp/luet-arm64.yaml
COPY luet-amd64.yaml /tmp/luet-amd64.yaml
# Set the default luet config to the current build arch
RUN mkdir -p /etc/luet/
RUN cp /tmp/luet-${TARGETARCH}.yaml /etc/luet/luet.yaml
## Uki artifacts, will be set under the /usr/kairos directory
## We can install both arches, as the artifacts are named differently
RUN luet install --config /tmp/luet-amd64.yaml -y system/systemd-boot
RUN luet install --config /tmp/luet-arm64.yaml -y system/systemd-boot

RUN dnf install -y binutils mtools efitools shim openssl dosfstools xorriso rsync

COPY --from=builder /enki /enki

ENTRYPOINT ["/enki"]