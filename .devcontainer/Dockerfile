FROM golang:1.23-bookworm AS go
FROM node:22-bookworm

COPY --from=go /usr/local/go /usr/local/go
ENV PATH="/usr/local/go/bin:/go/bin:${PATH}" \
    GOPATH=/go

# Pin the watcher to a release compatible with the project's Go version.
RUN go install github.com/air-verse/air@v1.61.7
WORKDIR /workspaces/WineVault
