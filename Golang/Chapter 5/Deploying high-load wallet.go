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
	"github.com/xssnick/tonutils-go/tvm/cell"
	"golang.org/x/crypto/pbkdf2"
	"log"
	"strings"
	"time"
)

func main() {
	base64BOC := "te6ccgEBCQEA5QABFP8A9KQT9LzyyAsBAgEgAgMCAUgEBQHq8oMI1xgg0x/TP/gjqh9TILnyY+1E0NMf0z/T//QE0VNggED0Dm+hMfJgUXO68qIH+QFUEIf5EPKjAvQE0fgAf44WIYAQ9HhvpSCYAtMH1DAB+wCRMuIBs+ZbgyWhyEA0gED0Q4rmMQHIyx8Tyz/L//QAye1UCAAE0DACASAGBwAXvZznaiaGmvmOuF/8AEG+X5dqJoaY+Y6Z/p/5j6AmipEEAgegc30JjJLb/JXdHxQANCCAQPSWb6VsEiCUMFMDud4gkzM2AZJsIeKz" // save our base64 encoded output from compiler to variable
	codeCellBytes, _ := base64.StdEncoding.DecodeString(base64BOC)                                                                                                                                                                                                                                                                                  // decode base64 in order to get byte array
	codeCell, err := cell.FromBOC(codeCellBytes)                                                                                                                                                                                                                                                                                                    // get cell with code from byte array
	if err != nil {                                                                                                                                                                                                                                                                                                                                 // check if there are any error
		panic(err)
	}

	log.Println("Hash:", base64.StdEncoding.EncodeToString(codeCell.Hash())) // get the hash of our cell, encode it to base64 because it has []byte type and output to the terminal

	highloadMnemonicArray := strings.Split("put your mnemonic that you have generated and saved before", " ") // word1 word2 word3
	mac := hmac.New(sha512.New, []byte(strings.Join(highloadMnemonicArray, " ")))
	hash := mac.Sum(nil)
	k := pbkdf2.Key(hash, []byte("TON default seed"), 100000, 32, sha512.New) // In TON libraries "TON default seed" is used as salt when getting keys
	// 32 is a key len
	highloadPrivateKey := ed25519.NewKeyFromSeed(k)                      // get private key
	highloadPublicKey := highloadPrivateKey.Public().(ed25519.PublicKey) // get public key from private key

	dataCell := cell.BeginCell().
		MustStoreUInt(698983191, 32).           // Subwallet ID
		MustStoreUInt(0, 64).                   // Last cleaned
		MustStoreSlice(highloadPublicKey, 256). // Public Key
		MustStoreBoolBit(false).                // indicate that the dictionary is empty
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
		MustStoreStringSnake("Deploying...").
		EndCell()

	internalMessage := cell.BeginCell().
		MustStoreUInt(0x10, 6). // no bounce
		MustStoreAddr(contractAddress).
		MustStoreBigCoins(tlb.MustFromTON("0.01").NanoTON()).
		MustStoreUInt(0, 1+4+4+64+32).
		MustStoreBoolBit(true).            // We have State Init
		MustStoreBoolBit(true).            // We store State Init as a reference
		MustStoreRef(stateInit).           // Store State Init as a reference
		MustStoreBoolBit(true).            // We store Message Body as a reference
		MustStoreRef(internalMessageBody). // Store Message Body Init as a reference
		EndCell()

	connection := liteclient.NewConnectionPool()
	configUrl := "https://ton-blockchain.github.io/global.config.json"
	err = connection.AddConnectionsFromConfigUrl(context.Background(), configUrl)
	if err != nil {
		panic(err)
	}
	client := ton.NewAPIClient(connection)

	block, err := client.CurrentMasterchainInfo(context.Background()) // get current block, we will need it in requests to LiteServer
	if err != nil {
		log.Fatalln("CurrentMasterchainInfo err:", err.Error())
		return
	}

	walletMnemonicArray := strings.Split("put your mnemonic", " ")
	mac = hmac.New(sha512.New, []byte(strings.Join(walletMnemonicArray, " ")))
	hash = mac.Sum(nil)
	k = pbkdf2.Key(hash, []byte("TON default seed"), 100000, 32, sha512.New) // In TON libraries "TON default seed" is used as salt when getting keys
	// 32 is a key len
	walletPrivateKey := ed25519.NewKeyFromSeed(k) // get private key
	walletAddress := address.MustParseAddr("put your wallet address with which you will deploy")

	getMethodResult, err := client.RunGetMethod(context.Background(), block, walletAddress, "seqno") // run "seqno" GET method from your wallet contract
	if err != nil {
		log.Fatalln("RunGetMethod err:", err.Error())
		return
	}
	seqno := getMethodResult.MustInt(0) // get seqno from response

	toSign := cell.BeginCell().
		MustStoreUInt(698983191, 32).                          // subwallet_id | We consider this further
		MustStoreUInt(uint64(time.Now().UTC().Unix()+60), 32). // transaction expiration time, +60 = 1 minute
		MustStoreUInt(seqno.Uint64(), 32).                     // store seqno
		// Do not forget that if we use Wallet V4, we need to add .MustStoreUInt(0, 8)
		MustStoreUInt(3, 8).          // store mode of our internal transaction
		MustStoreRef(internalMessage) // store our internalMessage as a reference

	signature := ed25519.Sign(walletPrivateKey, toSign.EndCell().Hash()) // get the hash of our message to wallet smart contract and sign it to get signature

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
