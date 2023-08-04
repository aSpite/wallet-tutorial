package main

import (
	"context"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/base64"
	"github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-go/liteclient"
	"github.com/xssnick/tonutils-go/tl"
	"github.com/xssnick/tonutils-go/tlb"
	"github.com/xssnick/tonutils-go/ton"
	"github.com/xssnick/tonutils-go/ton/wallet"
	"github.com/xssnick/tonutils-go/tvm/cell"
	"golang.org/x/crypto/pbkdf2"
	"log"
	"strings"
	"time"
)

func main() {
	// mnemonic := strings.Split("put your mnemonic", " ") // get our mnemonic as array
	mnemonic := wallet.NewSeed() // get new mnemonic

	// The following three lines will extract the private key using the mnemonic phrase. We will not go into cryptographic details. It has all been implemented in the tonutils-go library, but it immediately returns the finished object of the wallet with the address and ready methods. So we’ll have to write the lines to get the key separately. Goland IDE will automatically import all required libraries (crypto, pbkdf2 and others).
	mac := hmac.New(sha512.New, []byte(strings.Join(mnemonic, " ")))
	hash := mac.Sum(nil)
	k := pbkdf2.Key(hash, []byte("TON default seed"), 100000, 32, sha512.New) // In TON libraries "TON default seed" is used as salt when getting keys
	// 32 is a key len

	privateKey := ed25519.NewKeyFromSeed(k)              // get private key
	publicKey := privateKey.Public().(ed25519.PublicKey) // get public key from private key
	log.Println(publicKey)                               // print publicKey so that at this stage the compiler does not complain that we do not use our variable
	log.Println(mnemonic)                                // if we want, we can print our mnemonic

	var subWallet uint64 = 698983191

	base64BOC := "te6ccgEBCAEAhgABFP8A9KQT9LzyyAsBAgEgAgMCAUgEBQCW8oMI1xgg0x/TH9MfAvgju/Jj7UTQ0x/TH9P/0VEyuvKhUUS68qIE+QFUEFX5EPKj+ACTINdKltMH1AL7AOgwAaTIyx/LH8v/ye1UAATQMAIBSAYHABe7Oc7UTQ0z8x1wv/gAEbjJftRNDXCx+A==" // save our base64 encoded output from compiler to variable
	codeCellBytes, _ := base64.StdEncoding.DecodeString(base64BOC)                                                                                                                                                      // decode base64 in order to get byte array
	codeCell, err := cell.FromBOC(codeCellBytes)                                                                                                                                                                        // get cell with code from byte array
	if err != nil {                                                                                                                                                                                                     // check if there are any error
		panic(err)
	}

	log.Println("Hash:", base64.StdEncoding.EncodeToString(codeCell.Hash())) // get the hash of our cell, encode it to base64 because it has []byte type and output to the terminal

	dataCell := cell.BeginCell().
		MustStoreUInt(0, 32).           // Seqno
		MustStoreUInt(698983191, 32).   // Subwallet ID
		MustStoreSlice(publicKey, 256). // Public Key
		EndCell()

	stateInit := cell.BeginCell().
		MustStoreBoolBit(false). // No split_depth
		MustStoreBoolBit(false). // No special
		MustStoreBoolBit(true).  // We have code
		MustStoreRef(codeCell).
		MustStoreBoolBit(true). // We have data
		MustStoreRef(dataCell).
		MustStoreBoolBit(false). // No library
		EndCell()

	contractAddress := address.NewAddress(0, 0, stateInit.Hash()) // get the hash of stateInit to get the address of our smart contract in workchain with ID 0
	log.Println("Contract address:", contractAddress.String())    // Output contract address to console

	internalMessageBody := cell.BeginCell().
		MustStoreUInt(0, 32).
		MustStoreStringSnake("Hello, TON!").
		EndCell()

	internalMessage := cell.BeginCell().
		MustStoreUInt(0x10, 6). // no bounce
		MustStoreAddr(address.MustParseAddr("put your first wallet address from were you sent 0.1 TON")).
		MustStoreBigCoins(tlb.MustFromTON("0.03").NanoTON()).
		MustStoreUInt(1, 1+4+4+64+32+1+1). // We store 1 that means we have body as a reference
		MustStoreRef(internalMessageBody).
		EndCell()

	// transaction for our wallet
	toSign := cell.BeginCell().
		MustStoreUInt(subWallet, 32).
		MustStoreUInt(uint64(time.Now().UTC().Unix()+60), 32).
		MustStoreUInt(0, 32). // We put seqno = 0, because after deploying wallet will store 0 as seqno
		MustStoreUInt(3, 8).
		MustStoreRef(internalMessage)

	signature := ed25519.Sign(privateKey, toSign.EndCell().Hash())
	body := cell.BeginCell().
		MustStoreSlice(signature, 512).
		MustStoreBuilder(toSign).
		EndCell()

	externalMessage := cell.BeginCell().
		MustStoreUInt(0b10, 2). // indicate that it is an incoming external transaction
		MustStoreUInt(0, 2).    // src -> addr_none
		MustStoreAddr(contractAddress).
		MustStoreCoins(0).       // Import fee
		MustStoreBoolBit(true).  // We have State Init
		MustStoreBoolBit(true).  // We store State Init as a reference
		MustStoreRef(stateInit). // Store State Init as a reference
		MustStoreBoolBit(true).  // We store Message Body as a reference
		MustStoreRef(body).      // Store Message Body as a reference
		EndCell()

	connection := liteclient.NewConnectionPool()
	configUrl := "https://ton-blockchain.github.io/global.config.json"
	err = connection.AddConnectionsFromConfigUrl(context.Background(), configUrl)
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
