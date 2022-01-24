package interactivebrokers

import (
	"fmt"
	"github.com/hadrianl/ibapi"
	"testing"
)

const gateway = "127.0.0.1"
const port = 7497
const clientID = 100

func NotTest_GetContract(t *testing.T) {
	ibClient, err := NewIbClientConnector(gateway, port, clientID, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ibClient.Close()
	})

	amzn, err := ibClient.ReqContractDetails(ibapi.Contract{
		Symbol:       "TSLA",
		SecurityType: "STK",
		Currency:     "USD",
		Exchange:     "SMART",
	})

	println(fmt.Sprintf("%v", amzn))

}

func TestGetContracts(t *testing.T) {

	ibClient, err := NewIbClientConnector(gateway, port, clientID, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ibClient.Close()
	})

	contractsAmzn, err := ibClient.ReqContractDetails(ibapi.Contract{
		Symbol:       "AMZN",
		SecurityType: "STK",
		Currency:     "USD",
		Exchange:     "SMART",
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(contractsAmzn) != 1 {
		t.Fatalf("expecgted 1, got %v contracts ", len(contractsAmzn))
	}

	if contractsAmzn[0].LongName != "AMAZON.COM INC" ||
		contractsAmzn[0].Contract.ContractID != 3691937 {
		t.Fatal("contract mismatch")
	}

	// This will go in error
	_, err = ibClient.ReqContractDetails(ibapi.Contract{
		Symbol: "AMZN",
	})
	if err == nil {
		t.Error("security is mandatory for ReqContractDetails")
	}

	contractsTslaAll, err := ibClient.ReqContractDetails(ibapi.Contract{
		Symbol:       "TSLA",
		SecurityType: "STK",
	})
	if err != nil {
		t.Error(err)
	}
	if len(contractsTslaAll) < 3 {
		t.Errorf("Expected at least 3 contracts, got %v", len(contractsTslaAll))
	}
}
