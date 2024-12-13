#!/bin/sh

set -euo

forge build

cd build/emit.sol
cat EmitEvent.json | jq -r '.bytecode.object' > EmitEvent.bin
cat EmitEvent.json | jq '.abi' > EmitEvent.abi
cd ../..

mkdir -p bindings/emit
abigen --abi ./build/emit.sol/EmitEvent.abi --bin ./build/emit.sol/EmitEvent.bin --pkg emit --out ./bindings/emit/emit.go

cd build/ICrossL2Inbox.sol
cat ICrossL2Inbox.json | jq -r '.bytecode.object' > ICrossL2Inbox.bin
cat ICrossL2Inbox.json | jq '.abi' > ICrossL2Inbox.abi
cd ../..

mkdir -p bindings/inbox
abigen --abi ./build/ICrossL2Inbox.sol/ICrossL2Inbox.abi --bin ./build/ICrossL2Inbox.sol/ICrossL2Inbox.bin --pkg inbox --out ./bindings/inbox/inbox.go

cd build/ISystemConfig.sol
cat ISystemConfig.json | jq -r '.bytecode.object' > ISystemConfig.bin
cat ISystemConfig.json | jq '.abi' > ISystemConfig.abi
cd ../..

mkdir -p bindings/systemconfig
abigen --abi ./build/ISystemConfig.sol/ISystemConfig.abi --bin ./build/ISystemConfig.sol/ISystemConfig.bin --pkg systemconfig --out ./bindings/systemconfig/systemconfig.go
