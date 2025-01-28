# -----------------------------------------------------------------------------
# Stage 1: Copy OpenCV libs from your prebuilt image
# -----------------------------------------------------------------------------
# This image "ghcr.io/alverezyari/featherframe:build" is the one that already
# has OpenCV installed at /usr/local from your previous workflow.
FROM ghcr.io/alverezyari/featherframe:build AS opencv-libs

# -----------------------------------------------------------------------------
# Stage 2: Build your Go app (which depends on gocv / OpenCV)
# -----------------------------------------------------------------------------
FROM cgr.dev/chainguard/go:latest-dev AS builder-go

# Switch to root if needed to install packages (pkgconfig, git, etc.)
USER root
RUN apk update && apk add \
    pkgconfig \
    git \
    && rm -rf /var/cache/apk/*

# Enable CGO (required for gocv/OpenCV)
ENV CGO_ENABLED=1

# Copy the OpenCV libraries from the prebuilt image into this builder,
# so that the Go compiler + cgo can link against them.
COPY --from=opencv-libs /usr/local /usr/local

# Create and switch to our build directory
WORKDIR /app

# First copy go.mod / go.sum so we can cache dependency downloads
COPY go.mod go.sum ./
RUN go mod download

# Now copy the rest of your source code
COPY . .

# Build your Go executable (which should import "gocv.io/x/gocv")
RUN go build -o /tmp/feather-finder .

# -----------------------------------------------------------------------------
# Stage 3: Minimal runtime image
# -----------------------------------------------------------------------------
FROM cgr.dev/chainguard/glibc-dynamic:latest AS runtime

# Copy the OpenCV libraries from the prebuilt image
COPY --from=opencv-libs /usr/local /usr/local

# Copy your compiled Go binary from the builder
COPY --from=builder-go /tmp/feather-finder /usr/local/bin/feather-finder

# Make sure the loader can see the OpenCV .so libs
ENV LD_LIBRARY_PATH=/usr/local/lib

ENTRYPOINT ["/usr/local/bin/feather-finder"]

