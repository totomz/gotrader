package main

import (
	"testing"
	"time"
)

func TestEpochTs(t *testing.T) {
	input := "2021-01-11 15:53:28"
	daje, err := time.Parse("2006-01-02 15:04:05", input)
	if err != nil {
		panic(err)
	}

	println(daje.UnixMilli())
}
