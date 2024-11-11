VERSION 0.7

# renovate: datasource=docker depName=golang versioning=docker
ARG --global GO_VERSION=1.23-bookworm
# renovate: datasource=github-releases depName=kairos-io/kairos
ARG IMAGE_VERSION=v3.2.1
ARG --global BASE_IMAGE=quay.io/kairos/ubuntu:24.04-core-amd64-generic-${IMAGE_VERSION}-uki

enki-image:
    FROM DOCKERFILE -f Dockerfile .

    SAVE IMAGE enki-image

go-deps:
    ARG GO_VERSION
    FROM golang:$GO_VERSION
    WORKDIR /build
    COPY go.mod go.sum . # This will make the go mod download able to be cached as long as it hasnt change
    RUN go mod download
    SAVE ARTIFACT go.mod AS LOCAL go.mod
    SAVE ARTIFACT go.sum AS LOCAL go.sum

version:
    FROM +go-deps
    COPY .git ./
    RUN --no-cache echo $(git describe --always --tags --dirty) > VERSION
    RUN --no-cache echo $(git describe --always --dirty) > COMMIT
    ARG VERSION=$(cat VERSION)
    ARG COMMIT=$(cat COMMIT)
    SAVE ARTIFACT VERSION VERSION
    SAVE ARTIFACT COMMIT COMMIT

test:
    FROM +go-deps
    WORKDIR /build
    COPY . .
    ARG TEST_PATHS=./...
    ENV CGO_ENABLED=1
    # Some test require the docker sock exposed
    WITH DOCKER --load enki-image=(+enki-image)
        RUN go run github.com/onsi/ginkgo/v2/ginkgo run --label-filter "build-uki || genkey" -v --fail-fast --race --covermode=atomic --coverprofile=coverage.out --coverpkg=github.com/kairos-io/enki/... -p -r $TEST_PATHS
    END
    SAVE ARTIFACT coverage.out AS LOCAL coverage.out

build:
    FROM +go-deps
    COPY . .
    COPY +version/VERSION ./
    COPY +version/COMMIT ./
    ARG VERSION=$(cat VERSION)
    ARG COMMIT=$(cat COMMIT)
    RUN --no-cache echo "Building Version: ${VERSION} and Commit: ${COMMIT}"
    ARG LDFLAGS="-s -w -X github.com/kairos-io/enki/internal/version.VERSION=${VERSION} -X github.com/kairos-io/enki/internal/version.gitCommit=$COMMIT"
    ENV CGO_ENABLED=0
    RUN go build -o enki -ldflags "${LDFLAGS}" main.go
    SAVE ARTIFACT enki enki AS LOCAL build/enki

build-iso:
    FROM +enki-image
    ARG BASE_IMAGE
    WORKDIR /build
    RUN /enki genkey -e 7 --output /keys CIKEYS
    # Extend the default cmdline to write everything to serial first :D
    RUN /enki build-uki $BASE_IMAGE --output-dir /build/ -k /keys --output-type iso -x "console=ttyS0"
    SAVE ARTIFACT /build/*.iso enki.iso AS LOCAL build/enki.iso


test-bootable:
    FROM +go-deps
    WORKDIR /build
    RUN . /etc/os-release && echo "deb http://deb.debian.org/debian $VERSION_CODENAME-backports main contrib non-free" > /etc/apt/sources.list.d/backports.list
    RUN apt update
    RUN apt install -y qemu-system-x86 qemu-utils git swtpm && apt clean
    COPY . .
    COPY +build-iso/enki.iso enki.iso
    ARG ISO=/build/enki.iso
    ARG FIRMWARE=/usr/share/OVMF/OVMF_CODE.fd
    ARG USE_QEMU=true
    ARG MEMORY=4000
    ARG CPUS=2
    ARG CREATE_VM=true
    RUN date
    RUN go run github.com/onsi/ginkgo/v2/ginkgo run --label-filter "bootable" -v --fail-fast -r ./e2e
