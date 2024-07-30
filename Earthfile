VERSION 0.7

# renovate: datasource=docker depName=golang
ARG --global GO_VERSION=1.22-bookworm

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
    COPY . ./
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
    ARG LABEL_FILTER=
    ENV CGO_ENABLED=1
    # Some test require the docker sock exposed
    WITH DOCKER --load enki-image=(+enki-image)
        RUN go run github.com/onsi/ginkgo/v2/ginkgo run --label-filter "$LABEL_FILTER" -v --fail-fast --race --covermode=atomic --coverprofile=coverage.out --coverpkg=github.com/kairos-io/enki/... -p -r $TEST_PATHS
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
