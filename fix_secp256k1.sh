#!/usr/bin/env bash
cd vendor/github.com/ethereum/go-ethereum/crypto/secp256k1
rm -rf libsecp256k1
git clone https://github.com/bitcoin-core/secp256k1.git libsecp256k1
cd libsecp256k1
git checkout 9d560
