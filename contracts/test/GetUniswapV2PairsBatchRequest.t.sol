// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.0;

import "forge-std/console.sol";
import "./Test.sol";
import "../uniswap_v2/GetUniswapV2PairsBatchRequest.sol";
//import "../uniswap_v3/SyncUniswapV3PoolBatchRequest.sol";

contract DataTest is DSTest {
    function setUp() public {
        console.log("setup");
    }

    function testV2PairsBatchContract() public {
        address factory = 0x5C69bEe701ef814a2B6a3EDD4B1652CB9cc5aA6f;

        GetUniswapV2PairsBatchRequest batchContract = new GetUniswapV2PairsBatchRequest(
            2638438,
            300,
            factory
        );
        console.log(batchContract);
//        console.log("testV2PairsBatchContract");

    }
}
