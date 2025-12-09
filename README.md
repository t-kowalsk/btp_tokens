# BTP Tokens Transfer API
This is a GraphQL API for transferring BTP tokens between wallets.

## Setup:
Clone the repository:
```
git clone https://github.com/t-kowalsk/btp_tokens.git
```

Download project dependecies:
```
go mod download
```

Compose and run docker:
```
docker-compose up -d
```

The composed container named: **postgres-btp**, runs on port: **5432**, with password: **dbpass**. Compose creates two databases: **btp_tokens** and **btp_tokens_test**.

## Run application
To run the GraphQL server:
```
go run .\server.go
```

The server runs on http://localhost:8080/ address.

To run application tests (*run with -v for more details*):
```
go test ./test/...
```

## Transfer schema and examples:
Initially there is one wallet with address: **"0x0000000000000000000000000000000000000000"** and balance of **1000000 BTP** tokens.

The project's GraphQL has single transfer mutation that is called according to the schema:
```
input Transfer {
  from_address: String!
  to_address: String!
  amount: String!
}
```

Transfer mutation example:
```
mutation {
  transfer(input: {
      from_address: "0x0000000000000000000000000000000000000000",
      to_address: "0x0000000000000000000000000000000000000001",
      amount: "200000"
  })
}
```
This transfers 200000 BTP tokens from wallet with 0x0000000000000000000000000000000000000000 address to the wallet with 0x0000000000000000000000000000000000000001 address and returns updated balance of the sender (as a String). This happens only if the wallet that the tokens are pulled from has a sufficient balance (balance of at least 200000 BTP tokens). Otherwise "insufficient balance" error message is returned. The transferred value has to be a positive non-floating-point number. If the receiving wallet's address does not point to an existing wallet in Wallets table, a new record is created with that address and a balance equal to the transferred amount.