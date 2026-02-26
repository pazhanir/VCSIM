# Multi-stage build for custom vcsim
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Copy module files first for layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /vcsim ./cmd/vcsim

# Runtime image
FROM alpine:3.20

RUN apk add --no-cache ca-certificates curl

COPY --from=builder /vcsim /usr/local/bin/vcsim

# vSphere API port (standard vCenter HTTPS)
EXPOSE 443
# Scenario controller port
EXPOSE 8990

ENTRYPOINT ["vcsim"]
CMD ["-l", ":443", "-scenario-addr", ":8990"]
