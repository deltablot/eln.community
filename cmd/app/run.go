package main

import (
	"log"

	app "deltablot/partage/internal"
)

func main() {
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
