# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS build

RUN apk add --no-cache git

WORKDIR /src

# Cache dependencies
COPY go.mod go.sum ./
# GITHUB_TOKEN is required because go-bpmn-engine is a private module.
# Pass it at build time: docker build --secret id=github_token,env=GITHUB_TOKEN .
RUN --mount=type=secret,id=github_token \
    if [ -f /run/secrets/github_token ]; then \
      TOKEN=$(cat /run/secrets/github_token); \
      git config --global url."https://${TOKEN}@github.com/".insteadOf "https://github.com/"; \
    fi && \
    GOPRIVATE=github.com/cosmin-harangus/* go mod download

COPY . .
RUN --mount=type=secret,id=github_token \
    if [ -f /run/secrets/github_token ]; then \
      TOKEN=$(cat /run/secrets/github_token); \
      git config --global url."https://${TOKEN}@github.com/".insteadOf "https://github.com/"; \
    fi && \
    CGO_ENABLED=0 GOPRIVATE=github.com/cosmin-harangus/* \
    go build -o /bin/bpmn-server ./cmd/server

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata
COPY --from=build /bin/bpmn-server /bin/bpmn-server

EXPOSE 8080
ENTRYPOINT ["/bin/bpmn-server"]
