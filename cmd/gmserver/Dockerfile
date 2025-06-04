# Builder stage
FROM golang:1.21-alpine AS builder
LABEL stage=builder

WORKDIR /app

# Copy go.mod and go.sum first to leverage Docker cache
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy the rest of the application source (respecting .dockerignore)
COPY . .

# Build the gmserver binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o /app/gmserver cmd/gmserver/gmserver.go

# Runtime stage
FROM alpine:latest
WORKDIR /app

# Copy the server binary from the builder stage
COPY --from=builder /app/gmserver /app/gmserver

# Copy the configuration file.
COPY config/server.yaml /app/config/server.yaml

# Expose ports used by gmserver
EXPOSE 8088
EXPOSE 50057

CMD ["/app/gmserver"]
