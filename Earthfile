VERSION 0.7

# renovate: datasource=docker depName=golang
ARG --global GO_VERSION=1.21-alpine3.18

enki-image:
    FROM DOCKERFILE -f e2e/assets/Dockerfile.enki e2e/assets/

    SAVE IMAGE enki-image

test:
    FROM golang:$GO_VERSION
    RUN apk add rsync gcc musl-dev docker jq
    WORKDIR /build
    COPY go.mod go.sum . # This will make the go mod download able to be cached as long as it hasnt change
    RUN go mod download
    COPY . .
    ARG TEST_PATHS=./...
    ARG LABEL_FILTER=
    ENV CGO_ENABLED=1
    # Some test require the docker sock exposed
    WITH DOCKER --load enki-image=(+enki-image)
        RUN go run github.com/onsi/ginkgo/v2/ginkgo run --label-filter "$LABEL_FILTER" -v --fail-fast --race --covermode=atomic --coverprofile=coverage.out --coverpkg=github.com/kairos-io/enki/... -p -r $TEST_PATHS
    END
    SAVE ARTIFACT coverage.out AS LOCAL coverage.out
