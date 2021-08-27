#!/bin/bash
git clone --depth=1 --branch develop https://github.com/Synthetixio/synthetix.git
cd synthetix
npm install
npx hardhat --config ./hardhat.config.js test:integration:l2 --compile --deploy
npx hardhat --config ./hardhat.config.js test:integration:dual
