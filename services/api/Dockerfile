# Multi-stage: build the cgo SQLite driver against Debian glibc, run on slim.
FROM golang:1.22-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# mattn/go-sqlite3 needs FTS5 enabled via build tag (see ADR-0017).
RUN CGO_ENABLED=1 go build -tags sqlite_fts5 -o /out/api ./cmd/api

FROM debian:bookworm-slim
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates sqlite3 \
 && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/api /usr/local/bin/api

ENV DB_PATH=/data/personal-os.db
VOLUME ["/data"]
EXPOSE 8080
CMD ["/usr/local/bin/api"]
