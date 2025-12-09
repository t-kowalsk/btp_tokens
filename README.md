# BTP Tokens Transfer API
This is a GraphQL API for transferring BTP tokens between wallets.

## Setup:
Clone the repository:
> git clone https://github.com/t-kowalsk/btp_tokens.git

Download project dependecies:
> go mod download

Compose and run docker:
> docker-compose up -d

After these steps the application is set up.

## Run application
To run the application:
> go run .\server.go

To run application tests:
> go test ./test/... -v

## Transfer schema:
Project's GraphQL has single transfer mutation that is called according to the schema, for example:
```
mutation {
  transfer(input: {
      from_address: "0x0000000000000000000000000000000000000001",
      to_address: "0x0000000000000000000000000000000000000002",
      amount: "200000"
  })
}
```
This transfer 200000 BTP tokens from wallet with 0x0000000000000000000000000000000000000001 address to the wallet with 0x0000000000000000000000000000000000000002 address if the wallet that the tokens are pulled has a sufficient balance (balance of at least 200000 BTP tokens). Otherwise "insufficient balance" error is thrown.