FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/campusclaw ./cmd/server

FROM alpine:3.20
RUN addgroup -S campus && adduser -S -G campus campus
WORKDIR /app
COPY --from=build /out/campusclaw /app/campusclaw
RUN mkdir -p /data/uploads && chown -R campus:campus /data
USER campus
EXPOSE 8081
ENTRYPOINT ["/app/campusclaw"]

