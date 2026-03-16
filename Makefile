# Build the binary in the repo (default target).
build:
	go build -o git-builder .

# Build and update the local install: copy binary and reload service.
# On macOS: installs to ~/.local/bin and restarts LaunchAgent.
# On Linux: installs to /usr/local/bin (run with: sudo make install).
install: build
	./git-builder -install

# Alias for install — use after rebuilding to refresh the running service.
update: install

.PHONY: build install update
