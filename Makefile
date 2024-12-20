.SILENT: ;               # No need for @
.ONESHELL: ;             # Single shell for a target (required to properly use all of our local variables)
.EXPORT_ALL_VARIABLES: ; # Send all vars to shell
.DEFAULT: help # Running Make without target will run the help target

.PHONY: help clean build-vp

help: ## Show Help
	grep -E '^[a-zA-Z_-]+:.*?## .*$$' Makefile | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

clean:
	rm -f src/target/*

# run 'make build-vp OFFLINE=true' to build from vendor folder
build-vp: clean ## Build binary
	set -e
	cd src
	BUILD_FROM=mod
	if [ "$$OFFLINE" == "true" ]; then \
		echo "Building using local vendor folder (ie offline build) ..."; \
		BUILD_FROM=vendor; \
	else \
		echo "Building ..."; \
	fi
	ARCH=("linux/amd64" "darwin/arm64")
	for platform in "$${ARCH[@]}"; do \
		IFS='/' read -r GOOS GOARCH <<< "$$platform"; \
		output="vectp-$$GOOS-$$GOARCH"; \
		GOOS=$$GOOS GOARCH=$$GOARCH go build -mod=$$BUILD_FROM -a -o target/$$output; \
	done
	cd -
