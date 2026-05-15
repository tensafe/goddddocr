FROM golang:1.24-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go run ./cmd/ortfetch -goos linux -goarch amd64
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o /out/goddddocr-server ./cmd/goddddocr-server

FROM debian:bookworm-slim

RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=build /out/goddddocr-server /usr/local/bin/goddddocr-server
COPY --from=build /src/third_party/onnxruntime/linux_amd64/libonnxruntime.so /app/third_party/onnxruntime/linux_amd64/libonnxruntime.so

ENV ONNXRUNTIME_SHARED_LIBRARY_PATH=/app/third_party/onnxruntime/linux_amd64/libonnxruntime.so
EXPOSE 8088

CMD ["goddddocr-server", "-addr", ":8088"]

