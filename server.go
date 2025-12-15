package main

import (
	"btp_tokens/graph"
	"log"
	"net/http"
	"os"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/go-chi/chi/v5"

	database "btp_tokens/internal/pkg/db/migrations/postgres"
	"btp_tokens/internal/wallets"

	"github.com/joho/godotenv"
)

const defaultPort = "8080"
const dbURLKey = "DATABASE_URL"

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("cant load .env")
	}

	dbURL := os.Getenv(dbURLKey)
	if dbURL == "" {
		log.Fatalf("error: couldnt get database url variable")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	router := chi.NewRouter()

	database.InitDB(dbURL)
	db := database.Db
	defer database.CloseDB()
	database.Migrate("internal/pkg/db/migrations/postgres")

	walletsService := &wallets.WalletsService{DB: db}



	srv := handler.NewDefaultServer(graph.NewExecutableSchema(graph.Config{Resolvers: &graph.Resolver{ WalletsService: walletsService}}))


	router.Handle("/", playground.Handler("GraphQL playground", "/query"))
	router.Handle("/query", srv)

	log.Printf("connect to http://localhost:%s/ for GraphQL playground", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
}
