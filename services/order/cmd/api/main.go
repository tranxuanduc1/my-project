package main

import (
	"log"
	"os"

	"myproject/order/internal/bootstrap"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		if err := bootstrap.Migrate(); err != nil {
			log.Fatal(err)
		}
		return
	}
	if err := bootstrap.Run(); err != nil {
		log.Fatal(err)
	}
}
