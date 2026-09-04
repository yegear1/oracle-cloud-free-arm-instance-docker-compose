# ==============================================================================
# Estágio 1: Build do binário estático Go
# ==============================================================================
FROM golang:1.22-alpine AS builder

WORKDIR /build

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum* ./
RUN go mod download || true

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o oracle-fisher main.go

# ==============================================================================
# Estágio 2: Imagem final de produção ultraleve (~20MB)
# ==============================================================================
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copia o binário compilado estático
COPY --from=builder /build/oracle-fisher .

# Mantém scripts legados (Python e Bash) preservados para consulta
COPY main.py requirements.txt oracle_cloud_instance_creator.sh entrypoint.sh ./

# Executa o binário nativo em Go
CMD ["/app/oracle-fisher"]