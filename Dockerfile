# Build stage — runs natively, cross-compiles via GOARCH
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder
ARG TARGETARCH

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOARCH=${TARGETARCH} go build -o clay-server ./cmd/server

# Runtime stage — matches target platform
FROM alpine:3.20

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /app/clay-server .
COPY --from=builder /app/templates ./templates
COPY --from=builder /app/static ./static

# Data directories — mount volumes here
RUN mkdir -p /data/uploads/thumbnails

ENV PORT=8080
ENV UPLOAD_DIR=/data/uploads

EXPOSE 8080

CMD ["./clay-server"]
