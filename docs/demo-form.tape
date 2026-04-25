# demo-form.tape — vertical slice for the @param input form.
#
# Showcases:
#   - enum field with type-ahead ("p" jumps to "prod")
#   - text field with live cmdline preview
#   - bool field toggling with space
#   - the run pill on the cmdline updating in lock-step
#
# Run with:  make demo-form

Output docs/demo-form.gif

Set Shell        zsh
Set FontSize     14
Set Width        1000
Set Height       540
Set Theme        "TokyoNight"
Set TypingSpeed  80ms

Hide
Type "./mkm"
Enter
Sleep 1500ms
Show

# Filter and open the param form on a target with @param annotations.
Type "deploy-app"
Sleep 800ms
Enter
Sleep 1500ms

# Enum type-ahead: 'p' lands on "prod" without arrow-key cycling.
Type "p"
Sleep 1s

# Move to the VERSION text field. The cmdline preview updates as you type.
Down
Sleep 300ms
Type "v1.2.3"
Sleep 1s

# Bool field — space toggles, the run pill stays in lock-step.
Down
Sleep 300ms
Space
Sleep 1500ms

# Cycle back to ENV and walk the enum manually for variety.
Up
Up
Sleep 500ms
Right
Sleep 500ms
Right
Sleep 1500ms

# Bail out before actually running.
Escape
Sleep 500ms
Escape
Sleep 500ms
