# One still per screen, rendered once per breakpoint by `mise run demo:stills`,
# which fills in the name, width, height, and font placeholders below.
#
# The fleet is the throwaway set of repos demo/stills-fleet.sh builds, with HOME and the
# config pointed at it, so no real repo or setting is read. It has no GitHub data.
# Every Screenshot follows a Wait on text only that screen draws, so a still never
# shows a half-loaded or wrong screen. A Wait that never matches fails the render.
# Keep a Sleep after each Screenshot: VHS writes the file asynchronously and can exit first.

Output demo/.work/stills-__NAME__.gif

Set Shell "bash"
Set FontFamily "__FONT__"
Set FontSize 15
Set Width __WIDTH__
Set Height __HEIGHT__
Set Padding 18
Set WaitTimeout 30s
Set Framerate 15

Env COLORTERM "truecolor"

Hide
Type `eval "$(demo/stills-fleet.sh)" && bin=$PWD/gh-repo-dashboard && cd "$STILLS_FLEET" && echo fleet-$((6*7))` Enter
Wait+Screen /fleet-42/
Type "clear" Enter
Type "$bin" Enter
Wait+Screen /NEEDS YOU/
Wait+Screen /note.*gc/
Show

Down 2
Wait+Screen /theta/
Sleep 500ms
Screenshot docs/images/__NAME__-list.png
Sleep 1s

Type "f"
Type "d"
Wait+Screen /Dirty ×/
Sleep 300ms
Screenshot docs/images/__NAME__-filter.png
Sleep 1s

Type "*"
Escape
Wait+Screen /NEEDS YOU/
Up 2

Down 3
Type "v"
Sleep 1s
Screenshot docs/images/__NAME__-detail.png
Sleep 1s

Type "v"
Up 3
Enter
Wait+Screen /PEERS · 1/
Wait+Screen /feat: add feature file/
Sleep 300ms
Screenshot docs/images/__NAME__-focused.png
Sleep 1s

Escape
Wait+Screen /NEEDS YOU/
Type "T"
Wait+Screen /TASK .*LAST RUN/
Sleep 300ms
Screenshot docs/images/__NAME__-tasks.png
Sleep 1s

Enter
Wait+Screen /runs git gc/
Sleep 300ms
Screenshot docs/images/__NAME__-task-preview.png
Sleep 1s

Escape
Type "R"
Wait+Screen /NEEDS YOU/
Type "?"
Wait+Screen /keys/
Sleep 300ms
Screenshot docs/images/__NAME__-help.png
Sleep 1s

Escape
Sleep 500ms
Type ";"
Wait+Screen /find\.\.\./
Type "login"
Wait+Screen /2 matches/
Sleep 300ms
Screenshot docs/images/__NAME__-find.png
Sleep 1s
