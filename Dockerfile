FROM golang:1.27-alpine AS build
ARG VERSION=go-build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags="-s -w -X xray-status/internal/version.Version=${VERSION}" \
    -o /app/statuspage ./cmd/statuspage \
 && mkdir -p /data

# root намеренно: сам биндит 80/443 и пишет БД/обновления в /data и /app
FROM gcr.io/distroless/static-debian12
COPY --from=build /app /app
COPY --from=build /data /data
WORKDIR /app
EXPOSE 80 443 8080
ENTRYPOINT ["/app/statuspage"]
