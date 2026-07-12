# syntax=docker/dockerfile:1

# Shared build stage: compiles both binaries as static executables.
FROM golang:1.25 AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /out/data-hub . \
    && CGO_ENABLED=0 go build -o /out/simulator ./cmd/simulator

# data-hub service image.
FROM gcr.io/distroless/static-debian12 AS data-hub
COPY --from=build /out/data-hub /data-hub
ENTRYPOINT ["/data-hub"]

# robot simulator image.
FROM gcr.io/distroless/static-debian12 AS simulator
COPY --from=build /out/simulator /simulator
ENTRYPOINT ["/simulator"]
