VERSION := $(shell sed -n 's/^  version: //p' substreams.yaml | head -1)
SPKG := ../../spkgs/eth-methods-$(VERSION).spkg

.PHONY: all
all: pack

# Regenerates `src/pb` and builds the wasm. Reaches out to the buf remote plugins, which is why
# the generated bindings are committed: a plain `cargo build` then needs no network.
.PHONY: build
build:
	substreams build --no-pack

# `substreams build` packs next to the manifest under a name of its own choosing, the committed
# artifact lives elsewhere so it gets packed explicitly.
.PHONY: pack
pack: build
	substreams pack -o $(SPKG) substreams.yaml

.PHONY: test
test:
	cargo test

.PHONY: info
info: pack
	substreams info $(SPKG)

.PHONY: clean
clean:
	cargo clean
