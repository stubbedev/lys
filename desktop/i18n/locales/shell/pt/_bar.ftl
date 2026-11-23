### Etiquetas dos módulos da barra

## Rede
bar-network-connecting = A ligar...
bar-network-disconnected = Desligado
bar-network-wired = Com fios
bar-network-wifi-fallback = Wi-Fi
bar-network-no-wifi = Sem Wi-Fi
bar-network-no-ethernet = Sem Ethernet
bar-network-offline = Sem ligação

## Bateria
bar-battery-unavailable = N/D

## Bluetooth
bar-bluetooth-disabled = Desligado
bar-bluetooth-disconnected = Desligado
bar-bluetooth-connected-count = { $count ->
    [one] { $count } ligado
    [many] { $count } ligados
   *[other] { $count } ligados
}

## Título da janela
bar-window-title-empty = Ambiente de trabalho

## Inibidor de inatividade
bar-idle-inhibit-on = Ligado
bar-idle-inhibit-off = Desligado
bar-idle-policy-off = A política de inatividade integrada está desligada
bar-idle-held-by = Inatividade retida por { $holders }
bar-idle-held-by-you = si
bar-idle-all-reached = Todas as fases de inatividade foram executadas
bar-idle-next = Seguinte: { $stage } após { $after } de inatividade
bar-idle-stage-keyboard-backlight = luz do teclado desligada
bar-idle-stage-dim = escurecer
bar-idle-stage-lock = bloquear
bar-idle-stage-screen-off = ecrãs desligados
bar-idle-stage-suspend = suspender

## Gravador
bar-recorder-idle = Inativo
bar-recorder-recording = A gravar
bar-recorder-paused = Em pausa
bar-recorder-replay = Repetição armada

## Modo de atalhos
bar-keybind-mode-default = predefinido

## Luz noturna
bar-nightlight-on = Ligada
bar-nightlight-off = Desligada
bar-nightlight-no-location = Agendamento automático em pausa, sem localização
bar-nightlight-unavailable = A luz noturna não está disponível
bar-nightlight-refused = O ecrã recusou a luz noturna

## Armazenamento
bar-storage-multiple = Vários

## Multimédia
bar-media-playing = A reproduzir
bar-media-paused = Em pausa
bar-media-stopped = Parado

## Janelas minimizadas e a doca

bar-task-restore = Restaurar
bar-task-close = Fechar
bar-task-launch = Abrir
bar-task-none = Nada está minimizado

## Atalhos globais que a shell regista no compositor

shortcut-launcher = Abrir ou fechar o lançador

shortcut-dashboard = Abrir ou fechar o painel

shortcut-bar-hide = Ocultar todas as barras

shortcut-bar-show = Mostrar todas as barras

shortcut-bar-toggle = Ocultar ou mostrar todas as barras

## Títulos dos módulos
bar-module-background-apps = Aplicações em segundo plano
bar-module-audio = Áudio
bar-module-battery = Bateria
bar-module-bitwarden = Bitwarden
bar-module-bluetooth = Bluetooth
bar-module-brightness = Brilho
bar-module-cava = Visualizador de áudio
bar-module-clock = Relógio
bar-module-cpu = CPU
bar-module-dashboard = Painel
bar-module-idle-inhibit = Inibidor de inatividade
bar-module-keybind-mode = Modo de atalhos
bar-module-keyboard-input = Disposição do teclado
bar-module-mail = Correio
bar-module-media = Multimédia
bar-module-netstat = Tráfego de rede
bar-module-network = Rede
bar-module-nightlight = Luz noturna
bar-module-notifications = Notificações
bar-module-power = Energia
bar-module-power-profiles = Perfil de energia
bar-module-ram = Memória
bar-module-recorder = Gravador de ecrã
bar-module-screenshot = Captura de ecrã
bar-module-storage = Armazenamento
bar-module-treeman = Árvores de trabalho
bar-module-weather = Meteorologia
bar-module-window-title = Título da janela
bar-module-world-clock = Relógio mundial
bar-module-minimized-windows = Janelas minimizadas
bar-module-separator = Separador
bar-module-systray = Bandeja do sistema
bar-module-workspaces = Áreas de trabalho
bar-brightness-following = { $level }, automático
bar-brightness-paused = { $level }, em pausa: volta a seguir a luz ambiente quando retomar
bar-audio-speaker-muted = { $level }, silenciado
bar-audio-mic-muted = microfone silenciado
bar-audio-capture-mic = uma aplicação grava o microfone
bar-audio-capture-camera = uma aplicação usa uma câmara
bar-audio-capture-screen = uma aplicação partilha o ecrã

## Falhas que um módulo mostra no seu botão
bar-module-failed = Erro
bar-custom-command-timed-out = O comando demorou demasiado e foi interrompido
bar-custom-command-failed = Não foi possível executar o comando
bar-world-clock-invalid-zone = Fuso horário desconhecido «{ $zone }»
bar-world-clock-invalid-layout = Formato de hora não suportado «{ $layout }»
bar-world-clock-invalid-format = Não é possível mostrar o formato

## Painéis
bar-dropdown-size-not-saved = O painel mantém o novo tamanho, mas não foi guardado
