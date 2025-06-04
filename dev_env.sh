#!/usr/bin/env bash

# Script to manage the local development environment using Docker Compose

# --- Configuration ---
# Services that use pre-built images from Docker Hub (for the 'pull' command)
BACKEND_SERVICES="mongo redis consul nsqlookupd nsqd"

# --- Helper Functions ---

# Function to print usage instructions
usage() {
  echo "Usage: $0 [command]"
  echo ""
  echo "Manages the local development environment using Docker Compose."
  echo ""
  echo "Commands:"
  echo "  up        (Default) Builds images if necessary and starts all services in detached mode."
  echo "  down      Stops and removes all services, including volumes defined as external."
  echo "  stop      Stops services without removing them."
  echo "  start     Starts existing stopped services."
  echo "  logs [service...] Tail logs for all services or specified service(s). e.g., '$0 logs loginserver'"
  echo "  status    Show status of services."
  echo "  pull      Pulls the latest versions of backend service images (${BACKEND_SERVICES})."
  echo "  build [service...] Builds/rebuilds images for application services or specified service(s)."
  echo "  ps        Alias for 'status'."
  echo "  help      Show this help message."
  echo ""
  echo "Prerequisites: Docker and Docker Compose (v1 or v2 plugin) must be installed."
}

# Function to check for Docker and determine Docker Compose command
COMPOSE_CMD=""
check_deps() {
    if ! command -v docker &> /dev/null; then
        echo "Error: docker command not found. Please install Docker."
        exit 1
    fi

    if docker compose version &> /dev/null; then
        COMPOSE_CMD="docker compose"
    elif command -v docker-compose &> /dev/null; then
        COMPOSE_CMD="docker-compose"
    else
        echo "Error: Neither 'docker compose' (plugin) nor 'docker-compose' (standalone) found."
        echo "Please install Docker Compose. See https://docs.docker.com/compose/install/"
        exit 1
    fi
}

# --- Main Script Logic ---

# Check dependencies first
check_deps

COMMAND="$1"
shift # Remove command from argument list, so $@ can be used for service names

case "$COMMAND" in
  up|"") # Default to 'up' if no command or 'up' is specified
    echo "Starting local development environment..."
    echo "Building images if necessary and starting services in detached mode..."
    $COMPOSE_CMD up --build -d "$@" # Pass remaining args, though 'up' doesn't usually take service names like this
    echo ""
    echo "Environment started. Use '$COMPOSE_CMD ps' or '$0 status' to check."
    echo "Use '$0 logs' or '$0 logs <service_name>' to view logs."
    ;;
  down)
    echo "Stopping and removing local development environment (containers, networks)..."
    # Add --volumes to remove named volumes if desired, but often want to keep DB data
    $COMPOSE_CMD down "$@" # Pass remaining args
    echo "Environment stopped and removed."
    ;;
  stop)
    echo "Stopping local development environment services..."
    $COMPOSE_CMD stop "$@" # Pass remaining args
    echo "Services stopped."
    ;;
  start)
    echo "Starting local development environment services..."
    $COMPOSE_CMD start "$@" # Pass remaining args
    echo "Services started."
    ;;
  logs)
    echo "Tailing logs (Ctrl+C to stop)..."
    if [ -z "$1" ]; then # No specific service given
        $COMPOSE_CMD logs -f --tail="100"
    else # Specific service(s) given
        $COMPOSE_CMD logs -f --tail="100" "$@"
    fi
    ;;
  status|ps)
    echo "Current status of services:"
    $COMPOSE_CMD ps "$@"
    ;;
  pull)
    echo "Pulling latest images for backend services (${BACKEND_SERVICES})..."
    # $COMPOSE_CMD pull $BACKEND_SERVICES # This would pull services defined in docker-compose.yaml
    # To pull *only* the backend services which are defined with an 'image:' tag:
    for service_name in $BACKEND_SERVICES; do
        echo "Pulling $service_name..."
        $COMPOSE_CMD pull "$service_name"
    done
    echo "Backend images pulled."
    ;;
  build)
    echo "Building/rebuilding application images..."
    if [ -z "$1" ]; then # No specific service given
        $COMPOSE_CMD build
    else # Specific service(s) given
        $COMPOSE_CMD build "$@"
    fi
    echo "Build process complete."
    ;;
  help|--help|-h)
    usage
    ;;
  *)
    echo "Error: Unknown command '$COMMAND'"
    usage
    exit 1
    ;;
esac

exit 0
