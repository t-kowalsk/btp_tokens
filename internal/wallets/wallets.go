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

var ErrorInsufficientBalance = errors.New("insufficient wallet balance")

func (s *WalletsService) Transfer(ctx context.Context, fromAddress string, toAddress string, amount decimal.Decimal) (decimal.Decimal, error){
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return decimal.Decimal{}, err
	}

	defer tx.Rollback()

	if fromAddress == toAddress {
        return decimal.Decimal{}, errors.New("cannot transfer to the same address")
    }

	var senderBalance decimal.Decimal
	queryFrom := "SELECT Address, Balance FROM Wallets WHERE Address IN ($1, $2) ORDER BY Address ASC FOR UPDATE"

	rows, err := tx.QueryContext(ctx, queryFrom, fromAddress, toAddress)

	if err != nil {
		return decimal.Decimal{}, err
	}
	defer rows.Close()

	foundSender := false

	for rows.Next() {
		var address string
		var balance decimal.Decimal
		err := rows.Scan(&address, &balance)
		if err != nil {
			return decimal.Decimal{}, err
		}

		if address == fromAddress{
			senderBalance = balance
			foundSender = true
		}
	}

	if rows.Err() != nil {
		return decimal.Decimal{}, rows.Err()
	}

	if !foundSender {
		return decimal.Decimal{}, errors.New("sender wallet not found")
	}

	newSenderBalance := senderBalance.Sub(amount)
	if newSenderBalance.IsNegative() {
		return decimal.Decimal{}, ErrorInsufficientBalance
	}

	_, err = tx.ExecContext(ctx, "UPDATE Wallets SET Balance = $1 WHERE Address = $2", newSenderBalance, fromAddress)
	if err != nil {
		return decimal.Decimal{}, err
	}

	_, err = tx.ExecContext(ctx, `
        INSERT INTO Wallets (Address, Balance) 
        VALUES ($2, $1)
        ON CONFLICT (Address) 
        DO UPDATE SET Balance = Wallets.Balance + EXCLUDED.Balance;
    `, amount, toAddress)
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

