# Web build: Node >=26 <27, locked dependencies, local assets only.
FROM node:26-bookworm-slim AS webbuild
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Go build: Go 1.27, Keel v0.5.0 via go.mod, static binary plus probe client.
FROM golang:1.27-bookworm AS gobuild
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN CGO_ENABLED=0 go build -trimpath -o /out/tablekeeper ./cmd/tablekeeper
RUN CGO_ENABLED=0 go build -trimpath -o /out/probe ./probes/pclient

# Runtime: binary, zoneinfo for IANA zones, full web/dist, probe client.
# No build tools, no installers, no network services.
FROM debian:bookworm-slim
COPY --from=webbuild /web/dist /app/web/dist
COPY --from=gobuild /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=gobuild /out/tablekeeper /app/tablekeeper
COPY --from=gobuild /out/probe /app/probe
WORKDIR /app
ENV PORT=8080
ENV WEB_DIR=/app/web/dist
EXPOSE 8080
ENTRYPOINT ["/app/tablekeeper"]
