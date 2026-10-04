# Tablekeeper stage 1 (foundation)

HTTP reservation service foundation: health, test reset/export/import,
signup/login, public restaurant list/detail. Reservation writes,
availability and atomic moves arrive with later work items; their routes
are present and return documented `not_found` until then.

## Run with Docker (no manual setup)

```sh
docker build -t tablekeeper:stage-1 .
docker run --rm -e PORT=8080 -p 8080:8080 tablekeeper:stage-1
```

The service listens on `0.0.0.0` at `$PORT` (default `8080`).
A non-default port works the same way:

```sh
docker run --rm -e PORT=9001 -p 9001:9001 tablekeeper:stage-1
curl localhost:9001/health
```

## Run from source

Requires Go 1.27+.

```sh
go test ./...
go build -o tablekeeper ./cmd/tablekeeper
PORT=8080 ./tablekeeper
```

## Smoke check

```sh
curl -s localhost:8080/health
curl -s -X POST localhost:8080/_test/reset -H 'Content-Type: application/json' -d @- <<'EOF'
{"users":[],"restaurants":[],"reservations":[]}
EOF
```
