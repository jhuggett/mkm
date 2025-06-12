# Example makefile

.PHONY: log-abc
log-abc:
	@echo "This is a log message from the log-abc target.";

.PHONY: log-xyz
log-xyz:
	@echo "This is a log message from the log-xyz target.";

.PHONY: log-all
log-all: 
	log-abc log-xyz;

.PHONY: build
build:
	go build -o mkm;

.PHONY: tar
tar:
	tar -czvf mkm.tar.gz mkm;