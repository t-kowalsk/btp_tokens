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

func SetTwoWallets(firstAddress, secondAddress string, senderBalance, receiverBalance decimal.Decimal) {
    _, _ = database.Db.Exec(`
        INSERT INTO wallets (address, balance) VALUES
        ($1, $2) 
        ON CONFLICT (address) DO UPDATE SET balance = $2
    `, firstAddress, senderBalance)
    _, _ = database.Db.Exec(`
        INSERT INTO wallets (address, balance) VALUES
        ($1, $2)
        ON CONFLICT (address) DO UPDATE SET balance = $2
    `, secondAddress, receiverBalance)
}

func SetUpTestTwoWallets(t *testing.T, firstAddress, secondAddress string, senderBalance, receiverBalance decimal.Decimal) (*sql.DB, *httptest.Server) {
    db := setupTestDB(t)
    ResetTestDB()
    SetTwoWallets(firstAddress, secondAddress, senderBalance, receiverBalance)

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
    senderBalance string
    receiverBalance string
    amount string
    expectedKey string
    expectedValue string
    expectedErrorMsg string
}

func transferTest(args transferTestArgs) {
    decimalSenderBalance, _ := decimal.NewFromString(args.senderBalance)
    decimalReceiverBalance, _ := decimal.NewFromString(args.receiverBalance)
    _, server := SetUpTestTwoWallets(args.t, args.fromAddress, args.toAddress,  decimalSenderBalance, decimalReceiverBalance)
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
    args := transferTestArgs{
        t: t,
        fromAddress: "0x0000000000000000000000000000000000000001",
        toAddress: "0x0000000000000000000000000000000000000002",
        senderBalance: "10",
        receiverBalance: "15",
        amount: "6",
        expectedKey: "transfer",
        expectedValue: "4",
        expectedErrorMsg: "",
    }
    transferTest(args)

}

func TestTransferNegative(t *testing.T) {
    args := transferTestArgs{
        t: t,
        fromAddress: "0x0000000000000000000000000000000000000001",
        toAddress: "0x0000000000000000000000000000000000000002",
        senderBalance: "1000",
        receiverBalance: "500",
        amount: "-100",
        expectedKey: "errors",
        expectedValue: "",
        expectedErrorMsg: "Amount must be positive",
    }
    transferTest(args)

}
