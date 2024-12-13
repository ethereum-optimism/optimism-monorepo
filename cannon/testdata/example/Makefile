all: elf

.PHONY: elf32
elf32: $(patsubst %/go.mod,bin/%.elf,$(wildcard */go.mod))

.PHONY: elf64
elf64: $(patsubst %/go.mod,bin/%.64.elf,$(wildcard */go.mod))

.PHONY: elf
elf: elf32 elf64

.PHONY: dump
dump: $(patsubst %/go.mod,bin/%.dump,$(wildcard */go.mod))

bin:
	mkdir bin

bin/%.64.elf: bin
	cd $(@:bin/%.64.elf=%) && GOOS=linux GOARCH=mips64 GOMIPS64=softfloat go build -o ../$@ .

# take any directory with a go mod, and build an ELF
# verify output with: readelf -h bin/<name>.elf
# result is mips32, big endian, R3000
bin/%.elf: bin
	cd $(@:bin/%.elf=%) && GOOS=linux GOARCH=mips GOMIPS=softfloat go build -o ../$@ .

# take any ELF and dump it
# TODO: currently have the little-endian toolchain, but should use the big-endian one. The -EB compat flag works though.
bin/%.dump: bin/%.elf
	mipsel-linux-gnu-objdump -D --disassembler-options=no-aliases --wide --source -m mips:3000 -EB $(@:%.dump=%.elf) > $@
