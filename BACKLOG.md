# Backlog do Sistema de Autenticação

Este documento contém todas as tarefas mapeadas que faltam ser implementadas para atingirmos o escopo definido no `REQUIREMENTS.md`. Conforme formos finalizando as tarefas aqui, elas deverão ser movidas como registros concluídos para o arquivo `tasks.md`.

## 1. Novas Features de Negócio Avançadas (IAM)




- [ ] **Feature: MFA com TOTP (Google Authenticator)**
  - [ ] Domínio: campos `mfa_enabled bool`, `totp_secret string` na entidade User
  - [ ] Migration: `ALTER TABLE users ADD COLUMN mfa_enabled`, `totp_secret`
  - [ ] Use Case: `SetupMFAUseCase` — gera TOTP secret, retorna QR Code URL
  - [ ] Login: retornar `{ mfa_required: true, mfa_token }` sem JWT quando MFA habilitado
  - [ ] Use Case: `VerifyMFAUseCase` — valida código TOTP + mfa_token Redis → retorna JWT
  - [ ] Rotas: `POST /mfa/setup` (JWT necessário) e `POST /mfa/verify` (pública + rate limit)
  - [ ] Lib: `github.com/pquerna/otp`





## 3. Testes Unitários e Cobertura
O padrão adotado é a criação de Mocks/Stubs em memória do DDD sem instanciar frameworks:
- [x] **Testes:** `login_user_usecase_test.go` - Testar caminhos de erro, usuário não existe, senha inválida e autenticação com sucesso gerando Token Mock.
- [x] **Testes:** `process_user_registration_usecase_test.go` (Dentro do Worker) - Testar o consumo do RabbitMQ, chamada gRPC mockada e Inserção.
- [x] **Testes:** `process_user_roles_update_usecase_test.go` - Consumo, mutação nativa de arrays de papéis e retorno.
- [x] **Testes de Apresentação:** Validar se o Chi Roteador reage corretamente com 400 Bad Request nos Handlers.
- [ ] **Testes Documentários:** Expandir ou validar Swagger com E2E batendo no Nginx.
