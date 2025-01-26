# ============================================================================
# Stage 1: Build OpenCV
# ============================================================================
FROM cgr.dev/chainguard/gcc-glibc:latest AS builder-opencv



# The Chainguard c++ image has a Wolfi-based environment with clang, make, cmake, etc.
# Wolfi uses "apk" but it's not the Alpine version; it’s "apk-like" for Wolfi. 
# Check the docs: https://github.com/chainguard-images/images/tree/main/images/cplusplus
# The package manager might be "apk" or "wolfi-apk", depending on the image version.
# Confirm which packages exist in Wolfi if you need additional dev libs.

WORKDIR /tmp

# (1) Install additional packages to build OpenCV
RUN apk update && apk add \
    git \
    libjpeg-turbo-dev \
    libpng-dev \
    zlib-dev \
    # Possibly more if needed: e.g., tiff-dev, ffmpeg-dev, etc.
    && rm -rf /var/cache/apk/* 

# (2) Clone OpenCV (v4.7.0) and optionally opencv_contrib if needed
RUN git clone --depth 1 --branch 4.7.0 https://github.com/opencv/opencv.git
RUN git clone --depth 1 --branch 4.7.0 https://github.com/opencv/opencv_contrib.git 

# (3) Build OpenCV
RUN mkdir -p /tmp/opencv/build && cd /tmp/opencv/build && \
    cmake \
      -D CMAKE_BUILD_TYPE=Release \
      -D OPENCV_EXTRA_MODULES_PATH=/tmp/opencv_contrib/modules \
      -D BUILD_SHARED_LIBS=ON \
      -D CMAKE_INSTALL_PREFIX=/usr/local \
      ../ && \
    make -j$(nproc) && \
    make install 

# ============================================================================
# Stage 2: Build Go Application (using gocv)
# ============================================================================
FROM cgr.dev/chainguard/go:1.20 AS builder-go

# We'll need the OpenCV libraries and headers from stage 1
# so we can build GoCV. We'll copy the entire /usr/local from builder-opencv
COPY --from=builder-opencv /usr/local /usr/local

# If you need pkg-config or other dev tools in the Go build stage:
RUN apk update && apk add pkgconfig && rm -rf /var/cache/apk/*

# We specify CGO_ENABLED so that cgo is used
ENV CGO_ENABLED=1
ENV GOCV_VERSION="v0.40.0"

# (1) Install gocv so it compiles and links against the newly copied OpenCV.
#     This effectively compiles the Go wrappers for OpenCV.
RUN go install gocv.io/x/gocv@$GOCV_VERSION

# (2) Build your own Go app
WORKDIR /app
COPY . .  # copy your application code
RUN go mod tidy
RUN go build -o /tmp/feather-finder .

# ============================================================================
# Stage 3: Minimal Runtime
# ============================================================================
FROM cgr.dev/chainguard/glibc-dynamic:latest AS runtime

# Copy the OpenCV shared libraries from the builder-opencv stage
COPY --from=builder-opencv /usr/local /usr/local

# Copy your final compiled app from builder-go
COPY --from=builder-go /tmp/feather-finder /usr/local/bin/feather-finder

# Make sure libraries can be found at runtime
ENV LD_LIBRARY_PATH=/usr/local/lib

ENTRYPOINT ["/usr/local/bin/feather-finder"]

