package main

import (
	"context"
	"os"

	"github.com/gqlgo/gqlgenc /config"
	"github.com/gqlgo/gqlgenc /generator"
	"github.com/rs/zerolog/log"
)

const (
	graphapiGenDir = "./"
)

func main() {
	cfg, err := config.LoadConfig(graphapiGenDir + ".gqlgenc.yml")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load config")
		os.Exit(2)
	}

	if err := generator.Generate(context.Background(), cfg); err != nil {
		log.Error().Err(err).Msg("Failed to generate gqlgenc client")
	}
}
