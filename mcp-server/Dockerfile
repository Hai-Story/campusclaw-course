FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY *.go ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/mcp-demo .
FROM alpine:3.20
RUN addgroup -S campus && adduser -S -G campus campus
COPY --from=build /out/mcp-demo /app/mcp-demo
USER campus
EXPOSE 8090
ENTRYPOINT ["/app/mcp-demo"]
