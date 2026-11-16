### Bar Module Labels

## Network
bar-network-connecting = Connecting...
bar-network-disconnected = Disconnected
bar-network-wired = Wired
bar-network-wifi-fallback = WiFi
bar-network-no-wifi = No WiFi
bar-network-no-ethernet = No Ethernet
bar-network-offline = Offline

## Battery
bar-battery-unavailable = N/A

## Bluetooth
bar-bluetooth-disabled = Off
bar-bluetooth-disconnected = Disconnected
bar-bluetooth-connected-count = { $count ->
    [one] { $count } Connected
   *[other] { $count } Connected
}

## Window Title
bar-window-title-empty = Desktop

## Idle Inhibit
bar-idle-inhibit-on = On
bar-idle-inhibit-off = Off
bar-idle-policy-off = The built-in idle policy is off
bar-idle-held-by = Idle held by { $holders }
bar-idle-held-by-you = you
bar-idle-all-reached = Every idle stage has run
bar-idle-next = Next: { $stage } after { $after } idle
bar-idle-stage-dim = dim
bar-idle-stage-lock = lock
bar-idle-stage-screen-off = screens off
bar-idle-stage-suspend = suspend

## Recorder
bar-recorder-idle = Idle
bar-recorder-recording = Recording
bar-recorder-paused = Paused
bar-recorder-replay = Replay armed

## Keybind Mode
bar-keybind-mode-default = default

## Night light
bar-nightlight-on = On
bar-nightlight-off = Off
bar-nightlight-no-location = Auto-schedule paused, no location: { $reason }

## Storage
bar-storage-multiple = Multiple

## Media
bar-media-playing = Playing
bar-media-paused = Paused
bar-media-stopped = Stopped

## Minimized windows and the dock

bar-task-restore = Restore
bar-task-close = Close
bar-task-launch = Open
bar-task-none = Nothing is minimized

## Global shortcuts the shell registers with the compositor

shortcut-launcher = Open or close the launcher

shortcut-dashboard = Open or close the dashboard

shortcut-bar-hide = Hide every bar

shortcut-bar-show = Show every bar

shortcut-bar-toggle = Hide or show every bar

## Module titles: the name a screen reader and the tooltip give each module
bar-module-background-apps = Background apps
bar-module-audio = Audio
bar-module-battery = Battery
bar-module-bitwarden = Bitwarden
bar-module-bluetooth = Bluetooth
bar-module-brightness = Brightness
bar-module-cava = Audio visualizer
bar-module-clock = Clock
bar-module-cpu = CPU
bar-module-dashboard = Dashboard
bar-module-idle-inhibit = Idle inhibitor
bar-module-keybind-mode = Keybind mode
bar-module-keyboard-input = Keyboard layout
bar-module-mail = Mail
bar-module-media = Media
bar-module-netstat = Network traffic
bar-module-network = Network
bar-module-nightlight = Night light
bar-module-notifications = Notifications
bar-module-power = Power
bar-module-power-profiles = Power profile
bar-module-ram = Memory
bar-module-recorder = Screen recorder
bar-module-screenshot = Screenshot
bar-module-storage = Storage
bar-module-treeman = Worktrees
bar-module-weather = Weather
bar-module-window-title = Window title
bar-module-world-clock = World clock
bar-module-minimized-windows = Minimized windows
bar-module-separator = Separator
bar-module-systray = System tray
bar-module-workspaces = Workspaces
bar-module-hyprland-workspaces = { bar-module-workspaces }
bar-module-lys-wm-workspaces = { bar-module-workspaces }
bar-module-mango-workspaces = { bar-module-workspaces }
bar-module-niri-workspaces = { bar-module-workspaces }
bar-module-sway-workspaces = { bar-module-workspaces }
bar-brightness-following = { $level }, automatic
bar-brightness-paused = { $level }, paused: follows the room again when you resume
bar-audio-speaker-muted = { $level }, muted
bar-audio-mic-muted = microphone muted
bar-audio-capture-mic = an app records the microphone
bar-audio-capture-camera = an app uses a camera
bar-audio-capture-screen = an app shares the screen

## Failures a module shows on its button
bar-module-failed = Error
bar-custom-command-timed-out = The command took too long and was stopped
bar-custom-command-failed = The command could not run: { $error }
bar-world-clock-invalid-zone = Unknown time zone “{ $zone }”
bar-world-clock-invalid-layout = Unsupported time layout “{ $layout }”
bar-world-clock-invalid-format = The format cannot be shown: { $error }
