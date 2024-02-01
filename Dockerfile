ARG GO_VERSION=1.21-alpine3.18
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

FROM fedora as tools-image

RUN dnf install -y binutils systemd-boot mtools efitools sbsigntools shim openssl systemd-ukify dosfstools xorriso

COPY --from=builder /enki /enki

ENTRYPOINT ["/enki"]

FROM gcr.io/kaniko-project/executor:latest

COPY --from=builder /enki /enki

ENTRYPOINT ["/enki"]

CMD ["convert"]
