# syntax=docker/dockerfile:1

# ---- Derleme ----
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY web ./web
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/liderotomarket ./cmd/liderotomarket

# ---- Çalıştırma ----
# Ürün verisi (data/) yerelde "make guncelle" ile üretilir ve imaja gömülür; sunucuda disk (volume) gerekmez.
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/liderotomarket /app/liderotomarket
COPY config.yaml /app/config.yaml
COPY data /app/data
ENV PORT=8080
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/liderotomarket", "serve"]
