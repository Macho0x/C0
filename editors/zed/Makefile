.PHONY: all build install clean

WASM_TARGET := wasm32-wasip1
EXTENSION_DIR := $(HOME)/.local/share/zed/extensions/installed/c0

all: build

build:
	cargo build --target $(WASM_TARGET) --release
	cp target/$(WASM_TARGET)/release/c0.wasm extension.wasm

install: build
	mkdir -p $(EXTENSION_DIR)
	rsync -a --exclude='src/' --exclude='target/' --exclude='Cargo.*' \
		--exclude='*.lock' --exclude='README.md' \
		. $(EXTENSION_DIR)/
	@echo "C0 extension installed. Restart Zed or run 'zed: reload extensions'."

clean:
	rm -f extension.wasm
	cargo clean
