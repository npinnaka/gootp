gootp - Redis-backed OTP service

A minimal Go HTTP service that generates and validates time-limited OTP codes using Redis.

Features
- Generate a 6-digit OTP for a given userId with a 5-minute TTL.
- Validate and consume OTPs (one-time use).
- Native net/http, no frameworks.
- Redis connection via environment variables.
- Embedded Swagger UI and OpenAPI spec.
- Dockerfile and docker-compose for easy local setup.

Quick start
1) With Docker Compose (Redis only)
- Create an .env file (optional) to set Redis password and DB:
  REDIS_PASSWORD=changeme
  REDIS_DB=0
- Start Redis:
  docker compose up -d redis
- Run the app locally pointing to the compose Redis:
  export REDIS_ADDR=localhost:6379
  export REDIS_PASSWORD=${REDIS_PASSWORD:-changeme}
  export REDIS_DB=${REDIS_DB:-0}
  go run .

2) Build and run the app container
- Build:
  docker build -t gootp:latest .
- Run against local Redis (from compose or elsewhere):
  docker run --rm -p 8080:8080 \
    -e REDIS_ADDR=host.docker.internal:6379 \
    -e REDIS_PASSWORD=${REDIS_PASSWORD:-changeme} \
    -e REDIS_DB=${REDIS_DB:-0} \
    gootp:latest

3) Run everything with compose (uncomment app service)
- In docker-compose.yml, uncomment the app service block and start both:
  docker compose up -d

Configuration
- ADDR: HTTP listen address (default :8080)
- REDIS_ADDR: Redis address host:port (default localhost:6379)
- REDIS_PASSWORD: Redis password (optional)
- REDIS_DB: Redis DB index (default 0)

API
- Swagger UI: http://localhost:8080/swagger/
- Swagger JSON: http://localhost:8080/swagger.json

Endpoints
- POST /generate
  Request JSON: { "userId": "user-123" }
  Response JSON: { "userId": "user-123", "otp": "123456", "expiresInSeconds": 300 }

- POST /validate
  Request JSON: { "userId": "user-123", "otp": "123456" }
  Response JSON: { "valid": true, "message": "otp valid" }

Curl examples
- Generate OTP:
  curl -s http://localhost:8080/generate \
    -H 'Content-Type: application/json' \
    -d '{"userId":"user-123"}' | jq

- Validate OTP:
  curl -s http://localhost:8080/validate \
    -H 'Content-Type: application/json' \
    -d '{"userId":"user-123","otp":"123456"}' | jq

Development
- Requirements: Go 1.22+
- Run tests:
  go test ./...

Notes
- OTPs expire after 5 minutes and are deleted upon successful validation to prevent reuse.
- Always use a strong REDIS_PASSWORD in non-local environments.
