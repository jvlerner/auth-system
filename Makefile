# Makefile for Authentication System (WSL/Linux)

.PHONY: help rsa-keys test test-coverage

help:
	@echo "Usage:"
	@echo "  make rsa-keys         Generate RSA keys for JWT signing"
	@echo "  make tls-certs        Generate TLS certificates for internal RPCs and Nginx gateway"
	@echo "  make test             Run all unit tests with per-package coverage"
	@echo "  make test-coverage    Generate HTML coverage report in coverage.html"

rsa-keys:
	@mkdir -p configs/rsa
	@openssl genrsa -out configs/rsa/private.pem 2048
	@openssl rsa -in configs/rsa/private.pem -pubout -out configs/rsa/public.pem
	@echo "RSA keys generated successfully in configs/rsa/"

# Roda os testes unitários com cobertura por pacote (sem arquivo de perfil)
test:
	@echo "Running unit tests..."
	@go test ./internal/... -count=1 -cover

# Gera relatório HTML completo de cobertura (abre no browser)
test-coverage:
	@echo "Generating coverage report..."
	@go test ./internal/identity/... -count=1 -coverprofile=coverage.out
	@go tool cover -html=coverage.out -o coverage.html
	@echo "✓ Report saved to coverage.html"

tls-certs:
	@mkdir -p configs/certs
	@echo "Generating generic self-signed certificate for internal RPCs and Nginx gateway..."
	@openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
		-keyout configs/certs/server.key -out configs/certs/server.crt \
		-subj "/C=BR/ST=SP/L=Sao_Paulo/O=AuthSystem/OU=Dev/CN=localhost" \
		-addext "subjectAltName=DNS:localhost,DNS:api-gateway,DNS:grpc-password,DNS:grpc-token,IP:127.0.0.1"
	@echo "Certificates generated in configs/certs/"

test-register:
	@echo "Enviando comando de Registro para a API (HTTPS port 443)..."
	curl -k -X POST https://localhost/api/v1/commands/register \
		-H "Content-Type: application/json" \
		-d '{"email": "teste@meusistema.com", "password": "SenhaSuperForte123"}'
	@echo ""

test-login:
	@echo "Enviando comando de Login para a API (HTTPS port 443)..."
	curl -k -X POST https://localhost/api/v1/commands/login \
		-H "Content-Type: application/json" \
		-d '{"email": "teste@meusistema.com", "password": "SenhaSuperForte123"}'
	@echo ""
