package main

import (
	"context"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha512"
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
	internalMessagesAmount := [4]string{"0.01", "0.02", "0.03", "0.04"}
	internalMessagesComment := [4]string{
		"Hello, TON! #1",
		"Hello, TON! #2",
		"", // Let's leave the third message without comment
		"Hello, TON! #4",
	}
	destinationAddresses := [4]string{
		"Put any address that belongs to you",
		"Put any address that belongs to you",
		"Put any address that belongs to you",
		"Put any address that belongs to you",
	} // All 4 addresses can be the same

	var internalMessages [len(internalMessagesAmount)]*cell.Cell // array for our internal messages

	for i := 0; i < len(internalMessagesAmount); i++ {
		amount := internalMessagesAmount[i]

		internalMessage := cell.BeginCell().
			MustStoreUInt(0x18, 6). // bounce
			MustStoreAddr(address.MustParseAddr(destinationAddresses[i])).
			MustStoreBigCoins(tlb.MustFromTON(amount).NanoTON()).
			MustStoreUInt(0, 1+4+4+64+32+1)

		/*
		   At this stage, it is not clear if we will have a message body.
		   So put a bit only for stateInit, and if we have a comment, in means
		   we have a body message. In that case, set the bit to 1 and store the
		   body as a reference.
		*/

		if internalMessagesComment[i] != "" {
			internalMessage.MustStoreBoolBit(true) // we store Message Body as a reference

			internalMessageBody := cell.BeginCell().
				MustStoreUInt(0, 32).
				MustStoreStringSnake(internalMessagesComment[i]).
				EndCell()

			internalMessage.MustStoreRef(internalMessageBody)
		} else {
			/*
			   Since we do not have a message body, we indicate that
			   the message body is in this message, but do not write it,
			   which means it is absent. In that case, just set the bit to 0.
			*/
			internalMessage.MustStoreBoolBit(false)
		}
		internalMessages[i] = internalMessage.EndCell()
	}

	walletAddress := address.MustParseAddr("put your wallet address")

	connection := liteclient.NewConnectionPool()
	configUrl := "https://ton-blockchain.github.io/global.config.json"
	err := connection.AddConnectionsFromConfigUrl(context.Background(), configUrl)
	if err != nil {
		panic(err)
	}
	client := ton.NewAPIClient(connection)

	mnemonic := strings.Split("put your mnemonic", " ") // word1 word2 word3
	// The following three lines will extract the private key using the mnemonic phrase.
	// We will not go into cryptographic details. In the library tonutils-go, it is all implemented,
	// but it immediately returns the finished object of the wallet with the address and ready-made methods.
	// So we’ll have to write the lines to get the key separately. Goland IDE will automatically import
	// all required libraries (crypto, pbkdf2 and others).
	mac := hmac.New(sha512.New, []byte(strings.Join(mnemonic, " ")))
	hash := mac.Sum(nil)
	k := pbkdf2.Key(hash, []byte("TON default seed"), 100000, 32, sha512.New) // In TON libraries "TON default seed" is used as salt when getting keys
	// 32 is a key len
	privateKey := ed25519.NewKeyFromSeed(k) // get private key

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

	toSign := cell.BeginCell().
		MustStoreUInt(698983191, 32).                          // subwallet_id | We consider this further
		MustStoreUInt(uint64(time.Now().UTC().Unix()+60), 32). // message expiration time, +60 = 1 minute
		MustStoreUInt(seqno.Uint64(), 32)                      // store seqno
	// Do not forget that if we use Wallet V4, we need to add .MustStoreUInt(0, 8)

	for i := 0; i < len(internalMessages); i++ {
		internalMessage := internalMessages[i]
		toSign.MustStoreUInt(3, 8)           // store mode of our internal message
		toSign.MustStoreRef(internalMessage) // store our internalMessage as a reference
	}

	signature := ed25519.Sign(privateKey, toSign.EndCell().Hash()) // get the hash of our message to wallet smart contract and sign it to get signature

	body := cell.BeginCell().
		MustStoreSlice(signature, 512). // store signature
		MustStoreBuilder(toSign).       // store our message
		EndCell()

	externalMessage := cell.BeginCell().
		MustStoreUInt(0b10, 2).       // ext_in_msg_info$10
		MustStoreUInt(0, 2).          // src -> addr_none
		MustStoreAddr(walletAddress). // Destination address
		MustStoreCoins(0).            // Import Fee
		MustStoreBoolBit(false).      // No State Init
		MustStoreBoolBit(true).       // We store Message Body as a reference
		MustStoreRef(body).           // Store Message Body as a reference
		EndCell()

	var resp tl.Serializable
	err = client.Client().QueryLiteserver(context.Background(), ton.SendMessage{Body: externalMessage.ToBOCWithFlags(false)}, &resp)

	if err != nil {
		log.Fatalln(err.Error())
		return
	}
}
