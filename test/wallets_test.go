package test

import (
	"btp_tokens/graph"
	database "btp_tokens/internal/pkg/db/migrations/postgres"
	"btp_tokens/internal/wallets"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/shopspring/decimal"
)

type Wallet struct {
	Address string
	Balance decimal.Decimal
}

type Transfer struct {
    FromAddress string
    ToAddress string
    Amount string
}


func setupTestDB(t *testing.T) *sql.DB {
    db, err := sql.Open("postgres", "postgres://postgres:dbpass@localhost:5432/btp_tokens_test?sslmode=disable")
    if err != nil {
        t.Fatal(err)
    }
    database.Db = db

    database.Migrate("../internal/pkg/db/migrations/postgres")

    return db
}

func ResetTestDB() {
    _, _ = database.Db.Exec("TRUNCATE TABLE wallets RESTART IDENTITY CASCADE;")
}

func SetWallets(wallets []Wallet) {
    for _, w := range wallets{
        _, _ = database.Db.Exec(`
        INSERT INTO wallets (address, balance) VALUES
        ($1, $2) 
        ON CONFLICT (address) DO UPDATE SET balance = $2
    `, w.Address, w.Balance)
    }
}

func SetUpTest(t *testing.T, wallets []Wallet) (*sql.DB, *httptest.Server) {
    db := setupTestDB(t)
    ResetTestDB()
    SetWallets(wallets)

    server := startTestServer(db)
    return db, server
}

func startTestServer(db *sql.DB) *httptest.Server {
    resolver := &graph.Resolver{
        WalletsService: &wallets.WalletsService{DB: db},
    }
    srv := handler.NewDefaultServer(graph.NewExecutableSchema(graph.Config{Resolvers: resolver}))
    server := httptest.NewServer(srv)
    return server
}

func doMutation(t *testing.T, serverURL, mutation string) map[string]interface{} {
    body, _ := json.Marshal(map[string]string{"query": mutation})
    resp, err := http.Post(serverURL, "application/json", bytes.NewBuffer(body))
    if err != nil {
        t.Fatal(err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        t.Fatalf("expected 200, got %d", resp.StatusCode)
    }

    var respData map[string]interface{}
    if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
        t.Fatal(err)
    }
    return respData
}

func assertGraphQLError(t *testing.T, resp map[string]interface{}, expectedMsg string) {
    errors, ok := resp["errors"]
    if !ok || len(errors.([]interface{})) == 0 {
        t.Fatal("expected error but none returned")
    }
    msg := errors.([]interface{})[0].(map[string]interface{})["message"].(string)
    if msg != expectedMsg {
        t.Fatalf("expected error message %q, got %q", expectedMsg, msg)
    }
}

type transferTestArgs struct {
    t *testing.T 
    fromAddress string
    toAddress string
    amount string
    expectedKey string
    expectedValue string
    expectedErrorMsg string
}

func transferTest(args transferTestArgs, initial_wallets []Wallet) {
    _, server := SetUpTest(args.t, initial_wallets)
    defer database.CloseDB()
    defer server.Close()
    mutation := fmt.Sprintf(`
        mutation {
            transfer(input: {
            from_address: "%s", 
            to_address: "%s", 
            amount: "%s"
            }) 
        }
    `, args.fromAddress, args.toAddress, args.amount)

    transferResponse := doMutation(args.t, server.URL, mutation)

    if args.expectedErrorMsg != "" {
        assertGraphQLError(args.t, transferResponse, args.expectedErrorMsg)
    } else {

        respValue := transferResponse["data"].(map[string]interface{})[args.expectedKey]
        if respValue != args.expectedValue {
            args.t.Fatalf("expected %s, got %s", args.expectedValue, respValue)
        }
    }
}

func raceConditionsTest(t *testing.T, initial_wallets []Wallet, transfers []Transfer) {
    db, server := SetUpTest(t, initial_wallets)
    walletsService := &wallets.WalletsService{DB: db}
    
    defer database.CloseDB()
	defer server.Close()
    
    transfersNumber := len(transfers)
    var expectedTotal decimal.Decimal
    for _, wallet := range initial_wallets {
        expectedTotal = expectedTotal.Add(wallet.Balance)
    }


    ready := make(chan struct{}, transfersNumber)
    start := make(chan struct{})
    results := make(chan string, transfersNumber)
    
    var wg sync.WaitGroup
    wg.Add(transfersNumber)
    
    for i:= 0; i < transfersNumber; i++ {
        go func (idx int)  {
            defer wg.Done()
            ready <- struct{}{}

            <- start
            
            mutation := fmt.Sprintf(`
            mutation {
                transfer(input: {
                    from_address: "%s",
                    to_address: "%s",
                    amount: "%s"
                })
            }
            `, transfers[idx].FromAddress, transfers[idx].ToAddress, transfers[idx].Amount)

            resp := doMutation(t, server.URL, mutation)
            if errors, ok := resp["errors"]; ok {
                results <- fmt.Sprintf("error:\n from: %v to: %v \n amount: %v \n error msg: %v", transfers[idx].FromAddress, transfers[idx].ToAddress, transfers[idx].Amount,  errors)
            } else {
                results <- fmt.Sprintf("success: \n from: %v to: %v \n amount: %v \n senders updated balance %v", transfers[idx].FromAddress, transfers[idx].ToAddress, transfers[idx].Amount, resp["data"].(map[string]interface{})["transfer"])
            }
        }(i)
    }

    for i := 0; i < transfersNumber; i++ {
        <-ready
    }

    close(start)

    wg.Wait()
    close(results)

    for r := range results {
        t.Log(r) 
    }


    var total decimal.Decimal
    for i := 0; i<len(initial_wallets); i++ {
        balance, _ := walletsService.GetWalletBalance(context.Background(), initial_wallets[i].Address)
        if balance.IsNegative(){
            t.Fatalf("Negative balance: %s", balance)
        }
        total = total.Add(balance)
        t.Logf("\nWallet%s: %s", initial_wallets[i].Address,  balance)

    }

    if !total.Equal(expectedTotal) {
        t.Fatalf("Final sum mismatch: expected %s, got %s", expectedTotal, total)
    }
}

func TestTransferMutationWithDB(t *testing.T) {
    initial_wallets := []Wallet{
        {Address: "0x0000000000000000000000000000000000000001", Balance: decimal.NewFromInt(10)},
        {Address: "0x0000000000000000000000000000000000000002", Balance: decimal.NewFromInt(15)},
    }

    args := transferTestArgs{
        t: t,
        fromAddress: "0x0000000000000000000000000000000000000001",
        toAddress: "0x0000000000000000000000000000000000000002",
        amount: "6",
        expectedKey: "transfer",
        expectedValue: "4",
        expectedErrorMsg: "",
    }
    transferTest(args, initial_wallets)
}

func TestTransferMutation(t *testing.T) {
    initial_wallets := []Wallet{
        {Address: "0x0000000000000000000000000000000000000001", Balance: decimal.NewFromInt(1000000)},
        {Address: "0x0000000000000000000000000000000000000002", Balance: decimal.NewFromInt(400000)},
    }

    args := transferTestArgs{
        t: t,
        fromAddress: "0x0000000000000000000000000000000000000001",
        toAddress: "0x0000000000000000000000000000000000000002",
        amount: "700000",
        expectedKey: "transfer",
        expectedValue: "300000",
        expectedErrorMsg: "",
    }
    transferTest(args, initial_wallets)
}

func TestTransferNegative(t *testing.T) {
    initial_wallets := []Wallet{
        {Address: "0x0000000000000000000000000000000000000001", Balance: decimal.NewFromInt(1000)},
        {Address: "0x0000000000000000000000000000000000000002", Balance: decimal.NewFromInt(500)},
    }

    args := transferTestArgs{
        t: t,
        fromAddress: "0x0000000000000000000000000000000000000001",
        toAddress: "0x0000000000000000000000000000000000000002",
        amount: "-100",
        expectedKey: "errors",
        expectedValue: "",
        expectedErrorMsg: "amount must be positive",
    }
    transferTest(args, initial_wallets)

}

func TestTransferNotNumeric(t *testing.T) {
    initial_wallets := []Wallet{
        {Address: "0x0000000000000000000000000000000000000001", Balance: decimal.NewFromInt(1000)},
        {Address: "0x0000000000000000000000000000000000000002", Balance: decimal.NewFromInt(500)},
    }

    args := transferTestArgs{
        t: t,
        fromAddress: "0x0000000000000000000000000000000000000001",
        toAddress: "0x0000000000000000000000000000000000000002",
        amount: "10q",
        expectedKey: "errors",
        expectedValue: "",
        expectedErrorMsg: "invalid amount format: can't convert 10q to decimal",
    }
    transferTest(args, initial_wallets)
}

func TestTransferFloat(t *testing.T) {
    initial_wallets := []Wallet{
        {Address: "0x0000000000000000000000000000000000000001", Balance: decimal.NewFromInt(1000)},
        {Address: "0x0000000000000000000000000000000000000002", Balance: decimal.NewFromInt(500)},
    }


    args := transferTestArgs{
        t: t,
        fromAddress: "0x0000000000000000000000000000000000000001",
        toAddress: "0x0000000000000000000000000000000000000002",
        amount: "100.7",
        expectedKey: "errors",
        expectedValue: "",
        expectedErrorMsg: "amount must be an integer (cant be floating point)",
    }
    transferTest(args, initial_wallets)
}

func TestTransferInsufficientBalance(t *testing.T) {
    initial_wallets := []Wallet{
        {Address: "0x0000000000000000000000000000000000000001", Balance: decimal.NewFromInt(500)},
        {Address: "0x0000000000000000000000000000000000000002", Balance: decimal.NewFromInt(300)},
    }

    args := transferTestArgs{
        t: t,
        fromAddress: "0x0000000000000000000000000000000000000001",
        toAddress: "0x0000000000000000000000000000000000000002",
        amount: "700",
        expectedKey: "errors",
        expectedValue: "",
        expectedErrorMsg: "insufficient balance",
    }
    transferTest(args, initial_wallets)
}

func TestTransferWrongSenderAddress(t *testing.T) {
    initial_wallets := []Wallet{
        {Address: "0x0000000000000000000000000000000000000001", Balance: decimal.NewFromInt(500)},
        {Address: "0x0000000000000000000000000000000000000002", Balance: decimal.NewFromInt(300)},
    }

    args := transferTestArgs{
        t: t,
        fromAddress: "0x0000000000000000000000000000000000000003",
        toAddress: "0x0000000000000000000000000000000000000002",
        amount: "100",
        expectedKey: "errors",
        expectedValue: "",
        expectedErrorMsg: "transfer fail: sender wallet not found",
    }
    transferTest(args, initial_wallets)
}


func TestTransferWrongReceiverAddress(t *testing.T) {
    initial_wallets := []Wallet{
        {Address: "0x0000000000000000000000000000000000000001", Balance: decimal.NewFromInt(500)},
        {Address: "0x0000000000000000000000000000000000000002", Balance: decimal.NewFromInt(300)},
    }

    args := transferTestArgs{
        t: t,
        fromAddress: "0x0000000000000000000000000000000000000001",
        toAddress: "0x0000000000000000000000000000000000000003",
        amount: "100",
        expectedKey: "errors",
        expectedValue: "",
        expectedErrorMsg: "transfer fail: recipient wallet not found",
    }
    transferTest(args, initial_wallets)
}

func TestTransferRaceCondition(t *testing.T) {
    
    initial_wallets := []Wallet{
        {Address: "0x0000000000000000000000000000000000000001", Balance: decimal.NewFromInt(10)},
        {Address: "0x0000000000000000000000000000000000000002", Balance: decimal.NewFromInt(1)},
        {Address: "0x0000000000000000000000000000000000000003", Balance: decimal.NewFromInt(0)},
    }
    
    transfers := []Transfer{
        {FromAddress: "0x0000000000000000000000000000000000000002", ToAddress: "0x0000000000000000000000000000000000000001", Amount: "1"},
        {FromAddress: "0x0000000000000000000000000000000000000001", ToAddress: "0x0000000000000000000000000000000000000002", Amount: "4"},
        {FromAddress: "0x0000000000000000000000000000000000000001", ToAddress: "0x0000000000000000000000000000000000000003", Amount: "7"},
    }

    raceConditionsTest(t, initial_wallets, transfers)
}


func TestTransferRaceConditionManySendersAndReceivers(t *testing.T) {
    initial_wallets := []Wallet{
        {Address: "0x0000000000000000000000000000000000000001", Balance: decimal.NewFromInt(1000000)},
        {Address: "0x0000000000000000000000000000000000000002", Balance: decimal.NewFromInt(3000000)},
        {Address: "0x0000000000000000000000000000000000000003", Balance: decimal.NewFromInt(5000000)},
    }

    transfers := []Transfer{
            {FromAddress: "0x0000000000000000000000000000000000000003", ToAddress: "0x0000000000000000000000000000000000000002", Amount: "2000000"},
            {FromAddress: "0x0000000000000000000000000000000000000003", ToAddress: "0x0000000000000000000000000000000000000001", Amount: "3000000"},
            {FromAddress: "0x0000000000000000000000000000000000000001", ToAddress: "0x0000000000000000000000000000000000000003", Amount: "2000000"},
            {FromAddress: "0x0000000000000000000000000000000000000002", ToAddress: "0x0000000000000000000000000000000000000001", Amount: "4000000"},
        }

    raceConditionsTest(t, initial_wallets, transfers)
}