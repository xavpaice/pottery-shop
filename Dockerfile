# Build stage — use xx for cross-compilation with CGO
FROM --platform=$BUILDPLATFORM tonistiigi/xx:1.6.1 AS xx

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder
COPY --from=xx / /

ARG TARGETPLATFORM
RUN apk add --no-cache clang lld && xx-apk add --no-cache gcc musl-dev

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 xx-go build -o clay-server ./cmd/server && \
    xx-verify clay-server

# Runtime stage
FROM --platform=$TARGETPLATFORM alpine:3.20

RUN apk add --no-cache ca-certificates sqlite-libs

WORKDIR /app

COPY --from=builder /app/clay-server .
COPY --from=builder /app/templates ./templates
COPY --from=builder /app/static ./static

# Data directories — mount volumes here
RUN mkdir -p /data/uploads/thumbnails

ENV PORT=8080
ENV DB_PATH=/data/clay.db
ENV UPLOAD_DIR=/data/uploads

EXPOSE 8080

CMD ["./clay-server"]
