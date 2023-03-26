package main

import (
	"context"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha512"
	"fmt"
	"github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-go/liteclient"
	"github.com/xssnick/tonutils-go/tl"
	"github.com/xssnick/tonutils-go/tlb"
	"github.com/xssnick/tonutils-go/ton"
	"github.com/xssnick/tonutils-go/tvm/cell"
	"golang.org/x/crypto/pbkdf2"
	"log"
	"math/big"
	"math/rand"
	"strings"
	"time"
)

func main() {
	var internalMessages []*cell.Cell
	wallletAddress := address.MustParseAddr("Eput your wallet address from which you deployed high-load wallet")

	for i := 0; i < 12; i++ {
		comment := fmt.Sprintf("Hello, TON! #%d", i)
		internalMessageBody := cell.BeginCell().
			MustStoreUInt(0, 32).
			MustStoreBinarySnake([]byte(comment)).
			EndCell()

		internalMessage := cell.BeginCell().
			MustStoreUInt(0x18, 6). // bounce
			MustStoreAddr(wallletAddress).
			MustStoreBigCoins(tlb.MustFromTON("0.001").NanoTON()).
			MustStoreUInt(0, 1+4+4+64+32).
			MustStoreBoolBit(false).           // We do not have State Init
			MustStoreBoolBit(true).            // We store Message Body as a reference
			MustStoreRef(internalMessageBody). // Store Message Body Init as a reference
			EndCell()

		messageData := cell.BeginCell().
			MustStoreUInt(3, 8). // transaction mode
			MustStoreRef(internalMessage).
			EndCell()

		internalMessages = append(internalMessages, messageData)
	}

	dictionary := cell.NewDict(16) // create an empty dictionary with the key as a number and the value as a cell
	for i := 0; i < len(internalMessages); i++ {
		internalMessage := internalMessages[i]                             // get our message from an array
		err := dictionary.SetIntKey(big.NewInt(int64(i)), internalMessage) // save the message in the dictionary
		if err != nil {
			return
		}
	}

	queryID := rand.Uint32()
	timeout := 120                                                               // timeout for message expiration, 120 seconds = 2 minutes
	now := time.Now().Add(time.Duration(timeout)*time.Second).UTC().Unix() << 32 // get current timestamp + timeout
	finalQueryID := uint64(now) + uint64(queryID)                                // get our final query_id
	log.Println(finalQueryID)                                                    // print query_id. With this query_id we can call GET method to check if our request has been processed

	toSign := cell.BeginCell().
		MustStoreUInt(698983191, 32). // subwallet_id
		MustStoreUInt(finalQueryID, 64).
		MustStoreDict(dictionary)

	highloadMnemonicArray := strings.Split("put your high-load wallet mnemonic", " ") // word1 word2 word3
	mac := hmac.New(sha512.New, []byte(strings.Join(highloadMnemonicArray, " ")))
	hash := mac.Sum(nil)
	k := pbkdf2.Key(hash, []byte("TON default seed"), 100000, 32, sha512.New) // In TON libraries "TON default seed" is used as salt when getting keys
	// 32 is a key len
	highloadPrivateKey := ed25519.NewKeyFromSeed(k) // get private key
	highloadWalletAddress := address.MustParseAddr("put your high-load wallet address")

	signature := ed25519.Sign(highloadPrivateKey, toSign.EndCell().Hash())

	body := cell.BeginCell().
		MustStoreSlice(signature, 512). // store signature
		MustStoreBuilder(toSign).       // store our message
		EndCell()

	externalMessage := cell.BeginCell().
		MustStoreUInt(0b10, 2).               // ext_in_msg_info$10
		MustStoreUInt(0, 2).                  // src -> addr_none
		MustStoreAddr(highloadWalletAddress). // Destination address
		MustStoreCoins(0).                    // Import Fee
		MustStoreBoolBit(false).              // No State Init
		MustStoreBoolBit(true).               // We store Message Body as a reference
		MustStoreRef(body).                   // Store Message Body as a reference
		EndCell()

	connection := liteclient.NewConnectionPool()
	configUrl := "https://ton-blockchain.github.io/global.config.json"
	err := connection.AddConnectionsFromConfigUrl(context.Background(), configUrl)
	if err != nil {
		panic(err)
	}
	client := ton.NewAPIClient(connection)

	var resp tl.Serializable
	err = client.Client().QueryLiteserver(context.Background(), ton.SendMessage{Body: externalMessage.ToBOCWithFlags(false)}, &resp)

	if err != nil {
		log.Fatalln(err.Error())
		return
	}
}
