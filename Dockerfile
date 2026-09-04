# ==============================================================================
# Estágio 1: Build do binário estático Go
# ==============================================================================
FROM golang:1.22-alpine AS builder

WORKDIR /build

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum* ./
RUN go mod download || true

COPY main.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o oracle-fisher main.go

# ==============================================================================
# Estágio 2: Imagem final de produção ultraleve (~20MB)
# ==============================================================================
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copia o binário estático Go compilado
COPY --from=builder /build/oracle-fisher .

# Executa o binário nativo em Go
CMD ["/app/oracle-fisher"]