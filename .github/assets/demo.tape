# VHS documentation
#
# Run with:
#  vhs < .github/assets/demo.tape
#
# Output:
#   Output <path>.gif               Create a GIF output at the given <path>
#   Output <path>.mp4               Create an MP4 output at the given <path>
#   Output <path>.webm              Create a WebM output at the given <path>
#
# Settings:
#   Set FontSize <number>           Set the font size of the terminal
#   Set FontFamily <string>         Set the font family of the terminal
#   Set Height <number>             Set the height of the terminal
#   Set Width <number>              Set the width of the terminal
#   Set LetterSpacing <float>       Set the font letter spacing (tracking)
#   Set LineHeight <float>          Set the font line height
#   Set Theme <string>              Set the theme of the terminal (JSON)
#   Set Padding <number>            Set the padding of the terminal
#   Set Framerate <number>          Set the framerate of the recording
#   Set PlaybackSpeed <float>       Set the playback speed of the recording
#
# Sleep:
#   Sleep <time>                    Sleep for a set amount of <time> in seconds
#
# Type:
#   Type[@<time>] "<characters>"    Type <characters> into the terminal with a
#                                   <time> delay between each character
#
# Keys:
#   Backspace[@<time>] [number]     Press the Backspace key
#   Down[@<time>] [number]          Press the Down key
#   Enter[@<time>] [number]         Press the Enter key
#   Space[@<time>] [number]         Press the Space key
#   Tab[@<time>] [number]           Press the Tab key
#   Left[@<time>] [number]          Press the Left Arrow key
#   Right[@<time>] [number]         Press the Right Arrow key
#   Up[@<time>] [number]            Press the Up Arrow key
#   Down[@<time>] [number]          Press the Down Arrow key
#   Ctrl+<key>                      Press the Control key + <key> (e.g. Ctrl+C)
#
# Display:
#   Hide                            Hide the subsequent commands from the output
#   Show                            Show the subsequent commands in the output

Output .github/assets/demo.gif
Set Shell bash
Set PlaybackSpeed 1.0
Set TypingSpeed 10ms

Set Theme "Catppuccin Macchiato"
Set FontFamily "Hack Nerd Font Mono"

Set FontSize 14
Set Width 1600
Set Height 1000

Hide
Type `cd ~/Developer/kyleking/repo-dashboard`  Enter  Sleep 500ms
Type `go build -o gh-repo-dashboard .`  Enter  Sleep 2s
Type `cd ~/Developer/kyleking`  Enter  Sleep 500ms
Type `clear`  Enter
Show

# Start the TUI
Type `./repo-dashboard/gh-repo-dashboard . --depth 2`  Sleep 500ms  Enter
Sleep 2s

# Navigate repos
Down 3
Sleep 800ms

# Cycle filter mode (2 examples)
Type `f`
Sleep 1s
Type `f`
Sleep 1s

# Cycle sort mode (2 examples)
Type `s`
Sleep 1s
Type `s`
Sleep 1s

# Activate search
Type `/`
Sleep 400ms
Type `multi`
Sleep 1.5s
Ctrl+u
Sleep 400ms

# Exit search
Escape
Sleep 700ms

# Select a repo to drill down
Space
Sleep 1.5s

# Navigate branches
Down 3
Sleep 700ms

# Go back
Escape
Sleep 700ms

# Show help
Type `?`
Sleep 2s

# Close help and quit
Escape
Sleep 500ms
Type `q`
Sleep 300ms
