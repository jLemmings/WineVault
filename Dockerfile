FROM node:22-bookworm-slim AS frontend-build
WORKDIR /src/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ .
RUN npm run build

FROM golang:1.23-alpine AS backend-build
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -trimpath -ldflags="-s -w" -o /out/winevault .

FROM node:22-bookworm-slim
WORKDIR /app
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl \
    && rm -rf /var/lib/apt/lists/*

COPY --from=backend-build /out/winevault /usr/local/bin/winevault
COPY backend /app/backend
COPY --from=frontend-build /src/frontend/.output /app/frontend/.output
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

WORKDIR /app/backend
EXPOSE 3000 8080

ENV HTTP_ADDR=:8080 \
    PORT=3000 \
    AUTH_COOKIE_SECURE=false

ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
