# Use official golang image as builder
FROM golang:1.23-alpine AS builder

# Set working directory
WORKDIR /app

# Copy source code
COPY . .

# Download dependencies
RUN go mod download


# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o billing-job .

# Use minimal alpine image for final container
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates tzdata

ENV TZ=Asia/Shanghai

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/billing-job .

# Run the binary
CMD ["./billing-job"]
