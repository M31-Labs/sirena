# Stage 1: build the sirena binary
FROM golang:1.26 AS builder

WORKDIR /src

# Copy module files first for layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the full source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /sirena ./cmd/sirena

# Stage 2: minimal runtime image
FROM alpine:3.21

RUN apk add --no-cache bash

COPY --from=builder /sirena /usr/local/bin/sirena
COPY .github/actions/bake/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
