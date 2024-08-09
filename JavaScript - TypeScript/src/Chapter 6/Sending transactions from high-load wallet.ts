import { Address, beginCell, internal, OutActionSendMsg, SendMode, toNano } from "@ton/core";
import { TonClient } from "@ton/ton";
import { HighloadWalletV3 } from "./wrappers/HighloadWalletV3";
import { mnemonicToWalletKey } from "@ton/crypto";
import { HighloadQueryId } from "./wrappers/HighloadQueryId";

async function main() {
    const client = new TonClient({
        endpoint: 'https://toncenter.com/api/v2/jsonRPC',
        apiKey: 'put your api key' // you can get an api key from @tonapibot bot in Telegram
    });

    const walletMnemonicArray = 'put your mnemonic'.split(' ');
    const walletKeyPair = await mnemonicToWalletKey(walletMnemonicArray); // extract private and public keys from mnemonic
    const wallet = client.open(HighloadWalletV3.createFromAddress(Address.parse('put your high-load wallet address')));
    console.log(`Wallet address: ${wallet.address.toString()}`);

    const queryHandler = HighloadQueryId.fromShiftAndBitNumber(0n, 0n);

    const actions: OutActionSendMsg[] = [];
    actions.push({
        type: 'sendMsg',
        mode: SendMode.CARRY_ALL_REMAINING_BALANCE,
        outMsg: internal({
            to: Address.parse('put address of deployer wallet'),
            value: toNano(0),
            body: beginCell()
                .storeUint(0, 32)
                .storeStringTail('Hello, TON!')
                .endCell()
        })
    });
    const subwalletId = 0x10ad;
    const timeout = 60 * 60; // must be same as in the contract
    const internalMessageValue = toNano(0.01); // in real case it is recommended to set the value to 1 TON
    const createdAt = Math.floor(Date.now() / 1000) - 60; // LiteServers have some delay in time
    await wallet.sendBatch(
        walletKeyPair.secretKey,
        actions,
        subwalletId,
        queryHandler,
        timeout,
        internalMessageValue,
        SendMode.PAY_GAS_SEPARATELY,
        createdAt
    );
    queryHandler.getNext();
}

main().finally(() => console.log("Exiting..."));