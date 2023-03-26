package main

import (
	"context"
	"github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-go/liteclient"
	"github.com/xssnick/tonutils-go/ton"
	"log"
	"math/big"
)

func main() {
	connection := liteclient.NewConnectionPool()
	configUrl := "https://ton-blockchain.github.io/global.config.json"
	err := connection.AddConnectionsFromConfigUrl(context.Background(), configUrl)
	if err != nil {
		panic(err)
	}
	client := ton.NewAPIClient(connection)

	block, err := client.CurrentMasterchainInfo(context.Background()) // get current block, we will need it in requests to LiteServer
	if err != nil {
		log.Fatalln("CurrentMasterchainInfo err:", err.Error())
		return
	}

	walletAddress := address.MustParseAddr("EQDKbjIcfM6ezt8KjKJJLshZJJSqX7XOA4ff-W72r5gqPrHF") // my wallet address as an example

	getResult, err := client.RunGetMethod(context.Background(), block, walletAddress, "get_public_key") // run get_public_key GET Method
	if err != nil {
		log.Fatalln("RunGetMethod err:", err.Error())
		return
	}

	// We have a response as an array with values and should specify the index when reading it
	// In the case of get_public_key, we have only one returned value that is stored at 0 index
	publicKeyUInt := getResult.MustInt(0) // read answer that contains uint256
	publicKey := publicKeyUInt.Text(16)   // get hex string from bigint (uint256)
	log.Println(publicKey)

	oldWalletAddress := address.MustParseAddr("EQAM7M--HGyfxlErAIUODrxBA3yj5roBeYiTuy6BHgJ3Sx8k")
	subscriptionAddress := address.MustParseAddr("EQBTKTis-SWYdupy99ozeOvnEBu8LRrQP_N9qwOTSAy3sQSZ") // subscription plugin address which is already installed on the wallet

	hash := big.NewInt(0).SetBytes(subscriptionAddress.Data())
	// runGetMethod will automatically identify types of passed values
	getResult, err = client.RunGetMethod(context.Background(), block, oldWalletAddress,
		"is_plugin_installed",
		0,    // pass workchain
		hash) // pass plugin address
	if err != nil {
		log.Fatalln("RunGetMethod err:", err.Error())
		return
	}

	log.Println(getResult.MustInt(0)) // -1
}
