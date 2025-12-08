package test

import (
	"btp_tokens/graph"
	database "btp_tokens/internal/pkg/db/migrations/postgres"
	"btp_tokens/internal/wallets"
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/shopspring/decimal"
)


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

type Wallet struct {
	Address string
	Balance decimal.Decimal
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

// func assertStringsEqual(t *testing.T, stringOne string, stringTwo string){
// }

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
    // decimalSenderBalance, _ := decimal.NewFromString(args.senderBalance)
    // decimalReceiverBalance, _ := decimal.NewFromString(args.receiverBalance)
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
    // respValue := transferResponse["data"].(map[string]interface{})[args.expectedKey]

    if args.expectedErrorMsg != "" {
        assertGraphQLError(args.t, transferResponse, args.expectedErrorMsg)
    } else {

        respValue := transferResponse["data"].(map[string]interface{})[args.expectedKey]
        if respValue != args.expectedValue {
            args.t.Fatalf("expected %s, got %s", args.expectedValue, respValue)
        }
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