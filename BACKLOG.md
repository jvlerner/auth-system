# Backlog do Sistema de Autenticação

Este documento contém todas as tarefas mapeadas e pendentes de implementação. Itens concluídos são movidos para o `tasks.md`.

---

## 0. Refinamentos Arquiteturais (Auditoria CQRS)

Gaps identificados na auditoria de conformidade CQRS:

- [ ] **Idempotency Keys (api-write):** Header `Idempotency-Key` + Redis para evitar double-submit de comandos. Chave com TTL de 24h.
- [ ] **Login e RefreshToken na Read Replica:** `LoginUserUseCase` e `RefreshTokenUseCase` consultam o Master DB. Migrar para `UserReadRepository`.
- [ ] **ForgotPassword na Read Replica:** `ForgotPasswordUseCase` valida email no Master DB. Migrar para `UserReadRepository`.
- [ ] **Rate Limiting na `api-read`:** Adicionar Rate Limiter Redis para endpoints públicos de consulta.
- [ ] **RBAC por variável de ambiente:** `auth.RequireRoles(...)` com roles configuráveis via env em vez de hardcoded (`admin`, `user`, `manager`).
- [ ] **Perfil restrito ao próprio usuário (`GET /api/v1/queries/users/{id}`):** O endpoint de perfil deve exigir JWT e comparar o `{id}` da URL com o `sub` do token via `auth.GetUserID()`. Retornar `403 Forbidden` se não coincidir. Admins podem consultar qualquer perfil. O endpoint deve ser movido de "Público" para "**JWT**" na tabela de rotas.

---

## 1. Segurança Avançada

- [ ] **Account Lockout:** Bloquear conta por N minutos após X tentativas de login falhas por conta (além do rate limit por IP). Contador por `user_id` no Redis.
- [ ] **Detecção de Login Suspeito:** Alertar ou bloquear login de novo IP/país/device nunca visto. Exige persistência de `last_known_ip` e comparação de GeoIP.
- [ ] **Revogação Imediata de JWT:** Blacklist de `jti` (JWT ID) no Redis para invalidar tokens antes do expirar, ex: em logout forçado ou suspeita de comprometimento.
- [ ] **Política de Senha:** Validação de complexidade mínima (comprimento, maiúsculas, números, especiais) e restrição de reutilização dos últimos N hashes.
- [ ] **Audit Log de Eventos de Auth:** Tabela ou stream imutável de eventos: logins, logouts forçados, trocas de senha, alterações de role, tentativas falhas. Base para compliance.
- [ ] **Proteção por Device Fingerprint:** Emitir refresh tokens vinculados ao User-Agent + IP para detectar roubo de token.
- [ ] **PKCE / OAuth2 Authorization Code Flow:** Para integrações com SPAs e apps mobile sem expor client secret.

---

## 2. Resiliência e Performance

- [ ] **Circuit Breaker nos Clientes gRPC:** Implementar circuit breaker (ex: `sony/gobreaker`) nas chamadas a `grpc-password` e `grpc-token` para evitar cascata de falhas.
- [ ] **Retry com Exponential Backoff:** Política de retry nos clientes gRPC para falhas transientes.
- [ ] **Dead Letter Queue (DLQ):** Mensagens que falham repetidamente no `worker-events` após N tentativas devem ir para uma DLQ para inspeção manual sem bloquear o processamento.
- [ ] **PostgreSQL Read Replica Lag Monitoring:** Monitorar lag da réplica e redirecionar para Master se o lag exceder threshold configurável.
- [ ] **Redis Sentinel/Cluster:** Suporte a Redis em modo HA para evitar single point of failure na camada de cache e sessões.
- [ ] **Connection Pool Tuning:** Expor via env as configurações de `pgxpool` (`MaxConns`, `MinConns`, `MaxConnLifetime`) e `go-redis` (`PoolSize`, `IdleTimeout`).

---

## 3. Observabilidade e Operação

- [ ] **Dashboard Grafana:** Dashboards pré-configurados para métricas de auth: logins/s, taxa de erro, latência gRPC, cache hit/miss, tamanho da fila RabbitMQ.
- [ ] **Prometheus Metrics:** Expor métricas customizadas via `/metrics`: `auth_login_total`, `auth_login_failures_total`, `auth_token_issued_total`, `outbox_pending_events`, `grpc_password_duration_seconds`.
- [ ] **Alertas (Alertmanager):** Regras de alerta para: taxa de erro > threshold, fila Outbox crescendo, réplica lagging, Redis desconectado.
- [ ] **Tracing End-to-End (Jaeger):** Garantir que trace ID propague do Nginx → api-write → RabbitMQ → worker → gRPC através de W3C Trace Context headers.
- [ ] **Log Estruturado Centralizado:** Exportar logs JSON para stack ELK ou Loki + Grafana para correlação com traces.

---

## 4. Compliance e Governança (Enterprise/GDPR)

- [ ] **Right to Erasure (GDPR Art. 17):** Endpoint autenticado `DELETE /api/v1/commands/account` que apaga todos os dados PII do usuário (users, outbox_events associados, tokens Redis) com audit trail.
- [ ] **Data Export (GDPR Art. 20):** Endpoint para exportar todos os dados do usuário em formato JSON.
- [ ] **Aceite de Termos de Serviço:** Campo `terms_accepted_at` e versão dos termos aceitos na entidade User. Bloquear login se houver novos termos pendentes.
- [ ] **Multi-Tenancy:** Suporte a múltiplos tenants isolados via `tenant_id` no domínio, claims JWT e políticas de quota separadas.

---

## 5. Integração e Extensibilidade (Enterprise SSO)

- [ ] **SAML 2.0 / SSO:** Integração com Identity Providers corporativos (Okta, Azure AD) via SAML para login federado.
- [ ] **LDAP / Active Directory:** Adapter de autenticação LDAP para empresas com diretório corporativo existente.
- [ ] **Social Login (OAuth2):** Login via Google/GitHub com ligação de conta externa ao domínio de identidade interno.
- [ ] **Webhooks de Eventos:** Permitir que sistemas externos se inscrevam em eventos de auth (UserRegistered, PasswordChanged, etc.) via webhook configurável.

---

## 6. Developer Experience e Documentação

- [ ] **Documentação Swagger Dinâmica:** Geração automática da spec OpenAPI 3.0 a partir das rotas Chi.
- [ ] **SDK Go Client:** Pacote Go reutilizável com os métodos de auth pronto para ser importado por outros microserviços do ecossistema.
- [ ] **Admin Dashboard UI:** Interface web para gerenciar usuários, roles, sessões ativas, logs de audit e métricas.
- [ ] **Postman Collection / Bruno:** Collection exportável com todos os endpoints pré-configurados para facilitar testes manuais.

---

## 7. Infraestrutura e Deploy

- [ ] **Helmchart Kubernetes:** Chart com Deployment, Service, Ingress, ConfigMap, Secret, HPA, PDB e ServiceMonitor para cada microsserviço.
- [ ] **GitHub Actions CI/CD:** Pipeline de build, lint (`golangci-lint`), testes unitários, security scan (`gosec`), e push de imagens ao registry.
- [ ] **Testes E2E Automatizados:** Suite de testes de ponta a ponta contra o ambiente Docker Compose (ex: Playwright ou teste Go com `httptest` contra stack real).
- [ ] **Testes de Carga (k6/hey):** Scripts de load test para validar throughput de login, registro e refresh token sob carga de milhares de req/s.
- [ ] **Security Scan:** Integração com `trivy` (CVE em imagens Docker) e `gosec` (SAST em código Go) no pipeline CI.

---

## 8. Testes Documentários

- [ ] **Testes E2E com Swagger:** Expandir ou validar spec OpenAPI com testes automatizados contra o Nginx em ambiente integrado.

---

## ✅ Itens Concluídos (ver tasks.md)

- [x] Roteamento CQRS segregado (api-write / api-read)
- [x] Transactional Outbox Pattern
- [x] Registro assíncrono via Worker com Argon2id gRPC
- [x] MFA TOTP (RFC 6238)
- [x] Rate Limiting Redis (api-write)
- [x] Refresh Tokens rotativos opacos
- [x] Versionamento dinâmico de Hash (Argon2 v1→v2)
- [x] Confirmação de E-mail e Recuperação de Senha
- [x] RBAC embutido nos Claims JWT
- [x] Tracing Distribuído (OpenTelemetry)
- [x] Health Probes (Liveness & Readiness)
- [x] pkg/auth reutilizável (JWTGuard + RequireRoles + Context)
- [x] Segregação de interfaces de repositório (UserRepository Write / UserReadRepository Read)
- [x] Testes unitários (Login, Register, Worker, Handlers HTTP)
