package token

import (
	"github.com/jvlerner/auth-system/internal/token/application"
	"github.com/jvlerner/auth-system/internal/token/presentation"
	"go.uber.org/fx"
)

var Module = fx.Options(
	fx.Provide(
		application.NewTokenService,
		presentation.NewTokenGrpcServer,
	),
)
