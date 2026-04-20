# MakeMe (mkm)

TUI for discovering and running Makefile targets. Scans the current directory and subdirectories for Makefiles, presents them in a filterable list, and outputs the selected make command.

## Install

```
go install github.com/jhuggett/mkm@latest
```

## Shell setup

mkm prints the selected command to stdout so your shell can handle history and execution. Add one of the following to your shell config:

### zsh (~/.zshrc)

```zsh
mkm() {
  local cmd
  cmd=$(command mkm)
  if [ -n "$cmd" ]; then
    print -s "$cmd"
    eval "$cmd"
  fi
}
```

### bash (~/.bashrc)

```bash
mkm() {
  local cmd
  cmd=$(command mkm)
  if [ -n "$cmd" ]; then
    history -s "$cmd"
    eval "$cmd"
  fi
}
```

This gives you two things:
- The selected make command is added to your shell history, so pressing up arrow recalls `make build` instead of `mkm`
- The command is executed in your current shell

## Usage

Run `mkm` in any directory containing a Makefile. It recursively finds Makefiles in subdirectories (skipping `node_modules`, `.git`, `vendor`, etc.) and groups targets by path.

- Type to fuzzy filter targets
- Arrow keys to navigate
- Enter to select
- Esc to quit

## Parameterized targets

Targets that use variables can be annotated with `@param` comments so mkm prompts for values via a small form before running. See [PARAMS.md](PARAMS.md) for the spec.
