# One image, three binaries, chosen per service by `command`. Not a build target
# each: App Platform's compose support documents `build.context` and
# `build.dockerfile` and says nothing about `target`.

FROM golang:1.26.4-alpine AS build

WORKDIR /src

COPY api/go.mod api/go.sum ./
RUN go mod download

COPY api/ ./

# No cgo, so the binaries need no libc on the runtime base.
ENV CGO_ENABLED=0
RUN go build -o /out/api ./cmd/api \
 && go build -o /out/provisioner ./cmd/provisioner \
 && go build -o /out/migrate ./cmd/migrate

FROM alpine:3.22

RUN apk add --no-cache ca-certificates \
 && adduser -D -u 10001 cadence

WORKDIR /app

COPY --from=build /out/api /out/provisioner /out/migrate /app/

# cmd/migrate reads the chain off disk — MIGRATIONS_PATH, defaulting to
# `migrations` under the working directory — so this copy is load-bearing.
COPY api/migrations /app/migrations

USER cadence
