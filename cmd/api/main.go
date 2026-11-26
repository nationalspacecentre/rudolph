package main

import (
	"log/slog"
	"os"

	"github.com/airbnb/rudolph/internal/handlers"
	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	lambda.Start(handlers.ApiRouter)
}
