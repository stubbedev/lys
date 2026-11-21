### Bar Module Labels

## Network
bar-network-connecting = Conectando...
bar-network-disconnected = Desconectado
bar-network-wired = Cableada
bar-network-wifi-fallback = Wi-Fi
bar-network-no-wifi = Sin Wi-Fi
bar-network-no-ethernet = Sin Ethernet
bar-network-offline = Sin conexión

## Battery
bar-battery-unavailable = N/D

## Bluetooth
bar-bluetooth-disabled = Desactivado
bar-bluetooth-disconnected = Desconectado
bar-bluetooth-connected-count = { $count ->
    [one] { $count } conectado
    [many] { $count } conectados
   *[other] { $count } conectados
}

## Window Title
bar-window-title-empty = Escritorio

## Idle Inhibit
bar-idle-inhibit-on = Activado
bar-idle-inhibit-off = Desactivado
bar-idle-policy-off = La política de inactividad integrada está desactivada
bar-idle-held-by = Inactividad retenida por { $holders }
bar-idle-held-by-you = usted
bar-idle-all-reached = Se han ejecutado todas las etapas de inactividad
bar-idle-next = Siguiente: { $stage } tras { $after } de inactividad
bar-idle-stage-keyboard-backlight = apagar luz del teclado
bar-idle-stage-dim = atenuar
bar-idle-stage-lock = bloquear
bar-idle-stage-screen-off = apagar pantallas
bar-idle-stage-suspend = suspender

## Recorder
bar-recorder-idle = Inactivo
bar-recorder-recording = Grabando
bar-recorder-paused = En pausa
bar-recorder-replay = Repetición preparada

## Keybind Mode
bar-keybind-mode-default = predeterminado

## Night light
bar-nightlight-on = Activada
bar-nightlight-off = Desactivada
bar-nightlight-no-location = Programación automática en pausa, sin ubicación: { $reason }

## Storage
bar-storage-multiple = Varios

## Media
bar-media-playing = Reproduciendo
bar-media-paused = En pausa
bar-media-stopped = Detenido

## Minimized windows and the dock

bar-task-restore = Restaurar
bar-task-close = Cerrar
bar-task-launch = Abrir
bar-task-none = No hay nada minimizado

## Global shortcuts the shell registers with the compositor

shortcut-launcher = Abrir o cerrar el lanzador

shortcut-dashboard = Abrir o cerrar el panel de control

shortcut-bar-hide = Ocultar todas las barras

shortcut-bar-show = Mostrar todas las barras

shortcut-bar-toggle = Ocultar o mostrar todas las barras

## Module titles: the name a screen reader and the tooltip give each module
bar-module-background-apps = Aplicaciones en segundo plano
bar-module-audio = Audio
bar-module-battery = Batería
bar-module-bitwarden = Bitwarden
bar-module-bluetooth = Bluetooth
bar-module-brightness = Brillo
bar-module-cava = Visualizador de audio
bar-module-clock = Reloj
bar-module-cpu = CPU
bar-module-dashboard = Panel de control
bar-module-idle-inhibit = Inhibidor de inactividad
bar-module-keybind-mode = Modo de atajos
bar-module-keyboard-input = Distribución del teclado
bar-module-mail = Correo
bar-module-media = Multimedia
bar-module-netstat = Tráfico de red
bar-module-network = Red
bar-module-nightlight = Luz nocturna
bar-module-notifications = Notificaciones
bar-module-power = Energía
bar-module-power-profiles = Perfil de energía
bar-module-ram = Memoria
bar-module-recorder = Grabador de pantalla
bar-module-screenshot = Captura de pantalla
bar-module-storage = Almacenamiento
bar-module-treeman = Árboles de trabajo
bar-module-weather = Tiempo
bar-module-window-title = Título de la ventana
bar-module-world-clock = Reloj mundial
bar-module-minimized-windows = Ventanas minimizadas
bar-module-separator = Separador
bar-module-systray = Bandeja del sistema
bar-module-workspaces = Áreas de trabajo
bar-brightness-following = { $level }, automático
bar-brightness-paused = { $level }, en pausa: vuelve a seguir la luz ambiente al reanudar
bar-audio-speaker-muted = { $level }, silenciado
bar-audio-mic-muted = micrófono silenciado
bar-audio-capture-mic = una aplicación graba el micrófono
bar-audio-capture-camera = una aplicación usa una cámara
bar-audio-capture-screen = una aplicación comparte la pantalla

## Fallos que un módulo muestra en su botón
bar-module-failed = Error
bar-custom-command-timed-out = La orden tardó demasiado y se detuvo
bar-custom-command-failed = No se pudo ejecutar la orden: { $error }
bar-world-clock-invalid-zone = Zona horaria desconocida «{ $zone }»
bar-world-clock-invalid-layout = Formato de hora no admitido «{ $layout }»
bar-world-clock-invalid-format = No se puede mostrar el formato: { $error }

## Desplegables
bar-dropdown-size-not-saved = El desplegable conserva su nuevo tamaño, pero no se ha guardado: { $error }
