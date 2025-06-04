# pandaparty- Microservice-Based Game Backend

`pandaparty` is a backend system for a game, built using a microservice architecture in Go. It features services for login, game logic, room management, friend lists, and payments.

## Table of Contents

- [Services Overview](#services-overview)
- [Architecture](#architecture)
  - [Communication Protocols](#communication-protocols)
  - [Service Discovery](#service-discovery)
  - [Data Stores](#data-stores)
  - [Asynchronous Messaging](#asynchronous-messaging)
- [Configuration](#configuration)
- [Getting Started](#getting-started)
  - [Prerequisites](#prerequisites)
  - [Running the Services](#running-the-services)
- [Protobuf Generation](#protobuf-generation)
- [API Documentation](#api-documentation)
- [Project Structure](#project-structure) (Brief)

## Services Overview

The backend is composed of the following microservices:

*   **`loginserver`**:
    *   Handles user authentication via HTTP/JSON API (endpoints: `/api/login`, `/api/validate_session`).
    *   Manages user sessions using Redis, with a default session expiry of 24 hours.
*   **`gameserver`**:
    *   Manages core player state (e.g., inventory, stats - future).
    *   Orchestrates game-related actions and communicates with other backend services (like `roomserver`) via RPC.
*   **`roomserver`**:
    *   Manages game rooms, including creation, joining, and game state (actual game logic to be implemented within rooms).
    *   Exposes an RPC interface for room operations (e.g., `CreateRoom`, `JoinRoom`).
*   **`friendserver`**:
    *   Manages users' friend lists and relationships.
    *   Exposes an RPC interface (e.g., `GetFriendsList`).
*   **`payserver`**:
    *   Handles payment information, transaction status, and potentially interacts with payment gateways.
    *   Exposes an RPC interface (e.g., `GetPaymentStatus`).

## Architecture

### Communication Protocols

*   **HTTP/JSON:** Used by `loginserver` for client-facing authentication and session validation.
*   **RPC (Remote Procedure Call):** Used for internal communication between microservices (e.g., `gameserver` calling `roomserver`). A custom TCP-based RPC framework with connection pooling is implemented in `infra/network/rpc.go`.
    *   **Message Framing:** The RPC framework uses a length-prefixed message framing protocol:
        *   Request: `TotalFrameLength (int32) | MethodNameLength (int32) | MethodName ([]byte) | PayloadLength (int32) | Payload ([]byte)`
        *   Response: `TotalFrameLength (int32) | ErrorLength (int32) | ErrorString ([]byte) | PayloadLength (int32) | Payload ([]byte)`
*   **Protobuf (Protocol Buffers):** Used as the primary data serialization format for RPC messages and potentially for some data storage or NSQ messages.

### Service Discovery

*   **Consul:** Services register themselves with Consul, and other services can discover their addresses and ports using Consul. This allows for dynamic scaling and resilience.

### Data Stores

*   **MongoDB:** The primary persistent data store for:
    *   Player profiles and game-related data (e.g., in `loginserver`, `gameserver`).
    *   Friendship information (in `friendserver`).
    *   Payment records (in `payserver`).
    *   Room state snapshots (potentially in `roomserver`).
*   **Redis:** Used for:
    *   Session management by `loginserver` (session tokens and associated user IDs).
    *   Caching (future).
    *   Leaderboards (future).
    *   Real-time features (future, e.g., presence).

### Asynchronous Messaging

*   **NSQ:** Integrated for asynchronous task processing and communication where immediate responses are not required (e.g., broadcasting events, logging, delayed tasks). Each service that uses NSQ will typically have its own producer or consumer logic.

## Configuration

Service configurations are managed via YAML files, primarily `config/server.yaml`.

*   **`config/server_config.go`:** Defines the Go structs that map to the YAML configuration.
*   **`config/server.yaml`:** Contains connection details for Redis, MongoDB, Consul, NSQ, and server-specific settings.
    *   **`servicerpcports`:** A key map under the `server` section that defines the RPC listening ports for each microservice. For example:
        ```yaml
        server:
          host: "localhost"
          servicerpcports:
            roomserver: 50052
            friendserver: 50053
            payserver: 50054
        ```
    *   The `loginserver` HTTP port is typically configured via `server.port`.

Refer to the comments within `config/server.yaml` for more details on specific parameters.

## Getting Started

### Prerequisites

*   Go (version 1.18 or higher recommended)
*   MongoDB instance running
*   Redis instance running
*   Consul agent running
*   NSQD instance running (if using NSQ features)
*   Protocol Buffer Compiler (`protoc`) and Go plugins (`protoc-gen-go`) for protobuf generation.

### Running the Services

Each service can be run from the root of the project using `go run`:

```bash
# To run the login server (HTTP):
go run cmd/loginserver/loginserver.go

# To run the gameserver:
go run cmd/gameserver/gameserver.go

# To run the roomserver (RPC):
go run cmd/roomserver/roomserver.go

# To run the friendserver (RPC):
go run cmd/friendserver/friendserver.go

# To run the payserver (RPC):
go run cmd/payserver/payserver.go
```

Ensure that the dependent services (MongoDB, Redis, Consul, NSQ) are running and accessible as per the configurations in `config/server.yaml`.

## Protobuf Generation

Protocol Buffers are used for defining data structures for RPC messages and potentially other uses.
To generate Go code from `.proto` files:

1.  Ensure `protoc` and `protoc-gen-go` are installed and in your PATH.
    *   `protoc-gen-go` can be installed via: `go install google.golang.org/protobuf/cmd/protoc-gen-go@latest`
2.  Run the script:
    ```bash
    bash tools/genproto.sh
    ```
    This script will find all `.proto` files in `infra/protocol/` and `infra/model/` and generate the corresponding `.pb.go` files in `infra/pb/`.

For more detailed information on the original project structure that inspired parts of this setup, refer to [docs/readme.md](docs/readme.md) (Note: This link points to a document that might have been part of an initial template; adapt as necessary if this file doesn't exist or has different content in your current project structure).

## API Documentation

Detailed API specifications:

*   **Login Server API:** See [docs/API_GUIDE.md](docs/API_GUIDE.md) for HTTP/JSON endpoints like `/api/login` and `/api/validate_session`.
*   **Internal RPC APIs:** Defined by `.proto` files in `infra/protocol/`. Each service exposing an RPC interface will have its methods and message structures defined there. Refer to the GoDoc comments in `infra/network/rpc.go` for details on the RPC framework itself.

## Project Structure (Brief)

*   **`cmd/`**: Main applications for each microservice (e.g., `cmd/loginserver/loginserver.go`).
*   **`config/`**: Configuration files (`server.yaml`) and Go structs for parsing them.
*   **`docs/`**: Project documentation, including this README and API guides.
*   **`infra/`**: Shared infrastructure components:
    *   `infra/consul/`: Consul client for service discovery.
    *   `infra/mongo/`: MongoDB client utilities.
    *   `infra/network/`: Custom RPC framework (`rpc.go`).
    *   `infra/nsq/`: NSQ producer/consumer utilities.
    *   `infra/pb/`: Generated Protocol Buffer Go files.
    *   `infra/protocol/`: `.proto` definitions for RPC messages.
    *   `infra/model/`: `.proto` definitions for data models (e.g., `player.proto`).
    *   `infra/redis/`: Redis client utilities.
*   **`internal/`**: Service-specific business logic and handlers:
    *   `internal/<servicename>/`: Contains implementation details for each service (e.g., `internal/loginserver/`, `internal/roomserver/`).
*   **`tools/`**: Utility scripts (e.g., `genproto.sh` for protobuf generation).
*   **`pkg/`**: (Optional) Shared utility libraries that are not specific to this project's infrastructure. (Currently not extensively used).

---
```sh
#!/bin/bash

# apt install -y protobuf-compiler
# protoc --version  # Ensure compiler version is 3+

# go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
#go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

docker run --name nsqd -p 4150:4150 -p 4151:4151     nsqio/nsq /nsqd     --broadcast-address=127.0.0.1     --lookupd-tcp-address=127.0.0.1:4160


```
## MongoDB surport
go install github.com/favadi/protoc-go-inject-tag@latest

This README provides a high-level overview. For more details, consult the specific documentation files and code comments within each module.
