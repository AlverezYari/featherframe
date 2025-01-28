# -----------------------------------------------------------------------------
# Stage 1: Pull prebuilt OpenCV libs
# -----------------------------------------------------------------------------
FROM ghcr.io/alverezyari/featherframe:build AS opencv-libs

# -----------------------------------------------------------------------------
# Stage 2: Build Go app
# -----------------------------------------------------------------------------
FROM cgr.dev/chainguard/go:latest-dev AS builder-go

USER root
RUN apk update && apk add \
    pkgconf \
    && rm -rf /var/cache/apk/*

# Copy the OpenCV libraries from the prebuilt image
COPY --from=opencv-libs /usr/local /usr/local

ENV CGO_ENABLED=1

WORKDIR /app

# -- 1) copy module files first
COPY go.mod go.sum ./
RUN go mod download

# -- 2) copy the rest of your source code
COPY . .

# Build your main Go application
# If your main package is in the root directory (i.e., .)
RUN go build -o /tmp/feather-finder cmd/featherframe/main.go


# -----------------------------------------------------------------------------
# Stage 3: Minimal runtime
# -----------------------------------------------------------------------------
FROM cgr.dev/chainguard/glibc-dynamic:latest

# Copy OpenCV from prebuilt image
COPY --from=opencv-libs /usr/local /usr/local

# Copy the compiled binary from builder
COPY --from=builder-go /tmp/feather-finder /usr/local/bin/feather-finder

ENV LD_LIBRARY_PATH=/usr/local/lib

ENTRYPOINT ["/usr/local/bin/feather-finder"]

