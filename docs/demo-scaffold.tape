# demo-scaffold.tape — vertical slice for the @param scaffolding flow (^a).
#
# Showcases:
#   - ^a opens scaffold form pre-populated with detected $(VAR) refs
#   - the live GitHub-style diff preview against the source file
#   - editable description / type / required / default / desc fields
#
# Uses the demo-undoc fixture target (in this repo's Makefile), which
# has three undocumented $(VAR) refs so the auto-detection is visible.
#
# Run with:  make demo-scaffold

Output docs/demo-scaffold.gif

Set Shell        zsh
Set FontSize     14
Set Width        1100
Set Height       640
Set Theme        "TokyoNight"
Set TypingSpeed  80ms

Hide
Type "./mkm"
Enter
Sleep 1500ms
Show

# Filter to the demo fixture and open the scaffold form.
Type "demo-undoc"
Sleep 800ms
Ctrl+a
Sleep 2s

# Edit the description (focus starts on the description row).
Type "Roll out a service to a region"
Sleep 1500ms

# Tab into the SERVICE row and walk down through its sub-fields.
Tab
Sleep 600ms
Tab
Sleep 600ms
Tab
Sleep 600ms

# Cycle the type field a couple of times — diff updates live.
Right
Sleep 600ms
Right
Sleep 1500ms

# Bail out without writing.
Escape
Sleep 500ms
Escape
Sleep 500ms
