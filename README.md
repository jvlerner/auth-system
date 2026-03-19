<div align="center">
  <h1>🛡️ Go Auth System</h1>
  <p><strong>IAM Microservices Architecture - Cloud-Agnostic & Production-Ready</strong></p>

  [![Go Version](https://img.shields.io/badge/go-1.26.1+-00ADD8?style=flat-square&logo=go)](https://golang.org/)
  [![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-336791?style=flat-square&logo=postgresql)](https://www.postgresql.org/)
  [![RabbitMQ](https://img.shields.io/badge/RabbitMQ-3.13-FF6600?style=flat-square&logo=rabbitmq)](https://www.rabbitmq.com/)
  [![gRPC](https://img.shields.io/badge/gRPC-v1.62-244c5a?style=flat-square&logo=grpc)](https://grpc.io/)
  [![Docker](https://img.shields.io/badge/Docker-Native-2496ED?style=flat-square&logo=docker)](https://www.docker.com/)
</div>

<br />

Auth System é uma plataforma de Autenticação e Autorização construída com **Clean Architecture**, **Domain-Driven Design (DDD)** e **CQRS** com mensageria.

O sistema foca em performance, portabilidade e low coupling, sendo capaz de rodar em qualquer servidor Linux via Docker sem dependência de cloud proprietária.

---

## 🏗️ Topologia de Microsserviços

```
                  ┌──────────────────────────────────────────────────────┐
                  │                   NGINX (API Gateway)                │
                  │    /api/v1/commands/*  →  api-write                  │
                  │    /api/v1/queries/*   →  api-read                   │
                  └────────────────────┬─────────────────┬───────────────┘
                                       │                 │
                              ┌────────┴──────┐  ┌───────┴──────────┐
                              │   api-write   │  │    api-read       │
                              │ (Commands)    │  │ (Queries + Cache) │
                              └──────┬────────┘  └────────┬──────────┘
                                     │                    │
                 ┌───────────────────┼──────────┐         │
         Save Outbox                 │          │         │
           (Atomic)         ┌────────┴──────┐   │    ┌────┴──────────────┐
                            │  RabbitMQ     │   │    │ postgres-read     │
                            │  (Broker)     │   │    │ + Redis Cache     │
                            └──────┬────────┘   │    └───────────────────┘
                                   │            │
                            ┌──────┴────────┐   │
                            │ worker-events │   │
                            │ (Consumer)    │   │
                            └──────┬────────┘   │
                                   │            │
                   ┌───────────────┼──────┐     │
                   │                      │     │
          ┌────────┴──────┐   ┌───────────┴──┐  │
          │ grpc-password │   │  grpc-token  │◄─┘
          │  (Argon2id)   │   │  (JWT RSA)   │
          └───────────────┘   └──────────────┘
```

A arquitetura é dividida em serviços especializados para garantir escalabilidade e segurança:

1. **`api-write`**: Recebe comandos de mutação. Nunca escreve diretamente nas tabelas de usuários — persiste eventos no **Outbox** (transação atômica com o Master DB) e o `worker-events` os processa.
2. **`api-read`**: Handlers de consulta otimizados com cache Redis e leitura direto da réplica Postgres. Sem acesso ao Master DB.
3. **`worker-events`**: Processa tarefas assíncronas via RabbitMQ — hashing de senha, persistência do usuário, envio de emails, upgrade de hash legado.
4. **`grpc-password`**: Serviço interno de hashing Argon2id (CPU-bound isolado). Porta `50051`.
5. **`grpc-token`**: Serviço interno de emissão/validação de JWT com assinatura RSA assimétrica. Porta `50052`.

---

## 🔐 Rate Limiting e Segurança

O `api-write` aplica **Rate Limiting via Redis** (Fixed Window por IP + Path) nas rotas sensíveis: `register`, `login`, `forgot-password`, `reset-password`, `verify-mfa`. O limite padrão é de **5 req/min por IP**, retornando `429 Too Many Requests` quando excedido.

O `api-read` também deve ter Rate Limiting configurado para endpoints públicos.

> **ℹ️ Idempotency:** A API ainda não possui chaves de idempotência explícitas. O Outbox garante entrega at-least-once, mas operações duplicadas na fila são tratadas no worker (ex: usuário duplicado é descartado silenciosamente via `409 Conflict` detection). **Idempotency keys no nível HTTP estão no backlog.**

---

## 🔄 Fluxos de Autenticação e Dados

### 1. Registro de Usuário
`POST /register` → `api-write` valida email e persiste evento `user.register` no outbox → `worker-events` consome, chama `grpc-password` para hashing Argon2id e persiste o usuário no Master DB. Após persistência, emite evento `UserRegistered` que dispara o envio de email de confirmação.

### 2. Login com MFA (TOTP)
`POST /login` → `api-write` busca usuário e valida senha via `grpc-password`:
- **MFA Inativo:** `grpc-token` gera JWT → retorna `200 OK` com `access_token` + `refresh_token` opaco no Redis.
- **MFA Ativo:** Gera `mfa_token` no Redis (TTL 5min) → retorna `202 Accepted`. O usuário valida via `POST /verify-mfa` para receber o JWT final.

### 3. Refresh Token
`POST /refresh-token` → Valida token opaco no Redis, busca usuário e chama `grpc-token` para emitir novo JWT. Rotação com sliding window (7 dias), token antigo é destruído atomicamente.

### 4. Recuperação de Senha
`POST /forgot-password` → Emite evento `UserForgotPasswordRequested` no Outbox → `worker-events` gera token no Redis (TTL 15min). `POST /reset-password` valida token, chama `grpc-password` e atualiza o hash (burn-after-reading no Redis).

### 5. Confirmação de E-mail
`POST /confirm-email` → Valida token Redis (gerado pelo worker após registro) → seta `email_verified=true` diretamente no Master DB via `api-write`.

### 6. MFA Setup
`POST /setup-mfa` (autenticado) → Gera TOTP secret via `pquerna/otp` → salva secret e habilita MFA no usuário diretamente no Master DB.

### 7. Gestão de Papéis (RBAC)
Os papéis (`roles`) são atribuídos via endpoint Admin. São embutidos nos Claims JWT pelo `grpc-token`, eliminando consultas extras ao banco durante validação de permissões.

### 8. Upgrade Dinâmico de Hash (Legado)
No Login, se o hash for de versão anterior (detectado pelo prefixo `v1:`), o `api-write` emite `UserPasswordUpgradeRequested` no Outbox. O `worker-events` re-executa o hashing na versão atual e atualiza o banco transparentemente, sem retardar a resposta do login.

---

## 📡 Rotas da API (REST)

A API é acessível via **Nginx** na porta `80/443`.

| Método | Endpoint | Acesso | Descrição |
| :--- | :--- | :--- | :--- |
| `POST` | `/api/v1/commands/register` | Público¹ | Cadastro de novo usuário |
| `POST` | `/api/v1/commands/login` | Público¹ | Início da sessão |
| `POST` | `/api/v1/commands/verify-mfa` | Público¹ | Valida 2FA e emite JWT |
| `POST` | `/api/v1/commands/setup-mfa` | **JWT** | Ativa TOTP no perfil |
| `POST` | `/api/v1/commands/forgot-password` | Público¹ | Solicita reset de senha |
| `POST` | `/api/v1/commands/reset-password` | Público¹ | Redefine senha com token |
| `POST` | `/api/v1/commands/refresh-token` | Público | Rotaciona sessão |
| `POST` | `/api/v1/commands/logout` | Público | Invalida refresh token |
| `POST` | `/api/v1/commands/confirm-email` | Público | Valida token de email |
| `POST` | `/api/v1/commands/users/{id}/roles` | **JWT + Admin** | Atualiza papéis do usuário |
| `GET`  | `/api/v1/queries/users/{id}` | Público | Perfil via **API Read** |

¹ *Protegidas por Rate Limiting (5 req/min por IP).*

---

## 🔌 Serviços Internos gRPC

> Comunicação exclusivamente interna (rede Docker). TLS mútuo com certificados autogerados (`configs/certs/`).

### `grpc-password` — Porta `50051`
Serviço de hashing e verificação de senhas, isolando o processamento CPU-bound do fluxo HTTP.

| Método | Entrada | Saída | Descrição |
| :--- | :--- | :--- | :--- |
| `Hash` | `plaintext` | `hashed`, `version` | Gera hash Argon2id com versionamento |
| `Compare` | `plaintext`, `hashed` | `match`, `needs_upgrade` | Compara e sinaliza se precisa re-hash |

O serviço suporta **múltiplas versões de hash** (`v1:`, `v2:...`) via polimorfismo interno. Novos algoritmos são adicionados sem breaking change.

### `grpc-token` — Porta `50052`
Serviço de ciclo de vida de tokens JWT com assinatura RSA assimétrica (RS256).

| Método | Entrada | Saída | Descrição |
| :--- | :--- | :--- | :--- |
| `Generate` | `user_id`, `email`, `roles[]` | `access_token`, `expires_in` | Assina JWT com chave RSA privada |
| `Validate` | `token` | `claims` | Valida e extrai claims (usado internamente) |

Os `roles` são embutidos diretamente no payload do JWT, permitindo validação stateless em qualquer serviço que tenha a chave pública.

---

## 🌐 Nginx — API Gateway

O Nginx funciona como ponto único de entrada, roteando por prefixo de path:

```nginx
/api/v1/commands/*  →  api-write:8080  # Mutações
/api/v1/queries/*   →  api-read:8081   # Leituras
```

Também gerencia **CORS** e é o ponto para configuração de **SSL termination** em produção.

---

## 📦 Pacotes Compartilhados (`pkg/`)

### `pkg/auth` — JWT Guard & RBAC por Contexto
Pacote reutilizável para validação de JWT e controle de acesso. Projetado para ser importado por qualquer microserviço HTTP do monorepo.

```go
// Middleware: valida Bearer token e injeta claims no contexto
r.Use(auth.JWTGuard(auth.Config{PublicKeyPath: "configs/certs/jwt.pub"}, logger))

// Middleware: exige role específica (aplica após JWTGuard)
r.Group(func(admin chi.Router) {
    admin.Use(auth.RequireRoles("admin"))
    admin.Post("/users/{id}/roles", handler.UpdateRoles)
})

// Dentro de um handler: extrai identity do contexto
claims, ok := auth.GetUser(r.Context())  // → UserClaims{Subject, Email, Roles}
userID, ok := auth.GetUserID(r)          // → string (Subject)
isAdmin := auth.HasRole(r.Context(), "admin") // → bool
```

`UserClaims` é populado pelo `JWTGuard` após validar a assinatura RS256 e rejeitar algoritmos alternativos (anti-spoofing `"none"` alg).

### `pkg/middleware` — Rate Limiter Redis
Rate Limiting por IP + Path via Redis INCR/EXPIRE (Fixed Window). Fail-open em caso de falha do Redis para manter disponibilidade.

### `pkg/queue` — AMQP Wrapper
Publisher e Consumer sobre RabbitMQ com suporte a Topic Exchanges e routing keys do padrão `user.*`. O Consumer usa `Ack/Nack` para garantia de entrega at-least-once.

### `pkg/health` — Health Probes
`LivenessHandler` (lightweight 200 OK) e `ReadinessHandler` com verificação de conectividade PostgreSQL e Redis em timeout de 3s, retornando `503` se degradado. Compatível com probes Kubernetes.

### `pkg/telemetry` — OpenTelemetry
Setup do TracerProvider com exporter OTLP/HTTP. Integrado ao Chi router e aos clientes/servidores gRPC via `otelgrpc`.

### `pkg/db` — Pool de Conexões
Bootstrap do `pgxpool` (PostgreSQL) e `go-redis` com hooks de lifecycle Uber Fx para graceful close.

---

## 💻 Estrutura de Diretórios

```text
/
├── cmd/                          # Binários (API Write/Read, grpc e Worker)
├── configs/                      # Certificados TLS, Chaves RSA e Envs
├── internal/                     # Módulos seguindo DDD e Clean Architecture
│   ├── identity/                 # Contexto de Identidade (Aggregate, Use Cases)
│   │   ├── application/          # Casos de Uso (Commands e Queries)
│   │   ├── domain/               # Entidades e Ports (UserRepository Write/Read)
│   │   ├── infrastructure/       # Adapters (Postgres, Redis, gRPC Clients)
│   │   └── presentation/         # Handlers HTTP (Chi)
│   ├── password/                 # Bounded Context do Hash (Argon2id)
│   └── token/                    # Bounded Context do JWT (RSA Signer)
├── pkg/                          # Utilitários globais (DB, Queue, Telemetry)
│   ├── auth/                     # JWTGuard, RequireRoles, UserClaims context
│   ├── db/                       # pgxdb / redisdb bootstrap
│   ├── health/                   # Liveness & Readiness probes
│   ├── middleware/               # Rate Limiter Redis
│   ├── queue/                    # AMQP Publisher & Consumer
│   └── telemetry/                # OpenTelemetry setup
├── migrations/                   # Migrações SQL (golang-migrate)
└── docker-compose.yml            # Orquestração local completa
```

---

## 🛠️ Como Executar Localmente

Você precisará de **Docker** e **Make**. A aplicação resolve as dependências via Dockerfiles.

1.  Clone o repositório e configure o `.env` em /configs:
```bash
git clone https://github.com/jvlerner/auth-system.git
cp .env.example .env
```

2.  Gere credenciais e suba o ambiente:
```bash
make tls-certs
make rsa-keys
docker-compose up -d --build
```
> Isso subirá 9 containers configurados. O banco rodará `migrations` automaticamente.

---

## 📜 Roadmap Implementado
- [x] Roteamento CQRS segregado.
- [x] Transactional Outbox Pattern para persistência garantida.
- [x] Registro assíncrono via Worker (Offloading gRPC).
- [x] MFA TOTP (Google/Microsoft Authenticator).
- [x] Rate Limiting (Redis) e Refresh Tokens rotativos.
- [x] Versionamento e Upgrade Dinâmico de Hash (Argon2).
- [x] Confirmação de E-mail e Recuperação de Senha.
- [x] RBAC (Papéis) dentro dos Claims JWT.
- [x] Tracing Distribuído (OpenTelemetry) e Health Probes.
- [x] pkg/auth reutilizável (JWTGuard + RequireRoles + Context Injection).

**Próximos Passos**:
- [ ] Idempotency Keys no nível HTTP (api-write).
- [ ] Login e RefreshToken lendo da Read Replica em vez do Master DB.
- [ ] Documentação Swagger Dinâmica.
- [ ] Dashboard de Observabilidade (Grafana).
- [ ] Helmchart p/ Kubernetes.
- [ ] Utilizar variveis de ambiente para configuracao de RBAC de endpoints
admin.Use(auth.RequireRoles("admin")) -> admin, user, manager, etc pode vir de envs.
