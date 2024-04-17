import { Address, Cell, beginCell, toNano } from "@ton/core";
import { mnemonicToWalletKey, sign } from "@ton/crypto";
import { TonClient } from "@ton/ton";


async function main() {
    const internalMessagesAmount = ["0.01", "0.02", "0.03", "0.04"];
    const internalMessagesComment = [
        "Hello, TON! #1",
        "Hello, TON! #2",
        "", // Let's leave the third transaction without comment
        "Hello, TON! #4" 
    ]
    const destinationAddresses = [
        "Put any address that belongs to you",
        "Put any address that belongs to you",
        "Put any address that belongs to you",
        "Put any address that belongs to you"
    ] // All 4 addresses can be the same

    let internalMessages:Cell[] = []; // array for our internal messages

    for (let index = 0; index < internalMessagesAmount.length; index++) {
        const amount = internalMessagesAmount[index];
        
        let internalMessage = beginCell()
            .storeUint(0x18, 6) // bounce
            .storeAddress(Address.parse(destinationAddresses[index]))
            .storeCoins(toNano(amount))
            .storeUint(0, 1 + 4 + 4 + 64 + 32 + 1);
            
        /*
            At this stage, it is not clear if we will have a message body. 
            So put a bit only for stateInit, and if we have a comment, in means 
            we have a body message. In that case, set the bit to 1 and store the 
            body as a reference.
        */

        if(internalMessagesComment[index] != "") {
            internalMessage.storeBit(1) // we store Message Body as a reference

            let internalMessageBody = beginCell()
                .storeUint(0, 32)
                .storeStringTail(internalMessagesComment[index])
                .endCell();

            internalMessage.storeRef(internalMessageBody);
        } 
        else 
            /*
                Since we do not have a message body, we indicate that 
                the message body is in this message, but do not write it, 
                which means it is absent. In that case, just set the bit to 0.
            */
            internalMessage.storeBit(0);
        
        internalMessages.push(internalMessage.endCell());
    }

    const walletAddress = Address.parse('put your wallet address');
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
        .storeUint(seqno, 32); // store seqno
    // Do not forget that if we use Wallet V4, we need to add .storeUint(0, 8) 
    
    for (let index = 0; index < internalMessages.length; index++) {
        const internalMessage = internalMessages[index];
        toSign.storeUint(3, 8) // store mode of our internal transaction
        toSign.storeRef(internalMessage) // store our internalMessage as a reference
    }

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