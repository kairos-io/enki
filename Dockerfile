ARG GO_VERSION=1.20-alpine3.18
FROM golang:$GO_VERSION AS builder

WORKDIR /build
COPY . .

ENV CGO_ENABLED=0
RUN go build -ldflags '-extldflags "-static"' -o /enki

FROM gcr.io/kaniko-project/executor:latest

COPY --from=builder /enki /enki

ENTRYPOINT ["/enki"]

CMD ["convert"]
