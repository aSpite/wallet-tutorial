import { Address, beginCell, toNano } from "@ton/core";
import { mnemonicToWalletKey, sign } from "@ton/crypto";
import { TonClient } from "@ton/ton";


async function main() {
    const destinationAddress = Address.parse("put your wallet where you want to send NFT");
    const walletAddress = Address.parse("put your wallet which is the owner of NFT")
    const nftAddress = Address.parse("put your nft address");

    // We can add a comment, but it will not be displayed in the explorers, 
    // as it is not supported by them at the time of writing the tutorial.
    const forwardPayload = beginCell()
        .storeUint(0, 32)
        .storeStringTail("Hello, TON!")
        .endCell();

    const transferNftBody = beginCell()
        .storeUint(0x5fcc3d14, 32) // Opcode for NFT transfer
        .storeUint(0, 64) // query_id
        .storeAddress(destinationAddress) // new_owner
        .storeAddress(walletAddress) // response_destination for excesses
        .storeBit(0) // we do not have custom_payload
        .storeCoins(toNano("0.01")) // forward_payload
        .storeBit(1) // we store forward_payload as a reference
        .storeRef(forwardPayload) // store forward_payload as a reference
        .endCell();

    const internalMessage = beginCell()
        .storeUint(0x18, 6) // bounce
        .storeAddress(nftAddress)
        .storeCoins(toNano("0.05"))
        .storeUint(1, 1 + 4 + 4 + 64 + 32 + 1 + 1) // We store 1 that means we have body as a reference
        .storeRef(transferNftBody)
        .endCell();

    const client = new TonClient({
        endpoint: "https://toncenter.com/api/v2/jsonRPC",
        apiKey: "put your api key" // you can get an api key from @tonapibot bot in Telegram
    });
    
    const mnemonic = 'put your mnemonic'; // word1 word2 word3
    let getMethodResult = await client.runMethod(walletAddress, "seqno"); // run "seqno" GET method from your wallet contract
    let seqno = getMethodResult.stack.readNumber(); // get seqno from response
    
    const mnemonicArray = mnemonic.split(' '); // get array from string
    const keyPair = await mnemonicToWalletKey(mnemonicArray); // get Secret and Public keys from mnemonic 
    
    let toSign = beginCell()
        .storeUint(698983191, 32) // subwallet_id
        .storeUint(Math.floor(Date.now() / 1e3) + 60, 32) // Transaction expiration time, +60 = 1 minute
        .storeUint(seqno, 32) // store seqno
        // Do not forget that if we use Wallet V4, we need to add .storeUint(0, 8) 
        .storeUint(3, 8)
        .storeRef(internalMessage);
    
    let signature = sign(toSign.endCell().hash(), keyPair.secretKey); // get the hash of our message to wallet smart contract and sign it to get signature
    
    let body = beginCell()
        .storeBuffer(signature) // store signature
        .storeBuilder(toSign) // store our message
        .endCell();
    
    let externalMessage = beginCell()
        .storeUint(0b10, 2) // ext_in_msg_info$10
        .storeUint(0, 2) // src -> addr_none
        .storeAddress(walletAddress) // Destination address
        .storeCoins(0) // Import Fee
        .storeBit(0) // No State Init
        .storeBit(1) // We store Message Body as a reference
        .storeRef(body) // Store Message Body as a reference
        .endCell();
    
    client.sendFile(externalMessage.toBoc());
}

main().finally(() => console.log("Exiting..."));