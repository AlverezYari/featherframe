# -----------------------------------------------------------------------------
# Stage 1: Build Environment
# -----------------------------------------------------------------------------
FROM cgr.dev/chainguard/go:latest AS builder

# Install packages needed to build OpenCV
# This is fairly minimal, but might still be large. 
RUN apk add --no-cache \
    cmake \
    make \
    g++ \
    pkgconfig \
    git \
    libjpeg-turbo-dev \
    libpng-dev \
    zlib-dev

# Set Go env variables if needed
ENV CGO_ENABLED=1

WORKDIR /tmp

# Clone a specific tag of opencv and opencv_contrib if needed
RUN git clone --depth 1 --branch 4.7.0 https://github.com/opencv/opencv.git
RUN git clone --depth 1 --branch 4.7.0 https://github.com/opencv/opencv_contrib.git

# Build and install OpenCV
RUN mkdir -p /tmp/opencv/build && cd /tmp/opencv/build && \
    cmake \
      -D CMAKE_BUILD_TYPE=Release \
      -D BUILD_SHARED_LIBS=ON \
      -D OPENCV_EXTRA_MODULES_PATH=/tmp/opencv_contrib/modules \
      -D CMAKE_INSTALL_PREFIX=/usr/local \
      .. && \
    make -j$(nproc) && \
    make install

# GoCV version
ENV GOCV_VERSION="v0.40.0"

# Download gocv and build it (which will compile against installed OpenCV)
RUN go install gocv.io/x/gocv@$GOCV_VERSION

# Now copy your own app into the builder and build it
WORKDIR /app
COPY . .  # copy our app code into the container
RUN go mod tidy
RUN go build -o /tmp/out/feather-finder .

# -----------------------------------------------------------------------------
# Stage 2: Minimal Runtime
# -----------------------------------------------------------------------------
FROM cgr.dev/chainguard/glibc-dynamic AS runtime

# Copy over the needed OpenCV libs from builder stage
COPY --from=builder /usr/local/lib /usr/local/lib
COPY --from=builder /usr/local/bin /usr/local/bin
# If we install any libs under /usr/local/lib64 or other locations, copy those here.
# COPY --from=builder /usr/local/lib64 /usr/local/lib64
# Copy over our compiled app
COPY --from=builder /tmp/out/feather-finder /usr/local/bin/feather-finder

# Set the library path
ENV LD_LIBRARY_PATH=/usr/local/lib

# Expose any ports if needed (for a web server, metrics, etc.)
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/feather-finder"]

