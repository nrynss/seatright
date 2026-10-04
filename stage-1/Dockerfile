# Go build produces a static binary; the runtime image only needs zoneinfo
# for IANA timezone resolution plus the binary itself.
FROM golang:1.27-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN CGO_ENABLED=0 go build -trimpath -o /out/tablekeeper ./cmd/tablekeeper

FROM debian:bookworm-slim
COPY --from=build /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=build /out/tablekeeper /app/tablekeeper
# The web build (stage-1/web) is copied to /app/web/dist by the final
# packaging item once it exists; the service runs without it and answers
# page routes with a clean JSON 404 until then.
WORKDIR /app
ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["/app/tablekeeper"]
