package graph

import (
	"btp_tokens/internal/wallets"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require
// here.

type Resolver struct{
	WalletsService *wallets.WalletsService
}
