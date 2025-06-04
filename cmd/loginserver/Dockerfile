# Builder stage
FROM golang:1.21-alpine AS builder
LABEL stage=builder

WORKDIR /app

# Copy go.mod and go.sum first to leverage Docker cache
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy the rest of the application source (respecting .dockerignore)
COPY . .

# Build the loginserver binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o /app/loginserver cmd/loginserver/loginserver.go

# Runtime stage
FROM alpine:latest
WORKDIR /app

# Copy the server binary from the builder stage
COPY --from=builder /app/loginserver /app/loginserver

# Copy the configuration file.
# This path is relative to the build context (project root).
COPY config/server.yaml /app/config/server.yaml

# Expose ports used by loginserver (HTTP and potential RPC)
EXPOSE 8081
EXPOSE 50055

# Command to run the server
# The server binary will look for "config/server.yaml" relative to its execution path,
# but since WORKDIR is /app, it will look for /app/config/server.yaml.
# The server code uses "config/server.yaml" as a relative path.
CMD ["/app/loginserver"]
