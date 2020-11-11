package main

import (
	"context"
	"fmt"
)

func main() {
	ctx := context.Background()
	transfer, err := New(ctx)
	if err != nil {
		fmt.Printf("%#v\n", err)
		return
	}
	if err := transfer.Exec(ctx); err != nil {
		fmt.Printf("%#v\n", err)
	}
}
