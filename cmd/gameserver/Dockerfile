# Builder stage
FROM golang:1.21-alpine AS builder
LABEL stage=builder

WORKDIR /app

# Copy go.mod and go.sum first to leverage Docker cache
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy the rest of the application source (respecting .dockerignore)
COPY . .

# Build the gameserver binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o /app/gameserver cmd/gameserver/gameserver.go

# Runtime stage
FROM alpine:latest
WORKDIR /app

# Copy the server binary from the builder stage
COPY --from=builder /app/gameserver /app/gameserver

# Copy the configuration file.
COPY config/server.yaml /app/config/server.yaml

# Expose ports used by gameserver
EXPOSE 50051

CMD ["/app/gameserver"]
