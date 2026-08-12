APP := r3-intake
VENDOR := internal/assets/public/htmx.min.js internal/assets/public/alpine.min.js

.PHONY: build vendor run run-admin run-mcp clean

build: vendor
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $(APP) ./cmd/r3-intake

vendor:
	@for f in $(VENDOR); do \
	  [ -f $$f ] || (echo "fetching $$f"; \
	    case $$f in \
	      *htmx.min.js) curl -fsSL -o $$f https://unpkg.com/htmx.org@2.0.3/dist/htmx.min.js ;; \
	      *alpine.min.js) curl -fsSL -o $$f https://unpkg.com/alpinejs@3.14.1/dist/cdn.min.js ;; \
	    esac); \
	done

run: build
	./$(APP) serve

run-admin: build
	./$(APP) serve --admin

run-mcp: build
	./$(APP) mcp

clean:
	rm -f $(APP)

verify:
	@file ./$(APP) | grep -qE "statically linked|static-pie" && echo "static link OK" || (echo "FATAL: not statically linked"; exit 1)