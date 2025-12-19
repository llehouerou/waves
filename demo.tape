# Waves Demo
Output assets/demo.gif

Require make

Set Shell "bash"
Set FontFamily "JetBrainsMono Nerd Font"
Set FontSize 14
Set Width 1400
Set Height 810
Set Padding 10
Set Framerate 30
Set PlaybackSpeed 1
Set TypingSpeed 50ms

# Start waves (hidden)
Hide
Type "make run"
Enter
Sleep 3s
Show

# Force library album view (secret demo binding)
Ctrl+Alt+d
Sleep 300ms

# Search for Highway to Hell
Type "/"
Sleep 300ms
Type "highway to hell"
Sleep 500ms
Enter
Sleep 1s

# Play the album from first track
Enter
Sleep 3s

# Show queue panel
Tab
Sleep 2s

# Browse queue
Type "j" Sleep 150ms
Type "j" Sleep 150ms
Type "j" Sleep 150ms
Sleep 1s

# Back to navigator
Tab
Sleep 2s

# Quit (hidden)
Hide
Type "q"
Sleep 500ms
