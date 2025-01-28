# =============================================================================
# Stage 1: Builder - use your pre-built OpenCV image + a Go dev environment
# =============================================================================
# We'll start FROM your prebuilt image with OpenCV:
FROM ghcr.io/alverezyari/featherframe:build AS opencv-libs

# That image presumably has OpenCV in /usr/local
# plus compilers if you need them.

# If you need a separate Go environment, you can either:
#   1) Install Go into this same image
#   2) Or create a second stage with cgr.dev/chainguard/go:latest-dev
# 
# Let's demonstrate the second approach (multi-stage) to keep things tidy.

FROM cgr.dev/chainguard/go:latest-dev AS builder-go

USER root

ENV CGO_ENABLED=1
ENV GOCV_VERSION="v0.40.0"

# Copy the OpenCV libs from your pre-built opencv-libs image
COPY --from=opencv-libs /usr/local /usr/local

# Now install gocv (which links to the copied OpenCV libs in /usr/local)
RUN go install gocv.io/x/gocv@$GOCV_VERSION

# Build your application
WORKDIR /app
COPY . .   # copy your source
RUN go mod tidy
RUN go build -o /tmp/feather-finder .

# =============================================================================
# Stage 2: Minimal runtime
# =============================================================================
FROM cgr.dev/chainguard/glibc-dynamic:latest AS runtime

# Copy the OpenCV libraries from your opencv-libs stage
COPY --from=opencv-libs /usr/local /usr/local

# Copy your final compiled app from the builder-go stage
COPY --from=builder-go /tmp/feather-finder /usr/local/bin/feather-finder

# Let the loader find the libraries
ENV LD_LIBRARY_PATH=/usr/local/lib

ENTRYPOINT ["/usr/local/bin/feather-finder"]

