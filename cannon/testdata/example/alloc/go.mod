module alloc

go 1.22.0

toolchain go1.22.7

require github.com/ethereum-optimism/optimism v0.0.0

require (
	golang.org/x/crypto v0.28.0 // indirect
	golang.org/x/sys v0.26.0 // indirect
)

replace github.com/ethereum-optimism/optimism v0.0.0 => ../../../..
