# Installation

OP Deployer can be installed both from pre-built binaries and from source. This guide will walk you through both
methods.

## Install From Binaries

Installing OP Deployer from pre-built binaries is the easiest and most preferred way to get started. To install from 
binaries, download the latest release from the [releases page][releases] and extract the binary to a directory in your 
`$PATH`.

[releases]: https://github.com/ethereum-optimism/optimism/releases?q=op-deployer&expanded=true

## Install From Source

To install from source, you will need Go, `just`, and `git`. Then, run the following:

```shell
git clone git@github.com:ethereum-optimism/ethereum-optimism.git # you can skip this if you already have the repo
cd ethereum-optimism/op-deployer
just build
cp ./bin/op-deployer /usr/local/bin/op-deployer # or any other directory in your $PATH
```