# Nexlayer working build fix

This file is the authoritative, pinned build solution for this repo. Nexlayer uses it verbatim on every run and will not override it. If a future build with this fix fails, Nexlayer appends/updates it rather than regenerating.

## Fixed Dockerfile

```dockerfile
FROM mirror.gcr.io/library/node:22-alpine AS frontend-builder
WORKDIR /app-frontend
COPY web/package*.json ./
RUN npm install
COPY web/ ./
RUN npm run build

FROM mirror.gcr.io/library/golang:1.23-alpine AS backend-builder
ENV GOTOOLCHAIN=auto
WORKDIR /app-backend
COPY server/go.mod server/go.sum ./
# Fix the go.mod typo that causes toolchain mismatch while allowing automatic toolchain resolution
RUN sed -i 's/go 1.26.5/go 1.23/g' go.mod
RUN go mod download
COPY server/ ./
RUN go build -o main .

FROM mirror.gcr.io/library/alpine:3.20
WORKDIR /app
RUN apk add --no-cache ca-certificates

# Copy binary and static assets
COPY --from=backend-builder /app-backend/main .
COPY --from=frontend-builder /app-frontend/dist ./dist

# Set required runtime environment variables
ENV PORT=8080
ENV HOSTNAME=0.0.0.0

EXPOSE 8080

# Ensure binary is executable
RUN chmod +x ./main

CMD ["./main"]
```

## Fixed nexlayer.yaml

```yaml
application:
  name: cue-ai
  pods:
    - name: app
      image: "# filled by pipeline"
      path: /
      servicePorts:
        - 8080
      vars:
        PORT: "8080"
        HOSTNAME: "0.0.0.0"
        DATABASE_URL: "postgres://cueai:cueai@postgres.pod:5432/cueai?sslmode=disable"
        ALLOW_ORIGIN: "<% URL %>"
        OPENAI_API_KEY: "<% OPENAI_API_KEY %>"
        ANTHROPIC_API_KEY: "<% ANTHROPIC_API_KEY %>"
    - name: postgres
      image: mirror.gcr.io/library/postgres:16-alpine
      servicePorts:
        - 5432
      vars:
        POSTGRES_USER: "cueai"
        POSTGRES_PASSWORD: "cueai"
        POSTGRES_DB: "cueai"
```
