# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git

# Copy go workspace files
COPY go.work go.work.sum ./

# Copy coordinator module
COPY coordinator ./coordinator

# Build the coordinator binary
WORKDIR /build/coordinator
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o coordinator ./cmd/coordinator

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/coordinator/coordinator .

# Copy static files for dashboard
COPY --from=builder /build/coordinator/internal/httpapi/ui ./ui

# Expose coordinator port
EXPOSE 8080

# Run the binary
CMD ["./coordinator"]
