package main

import (
	"github.com/pinbrain/gophermart/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		panic(err)
	}
}
