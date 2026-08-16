# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder
ARG TARGETARCH
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOARCH=${TARGETARCH} go build -o cardboard-server ./cmd/cardboard

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /app/cardboard-server ./
COPY --from=builder /app/cmd/cardboard/static ./static
ENV PORT=8080
ENV STATIC_DIR=/app/static
EXPOSE 8080
CMD ["./cardboard-server"]
