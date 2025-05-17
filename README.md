# FeatherFrame

This project provides a camera management application written in Go. It can run
in a TUI for setup or in headless mode for deployments.

## Running Headless

Headless mode starts the web server and camera stream without the TUI. The
configuration is read from `~/.config/featherframe/config.json`.

```
$ go run cmd/featherframe/main.go
```

To enable headless mode set `"headless": true` in the config file.

## Container Usage

A Dockerfile is not provided, but the application should work with the official
`gocv` images for ARM64. Install OpenCV and gocv in the container, copy the
source, then build:

```
FROM gocv/gocv:latest
WORKDIR /app
COPY . .
RUN go build -o featherframe cmd/featherframe/main.go
CMD ["./featherframe"]
```

Bind mount `/dev/video0` and expose the configured port.
