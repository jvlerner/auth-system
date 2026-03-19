# Requisitos do Sistema: Serviço de Autenticação (Auth Service)

## 1. Visão Geral
Sistema de autenticação escalável, seguro e de baixo custo operacional. Desenvolvido em Golang utilizando a abordagem de microsserviços, com segregação de responsabilidades entre escrita e leitura (CQRS) e comunicação orientada a eventos, permitindo alta performance e independência de cloud provider (Cloud-Agnostic). A base de código seguirá estritamente os princípios do **Domain-Driven Design (DDD)**, **Clean Architecture** (Robert C. Martin) e princípios SOLID.

## 2. Decisões de Arquitetura

### 2.1 Padrões Arquiteturais e Code Design
*   **Domain-Driven Design (DDD) & Clean Architecture:** O sistema é organizado por *Bounded Contexts* (ex: Identidade, Tokens). O núcleo da aplicação (camada `domain`) deve ser completamente isolado de frameworks, bibliotecas externas pesadas e detalhes de infraestrutura.
*   **Inversão de Dependência (DIP):** Aplicada rigorosamente. Interfaces (*Ports*) são definidas pelas camadas internas (`domain` ou `application`) e implementadas nas camadas mais externas (`infrastructure`). A direção da dependência é sempre de fora para dentro.
*   **Microserviços & Monorepo:** Serviços independentes no mesmo repositório, facilitando a reutilização de código puramente técnico (ex: drivers, logs) através da pasta genérica `pkg/`.
*   **CQRS (Command Query Responsibility Segregation):**
    *   **API Write (`api-write`):** Recebe comandos (ex: `RegisterUser`), executa a lógica de negócios central via UseCases, altera o estado no banco, e publica eventos de domínio.
    *   **API Read (`api-read`):** Destinada unicamente à consulta de dados (ex: `GetProfile`). Executa consultas diretas e otimizadas.
*   **API Gateway (Nginx):** Ponto único de entrada (porta 80/443) para rotear requisições para `api-write` ou `api-read`, resolvendo problemas de CORS no frontend e simplificando a malha de rede para o Client. Mantido simples e altamente otimizado por default no Docker Compose.
*   **Event-Driven Architecture (EDA) & Transactional Outbox:** Comunicação assíncrona garantida. O `api-write` salva comandos no banco e na tabela *outbox* atômicamente. Um worker lê o outbox e garante a entrega ao RabbitMQ (At-Least-Once Delivery), e finalmente o consumidor processa.
*   **Injeção de Dependências & Graceful Shutdown (`go.uber.org/fx`):** Gerenciamento rigoroso do ciclo de vida da aplicação. Facilita o Clean Architecture injetando os "Ports" nos "Adapters" de forma segura e limpa, e garante que sinais OS (`SIGTERM`) façam a aplicação encerrar conexões (banco/fila) suavemente sem perder dados.
*   **gRPC para Serviços Internos:** Isolamento em rede fechada de domínios críticos como geração de hash de senhas e assinatura de tokens.

### 2.2 Stack Tecnológica
*   **Linguagem:** Golang (Standard library + frameworks ultraleves).
*   **Roteamento HTTP:** `go-chi/chi` (100% compatível com `net/http`, leve e muito performático).
*   **Bancos de Dados Relacionais (CQRS Físico):** PostgreSQL, segregado fisicamente em duas instâncias:
    *   **Postgres Write:** Banco normalizado, focado em alta concorrência de escritas e armazenamento da tabela *Outbox*. Acessado apenas pelo `api-write`.
    *   **Postgres Read:** Banco desnormalizado (tabelas otimizadas para leitura/queries). Acessado pelo `api-read`. É alimentado/sincronizado através dos eventos processados pelo Consumer/Worker do RabbitMQ.
*   **Fila/Mensageria:** RabbitMQ, para publicação de eventos duráveis.
*   **Cache de Sessões/Tokens:** Redis.
*   **Criptografia & Segurança:**
    *   **Hash de Senha:** `Argon2id` (resistente a ataques de hardware/GPU).
    *   **Tokens:** JWT assinado com chaves assimétricas **RSA** (RS256).
*   **Infraestrutura:** Docker e Docker Compose nativos.
*   **Migrations:** `golang-migrate/migrate` rodando como container efêmero no docker-compose para preparar o banco de forma agnóstica antes das APIs iniciarem.
## 3. Topologia de Serviços

O sistema será subdividido nos seguintes entrypoints e serviços:

1.  **API Write (`api-write`)**: Serviço REST para mutações.
2.  **API Read (`api-read`)**: Serviço REST para leitura.
3.  **Worker (`worker-events`)**: Consumidor de eventos assíncronos (background jobs).
4.  **gRPC Token Service (`grpc-token`)**: Serviço Geração/Validação de JWT RSA.
5.  **gRPC Password Service (`grpc-password`)**: Serviço de validação/hash Argon2id (CPU-bound).

## 4. Estrutura de Diretórios Proposta (Clean Architecture + DDD)

```text
/
├── cmd/                        # Entrypoints das aplicações
│   ├── api-write/              # main.go
│   ├── api-read/               # main.go
│   ├── worker-events/          # main.go
│   ├── grpc-password/          # main.go
│   └── grpc-token/             # main.go
├── internal/                   # Código de negócio isolado (Bounded Contexts)
│   ├── identity/               # Exemplo de Bounded Context (Gerenciamento de Usuários)
│   │   ├── domain/             # Entidades/Aggregates (User), Value Objects (Email, Password), e Interfaces de Repositório/Serviços. ZERO dependências externas.
│   │   ├── application/        # UseCases (Handlers para Commands e Queries), DTOs e Ports. Orquestra a lógica, dependendo APENAS do Domain.
│   │   ├── infrastructure/     # Implementações concretas de Repositórios (Postgres), publishers (RabbitMQ).
│   │   └── presentation/       # Handlers HTTP/gRPC. Consome a camada Application.
│   └── (outros contextos)...   # Ex: token/, auth/
├── pkg/                        # Bibliotecas genéricas técnicas. Não possui regras de negócio.
│   ├── db/                     # Setup de conexão pgx
│   ├── logger/                 # Setup de slog estruturado utilizando uber/zap
│   └── env/                    # Setup env
├── configs/                    # Arquivos .env, chaves RSA
├── deployments/                # Dockerfiles
├── docker-compose.yml
├── Dockerfile
└── tasks.md
```

## 5. Práticas de Implementação e Código Limpo

1.  **Domain Rules First:** O desenvolvimento deve ser de dentro para fora. O comportamento e as restrições da entidade (ex: um `User` deve estar sempre válido ao ser instanciado em memória) vêm antes da modelagem do banco de dados.
2.  **Clean Code:** 
    *   Nomes expressivos e baseados no idioma ubíquo do domínio.
    *   Funções pequenas e com responsabilidade única.
    *   Tratamento de erros explícito, encapsulando erros de infraestrutura sem vazar os detalhes para o domínio/apresentação (utilizando sentinelas ou tipos em Go).
3.  **Cloud-Agnostic & Performance:** Todas as implementações concretas (banco, fila, etc.) devem ser provisionáveis por Docker para facilitar o self-hosting em VPS. Operações custosas devem ser tratadas de forma concorrente apenas quando necessário, utilizando as primitivas seguras de concorrência do Go e nunca bloqueando as requisições principais de leitura.
4.  **Observabilidade:** Logs serão estruturados com formato JSON contendo contexto de rastreio (`trace_id`, `user_id`, `event`). As principais requisições (sign-up, login) devem emitir métricas (logs ou via um exportador prometheus).
5.  **Testes:**
    *   **Unitários:** Devem ser escritos para cada função, utilizando o padrão Arrange-Act-Assert.
    *   **Integração:** Devem ser escritos para cada UseCase, testando a interação entre as camadas.
    *   **E2E:** Devem ser escritos para cada endpoint, testando a interação entre as camadas.
6.  **Libs**
    *   **Utilizar libs** famosas para resolver problemas como libs da Uber, por exemplo.
7.  **Segurança:**
    *   **Hash de Senha:** `Argon2id` (resistente a ataques de hardware/GPU).
    *   **Tokens:** JWT assinado com chaves assimétricas **RSA** (RS256).
8.  **Performance:**
    *   **Cache de Sessões/Tokens:** Redis.
    *   **Operações Custosas:** Devem ser tratadas de forma concorrente apenas quando necessário, utilizando as primitivas seguras de concorrência do Go e nunca bloqueando as requisições principais de leitura.
8.  **Documentação:**
    *   **API:** Swagger/OpenAPI para documentação das APIs REST.
    *   **Código:** Comentários em inglês explicando a lógica de negócio e as decisões de arquitetura.

## 6. Funcionalidades de Domínio e Identidade (Roadmap)
Para atingir o estado de maturidade de Identity and Access Management (IAM), o serviço suportará as seguintes jornadas:

### 6.1 Confirmação de E-mail
*   Durante o `Register`, o estado inicial do usuário é "Pendente".
*   Um evento no RabbitMQ dispara o Worker para gerar um Token Seguro (ex: JWT curto ou randômico salvo no Redis/DB) e enviar via SMTP (Provedores Free-Tier: **Resend**, **SendGrid** ou Local Mock **MailHog**).
*   Endpoint GET no API Write valida o hash e marca a flag `email_verified = true`.

### 6.2 Recuperação de Senha (Forgot Password)
*   Endpoint POST `/forgot-password` que checa o email e emite um evento.
*   Worker escuta o evento, gera um Token de Uso Único (válido por 10min) no Redis e envia o e-mail transacional.
*   Endpoint POST `/reset-password` recebe a nova senha e o token, e delega para o gRPC-Password re-hashear e a API Write grava o update.

### 6.3 Controle de Acesso Baseado em Perfis (RBAC - Roles)
*   A entidade Usuário conterá claims de Acesso (Ex: `admin`, `user_premium`, `viewer`).
*   O microserviço `grpc-token` assinará no payload do JWT RSA estas Roles do usuário.
*   O API Gateway (ou Middlewares da API Read/Write) bloquearão rotas mediante verificação da claim.

### 6.4 Gestão de Sessão e Refresh Tokens
*   Geração de `Access Token` estrito de vida curta (15 minutos).
*   Geração de `Refresh Token` opaco longo e persistido no Redis vinculado ao UserID / DeviceID.
*   Endpoint de Rotação (`/refresh`) com mitigação de Re-use familiar (Anulação de chain em caso de uso duplo).
*   Endpoint `/logout` ou `/revoke` expulsando forçadamente os tokens no Cache da Blacklist.

### 6.5 Multi-Factor Authentication (MFA/2FA)
*   Possibilidade do Usuário ativar TOTP (Time-Based One Time Password), como Google Authenticator.
*   Setup de geração da semente (Secret) no banco, QR Code Endpoint, e validação no `/login`.
*   O `Login` deve retornar uma flag de fluxo pendente exigindo a segunda rota de `/verify-mfa` com o código temporário.

### 6.6 Rate Limiting e Prevenção Contra Abusos
*   Implementação de um Middleware Chi usando o Algoritmo Token Bucket nativo ou Redis Limit (janela de tempo).
*   Taxas sensíveis (ex: 5 requisições/minuto no IP para Endpoints mutáveis como `/login` e `/register`) evitando Brute-Force severo sob protocolo 429 Too Many Requests.

### 6.7 Segurança de Edge e Validadores
*   **JWT Anti-Spoofing Middleware**: Pacote centralizado `pkg/auth` para roteadores. Ele deve garantir forçadamente a decodificação através de curva RSA (`RS256`), evitando chaves simétricas fantasmas (`HS256` spoofing) e ataques algorítmicos "none".
*   Injeção transparente de claims vitais (UserID, Email, Roles) pelo Middleware no escopo seguro de request HTTP `context.Context`.
