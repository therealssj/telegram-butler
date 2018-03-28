package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/kittycash/telegram-butler/src/auction_butler"
	_ "github.com/lib/pq"
)

func loadJsonFromFile(filename string, result interface{}) error {
	infile, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf(
			"failed to open json from file '%s': %v",
			filename, err,
		)
	}
	defer infile.Close()

	decoder := json.NewDecoder(infile)
	if err := decoder.Decode(result); err != nil {
		return fmt.Errorf(
			"failed to decode json from file '%s': %v",
			filename, err,
		)
	}

	return nil
}

func main() {
	var config auction_butler.Config
	if err := loadJsonFromFile("config.json", &config); err != nil {
		panic(err)
	}
	bot, err := auction_butler.NewBot(config)
	if err != nil {
		panic(err)
	}

	bot.Start()
}
