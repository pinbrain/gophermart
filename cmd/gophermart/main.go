package main

import (
	"fmt"

	"github.com/pinbrain/gophermart/internal/app"
)

func main() {
	fmt.Println("~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~ starting app")
	if err := app.Run(); err != nil {
		panic(err)
	}
}
