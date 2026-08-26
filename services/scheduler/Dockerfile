# Multi-stage: scheduler is pure Go (no cgo) — tiny build.
FROM golang:1.22-bookworm AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/scheduler ./cmd/scheduler

FROM debian:bookworm-slim
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates \
 && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/scheduler /usr/local/bin/scheduler

ENV API_URL=http://api:8080
CMD ["/usr/local/bin/scheduler"]
