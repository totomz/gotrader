package main

import (
	"github.com/hadrianl/ibapi"
	ibdriver "github.com/totomz/gotrader/gotrader/interactivebrokers"
	"golang.org/x/exp/slog"
	"os"
)

const gateway = "ibgw.dc-cantina.my-ideas.it"
const port = 7496
const clientID = 100

var (
	stdout = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{AddSource: true}))
)

func main() {
	println("bella zio")

	ibClient, err := ibdriver.NewIbClientConnector(gateway, port, clientID)
	if err != nil {
		stdout.Error("can't open ibconnection", "error", err)
		os.Exit(1)
	}

	contracts, err := ibClient.ReqContractDetails(ibapi.Contract{
		Symbol:       "AMZN",
		SecurityType: "STK",
		Currency:     "USD",
		Exchange:     "SMART",
	})
	if err != nil {
		stdout.Error("can't get contract", "error", err)
		os.Exit(1)
	}

	for _, c := range contracts {
		stdout.Info("found contract", "contract", c.LongName)
	}

}
