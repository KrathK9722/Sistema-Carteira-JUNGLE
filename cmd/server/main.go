package main

import (
	"example.com/financial-service/internal/composition"
	"go.uber.org/fx"
	"log/slog"
	"os"
	"time"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	fx.New(composition.Module, fx.StartTimeout(60*time.Second), fx.StopTimeout(30*time.Second)).Run()
}
