# Build a fully static binary, then ship it on a minimal base image.
FROM golang:1.21-alpine AS build

WORKDIR /src

# Cache module downloads separately from source.
COPY go.mod ./
COPY go.sum* ./
RUN go mod download

COPY . .

ARG VERSION=0.0.0-dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=0 go build \
    -trimpath \
    -ldflags "-s -w \
      -X github.com/vexarnetwork/vexpay/internal/version.Version=${VERSION} \
      -X github.com/vexarnetwork/vexpay/internal/version.Commit=${COMMIT} \
      -X github.com/vexarnetwork/vexpay/internal/version.BuildDate=${BUILD_DATE}" \
    -o /out/vexpay ./cmd/vexpay

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/vexpay /usr/local/bin/vexpay

EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/vexpay"]
