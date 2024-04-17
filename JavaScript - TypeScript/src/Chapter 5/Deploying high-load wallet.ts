import { compileFunc } from "@ton-community/func-js";
import { Address, Cell, beginCell, toNano } from "@ton/core";
import { mnemonicToWalletKey, sign } from "@ton/crypto";
import { TonClient } from "@ton/ton";
import fs from 'fs'

async function main() {
    const result = await compileFunc({
        targets: ['highload_wallet.fc'], // targets of your project
        sources: {
            'stdlib.fc': fs.readFileSync('./src/stdlib.fc', { encoding: 'utf-8' }),
            'highload_wallet.fc': fs.readFileSync('./src/highload_wallet.fc', { encoding: 'utf-8' }),
        }
    });
    
    if (result.status === 'error') {
    console.error(result.message)
    return;
    }
    
    const codeCell = Cell.fromBoc(Buffer.from(result.codeBoc, 'base64'))[0];
    
    // now we have base64 encoded BOC with compiled code in result.codeBoc
    console.log('Code BOC: ' + result.codeBoc);
    console.log('\nHash: ' + codeCell.hash().toString('base64')); // get the hash of cell and convert in to base64 encoded string

    const highloadMnemonicArray = 'put your mnemonic that you have generated and saved before'.split(' ');
    const highloadKeyPair = await mnemonicToWalletKey(highloadMnemonicArray); // extract private and public keys from mnemonic

    const dataCell = beginCell()
        .storeUint(698983191, 32) // Subwallet ID
        .storeUint(0, 64) // Last cleaned
        .storeBuffer(highloadKeyPair.publicKey) // Public Key
        .storeBit(0) // indicate that the dictionary is empty
        .endCell();

    const stateInit = beginCell()
        .storeBit(0) // No split_depth
        .storeBit(0) // No special
        .storeBit(1) // We have code
        .storeRef(codeCell)
        .storeBit(1) // We have data
        .storeRef(dataCell)
        .storeBit(0) // No library
        .endCell();

    const contractAddress = new Address(0, stateInit.hash()); // get the hash of stateInit to get the address of our smart contract in workchain with ID 0
    console.log(`Contract address: ${contractAddress.toString()}`); // Output contract address to console

    const internalMessageBody = beginCell()
        .storeUint(0, 32)
        .storeStringTail('Deploying...')
        .endCell();

    const internalMessage = beginCell()
        .storeUint(0x10, 6) // no bounce
        .storeAddress(contractAddress)
        .storeCoins(toNano('0.01'))
        .storeUint(0, 1 + 4 + 4 + 64 + 32)
        .storeBit(1) // We have State Init
        .storeBit(1) // We store State Init as a reference
        .storeRef(stateInit) // Store State Init as a reference
        .storeBit(1) // We store Message Body as a reference
        .storeRef(internalMessageBody) // Store Message Body Init as a reference
        .endCell();


    const client = new TonClient({
        endpoint: 'https://toncenter.com/api/v2/jsonRPC',
        apiKey: 'put your api key' // you can get an api key from @tonapibot bot in Telegram
    });

    const walletMnemonicArray = 'put your mnemonic'.split(' ');
    const walletKeyPair = await mnemonicToWalletKey(walletMnemonicArray); // extract private and public keys from mnemonic
    const walletAddress = Address.parse('put your wallet address with which you will deploy');
    const getMethodResult = await client.runMethod(walletAddress, 'seqno'); // run "seqno" GET method from your wallet contract
    const seqno = getMethodResult.stack.readNumber(); // get seqno from response

    // transaction for our wallet
    const toSign = beginCell()
        .storeUint(698983191, 32) // subwallet_id
        .storeUint(Math.floor(Date.now() / 1e3) + 60, 32) // Transaction expiration time, +60 = 1 minute
        .storeUint(seqno, 32) // store seqno
        // Do not forget that if we use Wallet V4, we need to add .storeUint(0, 8) 
        .storeUint(3, 8)
        .storeRef(internalMessage);

    const signature = sign(toSign.endCell().hash(), walletKeyPair.secretKey); // get the hash of our message to wallet smart contract and sign it to get signature
    const body = beginCell()
        .storeBuffer(signature) // store signature
        .storeBuilder(toSign) // store our message
        .endCell();

    const external = beginCell()
        .storeUint(0b10, 2) // indicate that it is an incoming external transaction
        .storeUint(0, 2) // src -> addr_none
        .storeAddress(walletAddress)
        .storeCoins(0) // Import fee
        .storeBit(0) // We do not have State Init
        .storeBit(1) // We store Message Body as a reference
        .storeRef(body) // Store Message Body as a reference
        .endCell();

    client.sendFile(external.toBoc());
}

main().finally(() => console.log("Exiting..."));