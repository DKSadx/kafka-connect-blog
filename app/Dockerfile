# Build image
FROM golang:1.23.0 AS build

WORKDIR /build

RUN apt update \
  && apt install -y protobuf-compiler \
  && go install google.golang.org/protobuf/cmd/protoc-gen-go@latest \
  && go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

COPY . .

RUN protoc --go_out="." proto/*.proto \
  && go build -o demo-app \
  && chmod +x demo-app


# Runtime image
FROM debian:stable-slim

WORKDIR /app

CMD ["./demo-app"]

RUN apt update \
  && apt install -y ca-certificates

COPY --from=build /build/demo-app .
COPY --from=build /build/data data
