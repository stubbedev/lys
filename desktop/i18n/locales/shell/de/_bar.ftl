### Bar Module Labels

## Network
bar-network-connecting = Verbindung wird hergestellt …
bar-network-disconnected = Getrennt
bar-network-wired = Kabelgebunden
bar-network-wifi-fallback = WLAN
bar-network-no-wifi = Kein WLAN
bar-network-no-ethernet = Kein Ethernet
bar-network-offline = Offline

## Battery
bar-battery-unavailable = k. A.

## Bluetooth
bar-bluetooth-disabled = Aus
bar-bluetooth-disconnected = Getrennt
bar-bluetooth-connected-count = { $count ->
    [one] { $count } verbunden
   *[other] { $count } verbunden
}

## Window Title
bar-window-title-empty = Schreibtisch

## Idle Inhibit
bar-idle-inhibit-on = An
bar-idle-inhibit-off = Aus
bar-idle-policy-off = Die eingebaute Leerlaufrichtlinie ist aus
bar-idle-held-by = Leerlauf verhindert durch { $holders }
bar-idle-held-by-you = Sie
bar-idle-all-reached = Alle Leerlaufstufen wurden ausgeführt
bar-idle-next = Als Nächstes: { $stage } nach { $after } Leerlauf
bar-idle-stage-dim = abdunkeln
bar-idle-stage-lock = sperren
bar-idle-stage-screen-off = Bildschirme aus
bar-idle-stage-suspend = Bereitschaft

## Recorder
bar-recorder-idle = Untätig
bar-recorder-recording = Aufnahme läuft
bar-recorder-paused = Pausiert
bar-recorder-replay = Wiederholung bereit

## Keybind Mode
bar-keybind-mode-default = Standard

## Night light
bar-nightlight-on = An
bar-nightlight-off = Aus
bar-nightlight-no-location = Automatischer Zeitplan pausiert, kein Standort: { $reason }

## Storage
bar-storage-multiple = Mehrere

## Media
bar-media-playing = Wiedergabe
bar-media-paused = Pausiert
bar-media-stopped = Gestoppt

## Minimized windows and the dock

bar-task-restore = Wiederherstellen
bar-task-close = Schließen
bar-task-launch = Öffnen
bar-task-none = Nichts ist minimiert

## Global shortcuts the shell registers with the compositor

shortcut-launcher = Starter öffnen oder schließen

shortcut-dashboard = Dashboard öffnen oder schließen

shortcut-bar-hide = Alle Leisten ausblenden

shortcut-bar-show = Alle Leisten einblenden

shortcut-bar-toggle = Alle Leisten aus- oder einblenden

## Module titles: the name a screen reader and the tooltip give each module
bar-module-background-apps = Hintergrund-Apps
bar-module-battery = Akku
bar-module-bitwarden = Bitwarden
bar-module-bluetooth = Bluetooth
bar-module-brightness = Helligkeit
bar-module-cava = Audio-Visualisierung
bar-module-clock = Uhr
bar-module-cpu = CPU
bar-module-dashboard = Dashboard
bar-module-idle-inhibit = Leerlaufsperre
bar-module-keybind-mode = Tastenbelegungsmodus
bar-module-keyboard-input = Tastaturbelegung
bar-module-mail = E-Mail
bar-module-media = Medien
bar-module-microphone = Mikrofon
bar-module-netstat = Netzwerkverkehr
bar-module-network = Netzwerk
bar-module-nightlight = Nachtlicht
bar-module-notifications = Benachrichtigungen
bar-module-power = Energie
bar-module-power-profiles = Energieprofil
bar-module-privacy = Datenschutz
bar-module-ram = Arbeitsspeicher
bar-module-recorder = Bildschirmaufnahme
bar-module-screen-share = Bildschirmfreigabe
bar-module-screenshot = Bildschirmfoto
bar-module-storage = Speicherplatz
bar-module-treeman = Worktrees
bar-module-volume = Lautstärke
bar-module-weather = Wetter
bar-module-window-title = Fenstertitel
bar-module-world-clock = Weltuhr
bar-module-minimized-windows = Minimierte Fenster
bar-module-separator = Trenner
bar-module-systray = Systemleiste
bar-module-workspaces = Arbeitsflächen
bar-module-hyprland-workspaces = { bar-module-workspaces }
bar-module-lys-wm-workspaces = { bar-module-workspaces }
bar-module-mango-workspaces = { bar-module-workspaces }
bar-module-niri-workspaces = { bar-module-workspaces }
bar-module-sway-workspaces = { bar-module-workspaces }
bar-brightness-following = { $level }, automatisch
bar-brightness-paused = { $level }, pausiert: folgt wieder dem Raumlicht, wenn Sie fortsetzen
