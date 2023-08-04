import { TonClient } from '@ton/ton';
import { Address } from '@ton/core';

async function main() {
    const client = new TonClient({
        endpoint: "https://toncenter.com/api/v2/jsonRPC",
        apiKey: "put your api key" // you can get an api key from @tonapibot bot in Telegram
    });

    const walletAddress = Address.parse("EQDKbjIcfM6ezt8KjKJJLshZJJSqX7XOA4ff-W72r5gqPrHF"); // my wallet address as an example
    
    // I always call runMethodWithError instead of runMethod to be able to check the exit_code of the called method. 
    let getResult = await client.runMethodWithError(walletAddress, "get_public_key"); // run get_public_key GET Method
    const publicKeyUInt = getResult.stack.readBigNumber(); // read answer that contains uint256
    const publicKey = publicKeyUInt.toString(16); // get hex string from bigint (uint256)
    console.log(publicKey);

    const oldWalletAddress = Address.parse("EQAM7M--HGyfxlErAIUODrxBA3yj5roBeYiTuy6BHgJ3Sx8k"); // my old wallet address
    const subscriptionAddress = Address.parseFriendly("EQBTKTis-SWYdupy99ozeOvnEBu8LRrQP_N9qwOTSAy3sQSZ"); // subscription plugin address which is already installed on the wallet
    const hash = BigInt(`0x${subscriptionAddress.address.hash.toString("hex")}`) ;

    getResult = await client.runMethodWithError(oldWalletAddress, "is_plugin_installed", 
    [
        {type: "int", value: BigInt("0")}, // pass workchain as int
        {type: "int", value: hash} // pass plugin address hash as int
    ]);
    console.log(getResult.stack.readNumber());
}
  
main().finally(() => console.log("Exiting..."));