FROM node:20-alpine AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json* ./
RUN if [ -f package-lock.json ]; then npm ci; else npm install; fi
COPY frontend/ ./
RUN npm run build

FROM golang:1.25-alpine AS backend-builder
WORKDIR /app
RUN apk add --no-cache build-base
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY frontend ./frontend
COPY --from=frontend-builder /app/frontend/dist ./frontend/dist
RUN CGO_ENABLED=1 go build -o /app/bin/server ./cmd/server

FROM backend-builder AS backend-test
RUN CGO_ENABLED=1 go test ./...

FROM alpine:3.22
WORKDIR /app
RUN apk add --no-cache tzdata ca-certificates sqlite
ENV APP_CONFIG=/app/config/config.yaml
ENV TZ=Asia/Shanghai
COPY --from=backend-builder /app/bin/server /app/server
COPY --from=backend-builder /app/frontend/dist /app/frontend/dist
RUN mkdir -p /app/config /app/data
EXPOSE 8080
CMD ["/app/server"]
