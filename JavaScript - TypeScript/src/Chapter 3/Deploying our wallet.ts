import { compileFunc } from "@ton-community/func-js";
import { Address, Cell, beginCell, toNano } from "@ton/core";
import { mnemonicNew, mnemonicToWalletKey, sign } from "@ton/crypto";
import { TonClient } from "@ton/ton";
import fs from 'fs'; // we use fs for reading content of files

async function main() {
    // const mnemonicArray = 'put your mnemonic'.split(' ') // get our mnemonic as array
    const mnemonicArray = await mnemonicNew(24); // 24 is the number of words in a seed phrase
    const keyPair = await mnemonicToWalletKey(mnemonicArray); // extract private and public keys from mnemonic
    console.log(mnemonicArray) // if we want, we can print our mnemonic

    const subWallet = 698983191;

    const result = await compileFunc({
        targets: ['wallet_v3.fc'], // targets of your project
        sources: {
            "stdlib.fc": fs.readFileSync('./src/stdlib.fc', { encoding: 'utf-8' }),
            "wallet_v3.fc": fs.readFileSync('./src/wallet_v3.fc', { encoding: 'utf-8' }),
        }
    });

    if (result.status === 'error') {
        console.error(result.message)
        return;
    }

    const codeCell = Cell.fromBoc(Buffer.from(result.codeBoc, "base64"))[0]; // get buffer from base64 encoded BOC and get cell from this buffer

    // now we have base64 encoded BOC with compiled code in result.codeBoc
    console.log('Code BOC: ' + result.codeBoc);
    console.log('\nHash: ' + codeCell.hash().toString('base64')); // get the hash of cell and convert in to base64 encoded string. We will need it further

    const dataCell = beginCell().
        storeUint(0, 32). // Seqno
        storeUint(698983191, 32). // Subwallet ID
        storeBuffer(keyPair.publicKey). // Public Key
        endCell();

    const stateInit = beginCell().
        storeBit(0). // No split_depth
        storeBit(0). // No special
        storeBit(1). // We have code
        storeRef(codeCell).
        storeBit(1). // We have data
        storeRef(dataCell).
        storeBit(0). // No library
        endCell();

    const contractAddress = new Address(0, stateInit.hash()); // get the hash of stateInit to get the address of our smart contract in workchain with ID 0
    console.log(`Contract address: ${contractAddress.toString()}`); // Output contract address to console

    const internalMessageBody = beginCell().
    storeUint(0, 32).
    storeStringTail("Hello, TON!").
    endCell();

    const internalMessage = beginCell().
    storeUint(0x10, 6). // no bounce
    storeAddress(Address.parse("put your first wallet address from were you sent 0.1 TON")).
    storeCoins(toNano("0.03")).
    storeUint(1, 1 + 4 + 4 + 64 + 32 + 1 + 1). // We store 1 that means we have body as a reference
    storeRef(internalMessageBody).
    endCell();

    // transaction for our wallet
    const toSign = beginCell().
    storeUint(subWallet, 32).
    storeUint(Math.floor(Date.now() / 1e3) + 60, 32).
    storeUint(0, 32). // We put seqno = 0, because after deploying wallet will store 0 as seqno
    storeUint(3, 8).
    storeRef(internalMessage);

    const signature = sign(toSign.endCell().hash(), keyPair.secretKey);
    const body = beginCell().
    storeBuffer(signature).
    storeBuilder(toSign).
    endCell();

    const externalMessage = beginCell().
        storeUint(0b10, 2). // indicate that it is an incoming external transaction
        storeUint(0, 2). // src -> addr_none
        storeAddress(contractAddress).
        storeCoins(0). // Import fee
        storeBit(1). // We have State Init
        storeBit(1). // We store State Init as a reference
        storeRef(stateInit). // Store State Init as a reference
        storeBit(1). // We store Message Body as a reference
        storeRef(body). // Store Message Body as a reference
        endCell();

    const client = new TonClient({
        endpoint: "https://toncenter.com/api/v2/jsonRPC",
        apiKey: "put your api key" // you can get an api key from @tonapibot bot in Telegram
    });
        
    client.sendFile(externalMessage.toBoc());
}

main().finally(() => console.log("Exiting..."));