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

Use Docker Buildx to produce multi-architecture images. Example:

Bind mount `/dev/video0` and expose the configured port.

## Metrics

A Prometheus-compatible endpoint is available at `/metrics`. Two counters are
provided:

- `birds_seen` – counts bird detections
- `birds_captured` – counts saved captures

The values can be incremented via the `/api/birds_sceen` and
`/api/birds_captured` endpoints. These return `204 No Content` when called.

Additional generic metrics are exposed:

- `http_requests_total` – total HTTP requests served
- `uptime_seconds` – seconds since the server started

## Local Bird Data

Set your location in `config.json` using the `location` section:

```json
"location": {
  "latitude": 0,
  "longitude": 0
}
```

The endpoint `/api/local_birds` fetches recent bird recordings near your
configured GPS coordinates using the public xeno-canto API.
