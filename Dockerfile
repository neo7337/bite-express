ARG GO_VERSION=1.27

FROM golang:${GO_VERSION}-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server \
 && go build -trimpath -ldflags="-s -w" -o /out/seed   ./cmd/seed

# API Image
FROM alpine:3.20 AS api
RUN adduser -D -u 10001 app
WORKDIR /app
COPY --from=build /out/server /app/server
COPY config-*.yml /app/
RUN chmod 0644 /app/config-*.yml
USER app
EXPOSE 8082
ENTRYPOINT ["/app/server"]