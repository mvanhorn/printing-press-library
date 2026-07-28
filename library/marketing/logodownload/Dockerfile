# Estágio 1: Build
FROM golang:1.26-alpine AS builder

# Instala dependências do sistema necessárias para o build
RUN apk update && apk add --no-cache git ca-certificates

WORKDIR /app

# Copia os arquivos de dependência e faz o download
COPY go.mod go.sum ./
RUN go mod download

# Copia o código-fonte
COPY . .

# Compila um binário estático (sem dependências de C)
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o logodownload-pp-cli ./cmd/logodownload-pp-cli

# Estágio 2: Runner
FROM alpine:latest

# Certificados necessários para chamadas HTTPS
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copia apenas o binário do estágio anterior
COPY --from=builder /app/logodownload-pp-cli .

# Define a CLI como o ponto de entrada do container
ENTRYPOINT ["./logodownload-pp-cli"]
