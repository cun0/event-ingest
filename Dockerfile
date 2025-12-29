# Dockerfile
FROM golang:1.24.11 AS build

WORKDIR /src

# Prevent Go from trying to download a different toolchain during build.
ENV GOTOOLCHAIN=local
ENV CGO_ENABLED=0

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
RUN GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM gcr.io/distroless/static:nonroot

WORKDIR /app
COPY --from=build /out/api /app/api

ENV PORT=8080
EXPOSE 8080

USER nonroot:nonroot
ENTRYPOINT ["/app/api"]
