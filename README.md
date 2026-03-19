<div align="center">
  <h1>🛡️ Go Auth System</h1>
  <p><strong>A Cloud-Agnostic, Production-Ready Identity & Access Management (IAM) Microservices Architecture</strong></p>

  [![Go Version](https://img.shields.io/badge/go-1.22+-00ADD8?style=flat-square&logo=go)](https://golang.org/)
  [![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-336791?style=flat-square&logo=postgresql)](https://www.postgresql.org/)
  [![RabbitMQ](https://img.shields.io/badge/RabbitMQ-3.13-FF6600?style=flat-square&logo=rabbitmq)](https://www.rabbitmq.com/)
  [![gRPC](https://img.shields.io/badge/gRPC-v1.62-244c5a?style=flat-square&logo=grpc)](https://grpc.io/)
  [![Docker](https://img.shields.io/badge/Docker-Native-2496ED?style=flat-square&logo=docker)](https://www.docker.com/)
</div>

<br />

Auth System é uma plataforma de Autenticação e Autorização construída sob rigorosos padrões de engenharia de software abstrata. Foi projetada do zero fravorecendo os conceitos de **Clean Architecture**, **Domain-Driven Design (DDD)**, e **Segregação de Comandos e Consultas (CQRS)** com Mensageria Durável.

O sistema é focado em performance absoluta, custos baixíssimos de manutenção (completamente Host-Agnostic, capaz de rodar em qualquer VPS isolada ou Cluster K3s sem dependência de serviços proprietários da AWS/GCP), e alta modularidade interna (Monorepo de Múltiplos Entrypoints).

---

## 🏗️ Topologia e Arquitetura de Microsserviços

Em vez de um monólito frágil que tranca threads devido à carga de hashing de senhas ou escritas no DB, a arquitetura foi desmembrada com precisão cirúrgica em contêineres ultra-leves:

1. **`api-write`**: Recebe requisições de mutação (ex: Registro, Atualização de Profile).
   - *Design Pattern*: CQRS Command Side + Transactional Outbox.
   - O `api-write` nunca bloqueia para publicar mensagens externas. Ele persiste eventos `outbox` atômicamente no mesmo banco que os dados do usuário e possui um Pooler nativo que esvazia a fila retransmitindo para o Broker.
2. **`api-read`**: Apenas GET Requests. 
   - Atinge o banco de dados réplica (Read Replica Stream) e se beneficia de Cache (Redis). Escalável infinitamente e completamente sem bloqueios.
3. **`worker-events`**: Consumidor RabbitMQ background.
   - Puxa cargas de "trabalho sujo", delegando-as para os microserviços internos e confirmando gravações definitivas orientadas a Eventos.
4. **`grpc-password`**: Microserviço interno isolado 100% de CPU-bound. É especialista rodar a derivação pesada Argon2id para proteção OWASP contra Brute-Force limitando gargalos HTTP.
5. **`grpc-token`**: Microserviço interno encarregado do ciclo de vida de JWT. (Assinatura RSA Assimétrica), embutindo as "Roles" do usuário na Claim Criptográfica.

![Architecture Flow](https://upload.wikimedia.org/wikipedia/commons/thumb/1/14/CQRS.svg/512px-CQRS.svg.png) *(Ilustração CQRS padrão adotado)*

---

## 🚀 Tecnologias Essenciais

*   **Linguagem Core:** Golang puro (Livre de frameworks ORM mágicos ou pesados).
*   **Driver & Injeção DI:** `jackc/pgx/v5` (Performance SQL Extrema) & `uber-go/fx` (Dependency Injection and Graceful Shutdown Lifecycle Tracker).
*   **Roteamento HTTP / Observabilidade:** `go-chi/chi/v5` integrado nativamente à suíte **OpenTelemetry (OTel)** no Roteador e nos Transportes gRPC para Traces Perfeitos.
*   **Mensageria & EDA (Event Driven):** **RabbitMQ** com exchanges de Tópicos e Tratamentos de Delivery At-Least-Once.
*   **Armazenamento Isolado:** Instâncias **PostgreSQL** divididas entre Instância Master (Write) / Replica Embutida (Read) com Streaming Lógico Nativo do motor.
*   **Segurança (TLS):** Toda a malha gRPC interna opera em canais encriptados TLS usando Certificados Autogerados injetados nos volumes dos contêineres.

---

## 💻 Estrutura de Diretórios (Go Standard Layout)

```text
/
├── cmd/                          # Binários (API Write/Read, grpc, e Worker)
├── configs/                      # Certificados TLS gerados, Chaves RSA Locais e Envs
├── internal/                     # Domain-Driven Design Modules
│   ├── identity/                 # Agregados raiz, Domínio, Entidades e Repos
│   │   ├── application/          # Use Cases puros, handlers CQRS e DTOs
│   │   ├── domain/               # Core do Negócio zero-dependencies (User, Roles)
│   │   ├── infrastructure/       # Data Adapters Postgres, Implementações Outbox 
│   │   └── presentation/         # Ponto de amarração HTTP/gRPC (Routes do Chi)
│   ├── password/                 # Core domain da geração do Hash
│   └── token/                    # Core domain dos tokens (Signer RSA)
├── pkg/                          # Utilitários não vinculados ao core business
│   ├── db/                       # Bootstrap db (pgx / redisv9)
│   ├── queue/                    # AMQP Wrapper (Publisher, Consumer handlers)
│   └── telemetry/                # Setup provedor OpenTelemetry
├── migrations/                   # Arquivos dbUP/dbDOWN .sql p/ golang-migrate
└── docker-compose.yml            # Malha orquestradora All-in-One Local
```

---

## 🔄 Fluxos de Autenticação e Dados

O sistema opera com separação estrita de Command/Query (CQRS) e Mensageria:

### 1. Registro & Sincronização
`POST /register` → `api-write` valida → `Postgres Master` insere User + Evento Outbox → `OutboxRelay` publica no **RabbitMQ** → `worker-events` consome e sincroniza na réplica de leitura `Postgres Read`.

### 2. Login com MFA (TOTP)
`POST /login` → `api-write` consulta via `grpc-password`:
- **Se MFA Inativo**: Gera JWT imediatamente via `grpc-token` → Retorna 200 OK.
- **Se MFA Ativo**: Gera um `mfa_token` no **Redis** (5min) → Retorna 202 Accepted.
- O usuário deve então chamar `POST /verify-mfa` com o código do App Authenticator (Google/Microsoft) para receber os tokens finais.

---

## 📡 Principais Rotas da API (REST)

A API é acessível via **Nginx Gateway** na porta 80/443.

| Método | Endpoint | Acesso | Descrição |
| :--- | :--- | :--- | :--- |
| `POST` | `/api/v1/commands/register` | Público | Cadastro de novo usuário |
| `POST` | `/api/v1/commands/login` | Público¹ | Início da sessão (suporta MFA flag) |
| `POST` | `/api/v1/commands/verify-mfa` | Público¹ | Valida 2FA e emite JWT final |
| `POST` | `/api/v1/commands/setup-mfa` | **Privado** | Gera `Secret` e `QRCode` para TOTP |
| `POST` | `/api/v1/commands/forgot-password` | Público¹ | Solicita link de recuperação |
| `POST` | `/api/v1/commands/reset-password` | Público¹ | Altera senha com token temporário |
| `POST` | `/api/v1/commands/refresh-token` | Público | Rotaciona sessão sem senha |
| `POST` | `/api/v1/commands/logout` | Público | Expulsa sessão/refresh do Redis |
| `GET` | `/api/v1/queries/users/{id}` | Público | Retorna perfil via **API Read** |

¹ *Rotas protegidas por Rate Limiting agressivo (Anti Brute-Force).*

---

## 💻 Estrutura de Diretórios (Go Standard Layout)

```text
/
├── cmd/                          # Binários (API Write/Read, grpc, e Worker)
├── configs/                      # Certificados TLS gerados, Chaves RSA Locais e Envs
├── internal/                     # Domain-Driven Design Modules
├── pkg/                          # Utilitários não vinculados ao core business
├── migrations/                   # Arquivos dbUP/dbDOWN .sql p/ golang-migrate
└── docker-compose.yml            # Malha orquestradora All-in-One Local
```

---

## 🛠️ Como Executar Localmente

Você precisará de **Docker** e **Make** e do compilador **Go 1.26+**. A aplicação resolve todas as dependências automaticamente via multi-stage Dockerfiles.

1.  Clone o repositório:
```bash
git clone https://github.com/jvlerner/auth-system.git
cd auth-system
```

2.  Crie os `.env` locais:
```bash
cp .env.example .env
```

3.  Gere os Certificados Locais e Chaves JWT:
```bash
# Se tiver make:
make tls-certs
make rsa-keys
```

4.  Suba os microsserviços:
```bash
docker-compose up -d --build
```
> Isso subirá 9 containers configurados. O banco rodará `migrations` automaticamente.

---

## 📜 Roadmap Implementado
- [x] Roteamento CQRS (API Read/Write segregadas)
- [x] Transacional Outbox Pattern (Master -> Worker -> Replica)
- [x] gRPC-Password (Argon2id CPU Offloading)
- [x] gRPC-Token (RSA Signer Assimétrico)
- [x] **MFA TOTP** (RFC 6238) - Google/Microsoft Authenticator
- [x] **Rate Limiting** (Redis Token Bucket) contra Brute-Force
- [x] Refresh Tokens Rotativos & Revogação
- [x] Versionamento e Upgrade Dinâmico de Hash (Argon2 v1 -> v2)
- [x] Tracing Distribuído com **OpenTelemetry** e **Jaeger**
- [x] Health Probes (Liveness & Readiness) p/ Kubernetes

**Próximos Passos**:
- [ ] Documentação Swagger Dinâmica (v2)
- [ ] Testes E2E com Cypress/Playwright (Integrado)
- [ ] Dashboard de Observabilidade (Grafana/Prometheus)
- [ ] Helmchart para Kubernetes com Ingress, ConfigMap, Secrets, HPA, PDB, ServiceMonitor e Deployment.
