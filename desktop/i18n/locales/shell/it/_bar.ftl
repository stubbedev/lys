### Bar Module Labels

## Network
bar-network-connecting = Connessione in corso...
bar-network-disconnected = Disconnesso
bar-network-wired = Cablata
bar-network-wifi-fallback = Wi-Fi
bar-network-no-wifi = Nessun Wi-Fi
bar-network-no-ethernet = Nessuna Ethernet
bar-network-offline = Non in linea

## Battery
bar-battery-unavailable = N/D

## Bluetooth
bar-bluetooth-disabled = Spento
bar-bluetooth-disconnected = Disconnesso
bar-bluetooth-connected-count = { $count ->
    [one] { $count } connesso
    [many] { $count } connessi
   *[other] { $count } connessi
}

## Window Title
bar-window-title-empty = Scrivania

## Idle Inhibit
bar-idle-inhibit-on = Attivo
bar-idle-inhibit-off = Disattivo
bar-idle-policy-off = Il criterio di inattività integrato è disattivato
bar-idle-held-by = Inattività trattenuta da { $holders }
bar-idle-held-by-you = te
bar-idle-all-reached = Tutte le fasi di inattività sono state eseguite
bar-idle-next = Prossima: { $stage } dopo { $after } di inattività
bar-idle-stage-dim = attenuazione
bar-idle-stage-lock = blocco
bar-idle-stage-screen-off = schermi spenti
bar-idle-stage-suspend = sospensione

## Recorder
bar-recorder-idle = Inattivo
bar-recorder-recording = Registrazione
bar-recorder-paused = In pausa
bar-recorder-replay = Replay pronto

## Keybind Mode
bar-keybind-mode-default = predefinita

## Night light
bar-nightlight-on = Attiva
bar-nightlight-off = Disattiva
bar-nightlight-no-location = Pianificazione automatica in pausa, nessuna posizione: { $reason }

## Storage
bar-storage-multiple = Multipli

## Media
bar-media-playing = In riproduzione
bar-media-paused = In pausa
bar-media-stopped = Fermato

## Minimized windows and the dock

bar-task-restore = Ripristina
bar-task-close = Chiudi
bar-task-launch = Apri
bar-task-none = Nessuna finestra minimizzata

## Global shortcuts the shell registers with the compositor

shortcut-launcher = Apri o chiudi l'avviatore

shortcut-dashboard = Apri o chiudi la dashboard

shortcut-bar-hide = Nascondi tutte le barre

shortcut-bar-show = Mostra tutte le barre

shortcut-bar-toggle = Nascondi o mostra tutte le barre

## Module titles: the name a screen reader and the tooltip give each module
bar-module-background-apps = App in background
bar-module-audio = Audio
bar-module-battery = Batteria
bar-module-bitwarden = Bitwarden
bar-module-bluetooth = Bluetooth
bar-module-brightness = Luminosità
bar-module-cava = Visualizzatore audio
bar-module-clock = Orologio
bar-module-cpu = CPU
bar-module-dashboard = Dashboard
bar-module-idle-inhibit = Inibitore di inattività
bar-module-keybind-mode = Modalità scorciatoie
bar-module-keyboard-input = Disposizione tastiera
bar-module-mail = Posta
bar-module-media = Multimedia
bar-module-netstat = Traffico di rete
bar-module-network = Rete
bar-module-nightlight = Luce notturna
bar-module-notifications = Notifiche
bar-module-power = Alimentazione
bar-module-power-profiles = Profilo energetico
bar-module-ram = Memoria
bar-module-recorder = Registratore dello schermo
bar-module-screenshot = Schermata
bar-module-storage = Archiviazione
bar-module-treeman = Worktree
bar-module-weather = Meteo
bar-module-window-title = Titolo della finestra
bar-module-world-clock = Orologio mondiale
bar-module-minimized-windows = Finestre minimizzate
bar-module-separator = Separatore
bar-module-systray = Area di notifica
bar-module-workspaces = Spazi di lavoro
bar-brightness-following = { $level }, automatica
bar-brightness-paused = { $level }, in pausa: segue di nuovo l'ambiente quando riprendi
bar-audio-speaker-muted = { $level }, muto
bar-audio-mic-muted = microfono silenziato
bar-audio-capture-mic = un'app registra dal microfono
bar-audio-capture-camera = un'app usa una fotocamera
bar-audio-capture-screen = un'app condivide lo schermo

## Errori che un modulo mostra sul suo pulsante
bar-module-failed = Errore
bar-custom-command-timed-out = Il comando ha impiegato troppo tempo ed è stato interrotto
bar-custom-command-failed = Impossibile eseguire il comando: { $error }
bar-world-clock-invalid-zone = Fuso orario sconosciuto «{ $zone }»
bar-world-clock-invalid-layout = Formato dell’ora non supportato «{ $layout }»
bar-world-clock-invalid-format = Impossibile mostrare il formato: { $error }
