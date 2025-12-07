package wallets

import (
	"context"
	"database/sql"
	"errors"

	"github.com/shopspring/decimal"
)


type Wallet struct {
	Address string
	Balance decimal.Decimal
}

type WalletsService struct {
	DB *sql.DB
}

var ErrorInsufficientBalance = errors.New("Insufficient wallet balance")

func (s *WalletsService) Transfer(ctx context.Context, fromAddress string, toAddress string, amount decimal.Decimal) (decimal.Decimal, error){
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return decimal.Decimal{}, err
	}

	defer tx.Rollback()
	var senderBalance decimal.Decimal
	queryFrom := "SELECT Balance FROM Wallets WHERE Address = $1 FOR UPDATE"
	err = tx.QueryRowContext(ctx, queryFrom, fromAddress).Scan(&senderBalance)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows){
			return decimal.Decimal{}, errors.New("Sender wallet not found")
		}
		return decimal.Decimal{}, err
	}

	newSenderBalance := senderBalance.Sub(amount)
	if newSenderBalance.IsNegative() {
		return decimal.Decimal{}, ErrorInsufficientBalance
	}

	balanceUpdate := "UPDATE Wallets SET Balance = Wallets.Balance + $1 WHERE Address = $2"

	_, err = tx.ExecContext(ctx, balanceUpdate, newSenderBalance, fromAddress)
	if err != nil {
		return decimal.Decimal{}, err
	}

	_, err = tx.ExecContext(ctx, balanceUpdate, amount, toAddress)
	if err != nil {
		return decimal.Decimal{}, err
	}

	err = tx.Commit()
	if err != nil {
		return decimal.Decimal{}, err
	}

	return newSenderBalance, nil
}

func (s *WalletsService) GetWalletBalance(ctx context.Context, address string) (decimal.Decimal, error) {
	var balance decimal.Decimal
	query := "SELECT Balance FROM Wallets WHERE Address = $1"
	err := s.DB.QueryRowContext(ctx, query, address).Scan(&balance)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return decimal.Zero, errors.New("Wallet not found")
		}
		return decimal.Decimal{}, err
	}
	return balance, nil
}

