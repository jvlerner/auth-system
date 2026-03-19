package identity

import (
	"github.com/jvlerner/auth-system/internal/identity/application"
	"github.com/jvlerner/auth-system/internal/identity/presentation"
	"go.uber.org/fx"
)

// Module expõe as dependências do contexto Identity para a raiz da aplicação (Clean Architecture DI)
var Module = fx.Options(
	// Repositories & Adapters (Infra)
	// Handlers REST (Presentation)
	// Casos de Uso (Application)
	fx.Provide(
		application.NewRegisterUserUseCase,
		application.NewLoginUserUseCase,
		application.NewUpdateUserRolesUseCase,
		application.NewConfirmEmailUseCase,
		application.NewForgotPasswordUseCase,
		application.NewResetPasswordUseCase,
		application.NewRefreshTokenUseCase,
		application.NewLogoutUseCase,
		application.NewSetupMFAUseCase,
		application.NewVerifyMFAUseCase,
		application.NewProcessPasswordUpgradeUseCase,
		presentation.NewAuthHandler,
	),
)
