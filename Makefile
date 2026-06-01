APP_NAME := snishaper
GO_BUILD := go build
GO_FLAGS := -ldflags="-s -w"
GOOS ?= linux
GOARCH ?= amd64
CGO_ENABLED ?= 0
WEB_DIR := web
BUILD_DIR := build
DIST_DIR := $(WEB_DIR)/dist

.PHONY: all build clean install dev run web-run tun dist

all: dist

build:
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO_BUILD) $(GO_FLAGS) -o $(APP_NAME) .

web:
	cd $(WEB_DIR) && npm install && npm run build

dist: web build
	rm -rf $(BUILD_DIR)
	mkdir -p $(BUILD_DIR)/rules
	mkdir -p $(BUILD_DIR)/config
	mkdir -p $(BUILD_DIR)/core/mihomo
	cp $(APP_NAME) $(BUILD_DIR)/
	cp rules/config.json $(BUILD_DIR)/rules/
	cp config/settings.json $(BUILD_DIR)/config/
	cp -r core/mihomo/mihomo $(BUILD_DIR)/core/mihomo/
	cp README.md $(BUILD_DIR)/
	cp README_EN.md $(BUILD_DIR)/
	cp LICENSE $(BUILD_DIR)/
	@echo ""
	@echo "=== Build Complete ==="
	@echo "Output: $(BUILD_DIR)/"
	@ls -la $(BUILD_DIR)/$(APP_NAME)
	@echo "Rules: $(BUILD_DIR)/rules/"
	@echo "Config: $(BUILD_DIR)/config/"
	@echo "Mihomo: $(BUILD_DIR)/core/mihomo/"

clean:
	rm -f $(APP_NAME)
	rm -rf $(BUILD_DIR)
	rm -rf $(DIST_DIR)

install: dist
	install -m 755 $(BUILD_DIR)/$(APP_NAME) /usr/local/bin/
	cp -r $(BUILD_DIR)/rules /usr/local/share/snishaper/
	cp -r $(BUILD_DIR)/config /usr/local/share/snishaper/
	cp -r $(BUILD_DIR)/core /usr/local/share/snishaper/

dev:
	cd $(WEB_DIR) && npm run dev &

run: build
	./$(APP_NAME) start

web-run: build
	./$(APP_NAME) web

# Start proxy + web dashboard together (default)
start: build
	./$(APP_NAME) start

# Start proxy only (no web dashboard)
start-core: build
	./$(APP_NAME) start --no-web

tun: build
	sudo ./$(APP_NAME) tun start
