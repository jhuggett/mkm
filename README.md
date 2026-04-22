# MakeMe (mkm)

TUI for discovering and running Makefile targets. Scans the current directory and subdirectories for Makefiles, presents them in a filterable list, then runs the selected command.

## Install

```
go install github.com/jhuggett/mkm@latest
```

Run `mkm` straight after install and it works — the selected target is executed directly. For up-arrow in your current shell to recall mkm-launched commands, let mkm install a small zsh/bash wrapper into your rc file: inside the TUI press `ctrl+s`, focus the `shell_history` row, hit `ctrl+a`. Then `source ~/.zshrc` (or open a new shell). That's it. The row also lets you `ctrl+e` to edit the rc file or `ctrl+v` to view it.

The wrapper is a few lines — you can see the exact snippet in the TUI before applying, or let mkm append it for you.

## Usage

Run `mkm` in any directory containing a Makefile. It recursively finds Makefiles in subdirectories (skipping `node_modules`, `.git`, `vendor`, etc.) and groups targets by path.

- Type to fuzzy filter targets
- Arrow keys to navigate
- Enter to select
- Esc to quit

## Parameterized targets

Targets that use variables can be annotated with `@param` comments so mkm prompts for values via a small form before running. See [PARAMS.md](PARAMS.md) for the spec.

## Config

mkm reads `~/.config/mkm/config.json` on startup and creates it with defaults on first run:

```json
{
  "theme": "nord",
  "write_history": true,
  "shell_history": true
}
```

- `theme`: `nord`, `dracula`, `solarized-dark`, `mono`, `gruvbox-dark`, `tokyo-night`, `catppuccin-mocha`, `rose-pine`, `one-dark`, or `github-dark`. Unknown values fall back to `nord`.
- `write_history`: when `false`, mkm won't read or write `~/.cache/mkm/history`. Recency ranking is disabled but everything else works normally. Set this to `false` if history writes aren't working in your setup.
- `shell_history`: when `true` (the default), mkm also appends the executed command to your shell's `$HISTFILE` (zsh or bash — fish and others are skipped). Matters mainly for shells *without* the wrapper installed: it's how up-arrow in a future shell finds the entry. Harmless to leave on when the wrapper is installed — the wrapper runs mkm in `--print` mode, where this setting doesn't apply.

You can also edit these settings inside the TUI with `ctrl+s` — the theme updates live so you can preview before saving. `enter` persists, `esc` reverts.
