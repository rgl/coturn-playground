FROM golang:1.17.1-bullseye AS build
WORKDIR /build
COPY go.* ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 go build -ldflags="-s"

FROM debian:bullseye-slim
COPY --from=build /build/turn-ping /app/
ENTRYPOINT ["/app/turn-ping"]
