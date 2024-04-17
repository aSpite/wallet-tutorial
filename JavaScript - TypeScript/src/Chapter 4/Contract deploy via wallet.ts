import { Address, Cell, beginCell, toNano } from "@ton/core";
import { mnemonicToWalletKey, sign } from "@ton/crypto";
import { TonClient } from "@ton/ton";


async function main() {
    const mnemonicArray = 'put your mnemonic'.split(" ");
    const keyPair = await mnemonicToWalletKey(mnemonicArray); // extract private and public keys from mnemonic

    const codeCell = Cell.fromBase64('te6ccgEBCAEAhgABFP8A9KQT9LzyyAsBAgEgAgMCAUgEBQCW8oMI1xgg0x/TH9MfAvgju/Jj7UTQ0x/TH9P/0VEyuvKhUUS68qIE+QFUEFX5EPKj+ACTINdKltMH1AL7AOgwAaTIyx/LH8v/ye1UAATQMAIBSAYHABe7Oc7UTQ0z8x1wv/gAEbjJftRNDXCx+A==');
    const dataCell = beginCell()
        .storeUint(0, 32) // Seqno
        .storeUint(3, 32) // Subwallet ID
        .storeBuffer(keyPair.publicKey) // Public Key
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

    console.log(external.toBoc().toString('base64'));
    client.sendFile(external.toBoc());
}

main().finally(() => console.log("Exiting..."));