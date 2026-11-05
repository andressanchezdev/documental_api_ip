# Etapa 1: Compilación con el entorno oficial de Go
FROM golang:alpine AS builder

WORKDIR /app

# Descargar módulos de Go
COPY go.mod go.sum ./
RUN go mod download

# Copiar el código fuente completo
COPY . .

# Compilar binario estático optimizado para Linux
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o api ./cmd/api

# Etapa 2: Imagen final ligera de producción
FROM alpine:latest

# Certificados SSL para Google Drive / Docs APIs y zona horaria
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copiar el binario generado
COPY --from=builder /app/api .

# Exponer el puerto
EXPOSE 8080

# Iniciar el backend
CMD ["./api"]

