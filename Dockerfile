# STAGE 1: Build the binary
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum first to cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# STAGE 2: Run the binary
FROM alpine:latest
WORKDIR /root/

# Copy only the compiled binary from the builder
COPY --from=builder /app/main .

# (Migrations)
COPY --from=builder /app/migrations ./migrations

# Expose the port your Go app listens on
EXPOSE 8080

CMD ["./main"]