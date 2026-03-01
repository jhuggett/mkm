# Log a message from the abc subsystem
.PHONY: log-abc
log-abc:
	@echo "This is a log message from the log-abc target.";

# Log a message from the xyz subsystem
.PHONY: log-xyz
log-xyz:
	@echo "This is a log message from the log-xyz target.";

# Run all log targets
.PHONY: log-all
log-all: log-abc log-xyz
	@echo "All logs complete.";

# Compile the mkm binary
.PHONY: build
build:
	go build -o mkm;

# Build and install mkm globally
.PHONY: install
install: build
	cp mkm ~/go/bin/mkm

# Package the binary into a tarball for distribution
.PHONY: tar
tar: build
	tar -czvf mkm.tar.gz mkm;