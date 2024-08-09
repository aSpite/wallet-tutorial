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
	destinationAddress := address.MustParseAddr("put your wallet where you want to send NFT")
	walletAddress := address.MustParseAddr("put your wallet which is the owner of NFT")
	nftAddress := address.MustParseAddr("put your nft address")

	// We can add a comment, but it will not be displayed in the explorers,
	// as it is not supported by them at the time of writing the tutorial.
	forwardPayload := cell.BeginCell().
		MustStoreUInt(0, 32).
		MustStoreStringSnake("Hello, TON!").
		EndCell()

	transferNftBody := cell.BeginCell().
		MustStoreUInt(0x5fcc3d14, 32).                        // Opcode for NFT transfer
		MustStoreUInt(0, 64).                                 // query_id
		MustStoreAddr(destinationAddress).                    // new_owner
		MustStoreAddr(walletAddress).                         // response_destination for excesses
		MustStoreBoolBit(false).                              // we do not have custom_payload
		MustStoreBigCoins(tlb.MustFromTON("0.01").NanoTON()). // forward_payload
		MustStoreBoolBit(true).                               // we store forward_payload as a reference
		MustStoreRef(forwardPayload).                         // store forward_payload as a reference
		EndCell()

	internalMessage := cell.BeginCell().
		MustStoreUInt(0x18, 6). // bounce
		MustStoreAddr(nftAddress).
		MustStoreBigCoins(tlb.MustFromTON("0.05").NanoTON()).
		MustStoreUInt(1, 1+4+4+64+32+1+1). // We store 1 that means we have body as a reference
		MustStoreRef(transferNftBody).
		EndCell()

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
		MustStoreUInt(seqno.Uint64(), 32).                     // store seqno
		// Do not forget that if we use Wallet V4, we need to add .MustStoreUInt(0, 8)
		MustStoreUInt(3, 8).          // store mode of our internal message
		MustStoreRef(internalMessage) // store our internalMessage as a reference

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
