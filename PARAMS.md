# Parameter annotations

mkm reads `@param` lines from the leading comment block above a target. Annotated targets open a small input form on selection; mkm then emits `make TARGET VAR=value …` with the values you supplied.

## Syntax

```
# @param {<type>} <name-spec>    <description>
```

- **`{<type>}`** — one of `string`, `int`, `bool`, or a pipe-separated enum like `dev|staging|prod`.
- **`<name-spec>`** — the Make variable name:
  - `NAME` — required.
  - `[NAME]` — optional, no default (empty is fine).
  - `[NAME=default]` — optional with default.
- **`<description>`** — everything after the name-spec; rendered next to the field.

## Types

| Type          | Input UI              | Notes                                    |
| ------------- | --------------------- | ---------------------------------------- |
| `{string}`    | free-text input       | any value                                |
| `{int}`       | numeric input         | non-digit keys ignored (leading `-` ok) |
| `{bool}`      | toggle                | serialized as `true` / `false`           |
| `{a\|b\|c}`   | enum picker           | `←/→` (or `space`) to cycle              |

Enums need ≥ 2 options. If you set a default on an enum, it must match one of the options or the `@param` is dropped.

## Placement

`@param` lines can live in two places:

**Target-level** — in the leading comment block directly above a target (above `.PHONY` if present). A blank line ends the block. These apply only to that target.

**File-level** — anywhere in the Makefile outside a target's comment block (typically at the top). These apply to any target in the same Makefile whose recipe references them. Handy for project-wide inputs like API keys or environment names loaded from `.env`.

Order is preserved in the form and on the command line.

```makefile
# @param {string} DEVELOPER_ID_APPLICATION  Apple Developer ID (from .env)

APP_NAME := My App
VERSION  := 0.1.0

# Deploy the app to an environment
# @param {dev|staging|prod} ENV           target environment
# @param {string} [RELEASE=latest]        release tag
# @param {bool} [DRY_RUN=false]           preview without executing
.PHONY: deploy
deploy:
	./deploy.sh $(ENV) $(RELEASE) --cert "$(DEVELOPER_ID_APPLICATION)"
```

Because `deploy`'s recipe references `$(DEVELOPER_ID_APPLICATION)`, the form will include that field alongside its own `ENV` / `RELEASE` / `DRY_RUN`. Targets that don't reference the file-level var won't see it.

You can still use leading `#` comments for the target description and inline `##` descriptions; inline `##` wins over the leading comment.

## Runtime behavior

- **No `@param` on target** → `enter` runs `make TARGET` (previous behavior).
- **With `@param`** → `enter` opens the form:
  - Fields are pre-populated from `default`, the first enum option, or `false` for bool.
  - Required fields are marked `*`.
  - `↑↓` / `tab` to move between fields, `←→` for enums & bools, typing for strings/ints.
  - `enter` builds the command and exits; `esc` goes back to the list.
- **Empty string values are omitted** from the emitted command, so any Makefile-internal `?=` defaults still apply.
- **Shell-special values are single-quoted** automatically.

## Example

```makefile
# Scaffold a new service from the template
# @param {string} NAME                     service name (kebab-case)
# @param {go|rust|ts} [LANG=go]            implementation language
# @param {http|grpc|both} [TRANSPORT=http] transport surfaces
.PHONY: scaffold-service
scaffold-service:
	./scaffold.sh $(NAME) --lang=$(LANG) --transport=$(TRANSPORT)
```

After picking `scaffold-service` in mkm, filling `NAME=billing`, leaving `LANG=go`, and flipping `TRANSPORT` to `grpc`:

```
$ make scaffold-service NAME=billing LANG=go TRANSPORT=grpc
```

## Grammar reference

```
@param  {<type>}  <name-spec>  <description>

<type>      ::= "string" | "int" | "bool" | <enum>
<enum>      ::= <option> ("|" <option>)+
<name-spec> ::= NAME | "[" NAME "]" | "[" NAME "=" <default> "]"
```
