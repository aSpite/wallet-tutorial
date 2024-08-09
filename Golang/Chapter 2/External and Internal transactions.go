package main

import (
	"context"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/base64"
	"log"
	"strings"
	"time"

	"github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-go/liteclient"
	"github.com/xssnick/tonutils-go/tl"
	"github.com/xssnick/tonutils-go/tlb"
	"github.com/xssnick/tonutils-go/ton"
	"github.com/xssnick/tonutils-go/tvm/cell"
	"golang.org/x/crypto/pbkdf2"
)

func main() {
	internalMessageBody := cell.BeginCell().
		MustStoreUInt(0, 32).                // write 32 zero bits to indicate that a text comment will follow
		MustStoreStringSnake("Hello, TON!"). // write our text comment
		EndCell()

	walletAddress := address.MustParseAddr("put your address")

	internalMessage := cell.BeginCell().
		MustStoreUInt(0, 1).     // indicate that it is an internal message -> int_msg_info$0
		MustStoreBoolBit(true).  // IHR Disabled
		MustStoreBoolBit(true).  // bounce
		MustStoreBoolBit(false). // bounced
		MustStoreUInt(0, 2).     // src -> addr_none
		MustStoreAddr(walletAddress).
		MustStoreCoins(tlb.MustFromTON("0.2").NanoTON().Uint64()). // amount
		MustStoreBoolBit(false).                                   // Extra currency
		MustStoreCoins(0).                                         // IHR Fee
		MustStoreCoins(0).                                         // Forwarding Fee
		MustStoreUInt(0, 64).                                      // Logical time of creation
		MustStoreUInt(0, 32).                                      // UNIX time of creation
		MustStoreBoolBit(false).                                   // No State Init
		MustStoreBoolBit(true).                                    // We store Message Body as a reference
		MustStoreRef(internalMessageBody).                         // Store Message Body as a reference
		EndCell()

	mnemonic := strings.Split("put your mnemonic", " ") // get our mnemonic as array

	connection := liteclient.NewConnectionPool()
	configUrl := "https://ton-blockchain.github.io/global.config.json"
	err := connection.AddConnectionsFromConfigUrl(context.Background(), configUrl)
	if err != nil {
		panic(err)
	}
	client := ton.NewAPIClient(connection) // create client

	block, err := client.CurrentMasterchainInfo(context.Background()) // get current block, we will need it in requests to LiteServer
	if err != nil {
		log.Fatalln("CurrentMasterchainInfo err:", err.Error())
		return
	}

	getMethodResult, err := client.RunGetMethod(context.Background(), block, walletAddress, "seqno") // run "seqno" GET method from your wallet contract
	if err != nil {
		log.Fatalln("RunGetMethod err:", err.Error())
		return
	}
	seqno := getMethodResult.MustInt(0) // get seqno from response

	// The next three lines will extract the private key using the mnemonic phrase. We will not go into cryptographic details. With the tonutils-go library, this is all implemented, but we’re doing it again to get a full understanding.
	mac := hmac.New(sha512.New, []byte(strings.Join(mnemonic, " ")))
	hash := mac.Sum(nil)
	k := pbkdf2.Key(hash, []byte("TON default seed"), 100000, 32, sha512.New) // In TON libraries "TON default seed" is used as salt when getting keys

	privateKey := ed25519.NewKeyFromSeed(k)

	toSign := cell.BeginCell().
		MustStoreUInt(698983191, 32).                          // subwallet_id | We consider this further
		MustStoreUInt(uint64(time.Now().UTC().Unix()+60), 32). // Message expiration time, +60 = 1 minute
		MustStoreUInt(seqno.Uint64(), 32).                     // store seqno
		MustStoreUInt(uint64(3), 8).                           // store mode of our internal message
		MustStoreRef(internalMessage)                          // store our internalMessage as a reference

	signature := ed25519.Sign(privateKey, toSign.EndCell().Hash()) // get the hash of our message to wallet smart contract and sign it to get signature

	body := cell.BeginCell().
		MustStoreSlice(signature, 512). // store signature
		MustStoreBuilder(toSign).       // store our message
		EndCell()

	externalMessage := cell.BeginCell().
		MustStoreUInt(0b10, 2).       // 0b10 -> 10 in binary
		MustStoreUInt(0, 2).          // src -> addr_none
		MustStoreAddr(walletAddress). // Destination address
		MustStoreCoins(0).            // Import Fee
		MustStoreBoolBit(false).      // No State Init
		MustStoreBoolBit(true).       // We store Message Body as a reference
		MustStoreRef(body).           // Store Message Body as a reference
		EndCell()

	log.Println(base64.StdEncoding.EncodeToString(externalMessage.ToBOCWithFlags(false)))

	var resp tl.Serializable
	err = client.Client().QueryLiteserver(context.Background(), ton.SendMessage{Body: externalMessage.ToBOCWithFlags(false)}, &resp)

	if err != nil {
		log.Fatalln(err.Error())
		return
	}
}
