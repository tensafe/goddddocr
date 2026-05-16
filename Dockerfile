FROM golang:1.24-bookworm AS build

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN targetos="${TARGETOS:-linux}" \
  && targetarch="${TARGETARCH:-$(go env GOARCH)}" \
  && go run ./cmd/ortfetch -goos "$targetos" -goarch "$targetarch"
RUN targetos="${TARGETOS:-linux}" \
  && targetarch="${TARGETARCH:-$(go env GOARCH)}" \
  && CGO_ENABLED=1 GOOS="$targetos" GOARCH="$targetarch" go build -o /out/goddddocr-server ./cmd/goddddocr-server
RUN targetos="${TARGETOS:-linux}" \
  && targetarch="${TARGETARCH:-$(go env GOARCH)}" \
  && cp "/src/third_party/onnxruntime/${targetos}_${targetarch}/libonnxruntime.so" /out/libonnxruntime.so

FROM debian:bookworm-slim

RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates wget \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=build /out/goddddocr-server /usr/local/bin/goddddocr-server
COPY --from=build /out/libonnxruntime.so /app/third_party/onnxruntime/libonnxruntime.so

ENV ONNXRUNTIME_SHARED_LIBRARY_PATH=/app/third_party/onnxruntime/libonnxruntime.so
EXPOSE 8088

HEALTHCHECK --interval=10s --timeout=3s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8088/ready || exit 1

CMD ["goddddocr-server", "-addr", ":8088"]
