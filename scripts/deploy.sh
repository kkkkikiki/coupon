#!/bin/bash

# Coupon Service Deployment Script
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
COMPOSE_FILE="docker-compose.yml"
SCALE_FILE="docker-compose.scale.yml"
SCALE_10_FILE="docker-compose.scale-10.yml"
PROJECT_NAME="coupon-system"

# Functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if Docker is running
check_docker() {
    if ! docker info > /dev/null 2>&1; then
        log_error "Docker is not running. Please start Docker first."
        exit 1
    fi
    log_info "Docker is running"
}

# Build and deploy standard configuration (3 instances)
deploy_standard() {
    log_info "Deploying standard configuration (3 service instances)..."
    
    # Build images
    log_info "Building Docker images..."
    docker compose -p $PROJECT_NAME build
    
    # Start services
    log_info "Starting services..."
    docker compose -p $PROJECT_NAME up -d
    
    # Wait for services to be healthy
    log_info "Waiting for services to be healthy..."
    sleep 30
    
    # Check health
    check_health
}

# Build and deploy scaled configuration (5 instances)
deploy_scaled() {
    log_info "Deploying scaled configuration (5 service instances)..."
    
    # Build images
    log_info "Building Docker images..."
    docker compose -f $COMPOSE_FILE -f $SCALE_FILE -p $PROJECT_NAME build
    
    # Start services
    log_info "Starting services..."
    docker compose -f $COMPOSE_FILE -f $SCALE_FILE -p $PROJECT_NAME up -d
    
    # Wait for services to be healthy
    log_info "Waiting for services to be healthy..."
    sleep 45
    
    # Check health
    check_health
}

# Build and deploy maximum scaled configuration (10 instances)
deploy_10() {
    log_info "Deploying maximum scaled configuration (10 service instances)..."
    
    # Build images
    log_info "Building Docker images..."
    docker compose -f $COMPOSE_FILE -f $SCALE_FILE -f $SCALE_10_FILE -p $PROJECT_NAME build
    
    # Start services
    log_info "Starting services..."
    docker compose -f $COMPOSE_FILE -f $SCALE_FILE -f $SCALE_10_FILE -p $PROJECT_NAME up -d
    
    # Wait for services to be healthy
    log_info "Waiting for services to be healthy..."
    sleep 60
    
    # Check health
    check_health
}

# Check service health
check_health() {
    log_info "Checking service health..."
    
    # Check load balancer
    if curl -f -s http://localhost/health > /dev/null; then
        log_info "✅ Load balancer is healthy"
    else
        log_error "❌ Load balancer health check failed"
        return 1
    fi
    
    # Check database health
    if curl -f -s http://localhost/health/db > /dev/null; then
        log_info "✅ Database connection is healthy"
    else
        log_error "❌ Database health check failed"
        return 1
    fi
    
    log_info "🎉 All services are healthy!"
}

# Stop all services
stop_services() {
    log_info "Stopping all services..."
    docker compose -p $PROJECT_NAME down
    log_info "Services stopped"
}

# Show service status
show_status() {
    log_info "Service Status:"
    docker compose -p $PROJECT_NAME ps
    
    echo ""
    log_info "Load Balancer Stats:"
    curl -s http://localhost/nginx_status 2>/dev/null || log_warn "Nginx status not available"
}

# Show logs
show_logs() {
    local service=${1:-}
    if [ -n "$service" ]; then
        log_info "Showing logs for $service..."
        docker compose -p $PROJECT_NAME logs -f "$service"
    else
        log_info "Showing logs for all services..."
        docker compose -p $PROJECT_NAME logs -f
    fi
}

# Load test
load_test() {
    log_info "Running basic load test..."
    
    # Check if service is running
    if ! curl -f -s http://localhost/health > /dev/null; then
        log_error "Service is not running. Deploy first."
        exit 1
    fi
    
    log_info "Creating test campaign..."
    # Add your load test logic here
    
    log_info "Load test completed"
}

# Main script logic
case "${1:-}" in
    "deploy")
        check_docker
        deploy_standard
        ;;
    "deploy-scaled")
        check_docker
        deploy_scaled
        ;;
    "deploy-10")
        check_docker
        deploy_10
        ;;
    "stop")
        stop_services
        ;;
    "status")
        show_status
        ;;
    "logs")
        show_logs "${2:-}"
        ;;
    "health")
        check_health
        ;;
    "test")
        load_test
        ;;
    *)
        echo "Usage: $0 {deploy|deploy-scaled|deploy-10|stop|status|logs [service]|health|test}"
        echo ""
        echo "Commands:"
        echo "  deploy        - Deploy standard configuration (3 instances)"
        echo "  deploy-scaled - Deploy scaled configuration (5 instances)"
        echo "  deploy-10     - Deploy maximum scaled configuration (10 instances)"
        echo "  stop          - Stop all services"
        echo "  status        - Show service status"
        echo "  logs [service]- Show logs (optionally for specific service)"
        echo "  health        - Check service health"
        echo "  test          - Run basic load test"
        exit 1
        ;;
esac
