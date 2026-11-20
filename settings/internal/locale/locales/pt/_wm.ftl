settings-nav-wm-windows = Janelas
settings-page-wm-windows = Janelas
settings-nav-wm-effects = Sombra e desfoque
settings-page-wm-effects = Sombra e desfoque
settings-nav-wm-tiling = Mosaico e abas
settings-page-wm-tiling = Mosaico e abas
settings-nav-wm-binds = Atalhos de teclado
settings-page-wm-binds = Atalhos de teclado
settings-nav-wm-outputs = Ecrãs
settings-page-wm-outputs = Ecrãs
settings-nav-wm-session = Regras e sessão
settings-page-wm-session = Regras e sessão

wm-section-gaps = Espaços
    .description = Espaço à volta e entre as janelas em mosaico
wm-section-borders = Contornos
    .description = A linha desenhada à volta de cada janela
wm-section-shape = Forma e transparência
    .description = Arredondamento dos cantos, e a opacidade das janelas consoante o foco
wm-section-shadow = Sombra
    .description = A sombra sob as janelas
wm-section-blur = Desfoque
    .description = O que se vê através das janelas translúcidas
wm-section-overview = Vista geral
    .description = A vista de todas as áreas de trabalho em simultâneo
wm-section-drag = Pré-visualização do arrasto
    .description = A janela que segue o ponteiro durante o arrasto
wm-section-autotile = Mosaico automático
    .description = Onde abre uma nova janela numa disposição em mosaico
wm-section-tabs = Abas
    .description = A barra sobre os grupos em abas
wm-section-keyboard = Teclado
    .description = Disposição e repetição de teclas
wm-section-pointer = Rato
    .description = Ratos e trackballs
wm-section-touchpad = Painel tátil
    .description = Toque, deslocamento e velocidade
wm-section-trackpoint = Trackpoint
    .description = Pontos de controlo no teclado de um portátil
wm-section-binds = Teclas e gestos
    .description = Combinações de teclas e rato e as ações que executam
wm-section-outputs = Saídas
    .description = Modo, escala, posição e cor de cada ecrã
wm-section-display = Ecrã
    .description = Definições partilhadas por todos os ecrãs
wm-section-rules = Regras de janelas
    .description = Como as janelas correspondentes abrem e se apresentam
wm-section-layer-rules = Painéis e sobreposições
    .description = Como a barra, a doca, os pop-ups e outras superfícies da shell se apresentam
wm-rules-namespace = Superfície
wm-rules-blur-alpha-threshold = Limiar alfa do desfoque
wm-layer-lys-shell-bar = Barra
wm-layer-lys-shell-dock = Doca
wm-layer-lys-shell-wallpaper = Imagem de fundo
wm-layer-lys-shell-launcher = Lançador
wm-layer-lys-shell-notification-popup = Pop-ups de notificação
wm-layer-lys-shell-osd = Indicador de volume e brilho
wm-layer-lys-shell-power-menu = Menu de energia
wm-layer-lys-shell-portal-dialog = Diálogos do portal
wm-layer-lys-shell-file-chooser = Seletor de ficheiros
wm-layer-lys-shell-print = Diálogo de impressão
wm-layer-lys-shell-share-picker = Seletor de partilha de ecrã
wm-layer-lys-shell-screenshot-reading = Reconhecimento de texto
wm-layer-lys-shell-screenshot-thumbnail = Miniatura da captura
wm-layer-lys-shell-color-picker = Seletor de cor
wm-layer-lys-shell-region-overlay = Seleção de região
wm-layer-lys-shell-window-pick = Seletor de janela
wm-section-start = Arranque
    .description = O que é executado com a sessão, e o seu ambiente
wm-section-xwayland = Xwayland
    .description = Aplicações X11
wm-section-palette = Paleta
    .description = Nomes de cores que as outras chaves podem usar

enum-wm-toggle-on = Ligado
enum-wm-toggle-off = Desligado
wm-status-not-running = Guardado. Nenhum lys-wm está em execução aqui; o próximo irá lê-lo.
wm-pointers-connected = Ligados: { $devices }
wm-pointers-device = { $name } ({ $kind })
wm-pointers-class-pointer = rato
wm-pointers-class-touchpad = touchpad
wm-pointers-class-trackpoint = trackpoint
wm-pointers-none = Não há nenhum rato, touchpad ou trackpoint ligado.
wm-pointers-not-running = Não há nenhum lys-wm em execução aqui que diga que dispositivos estão ligados, por isso todas as secções são mostradas.
wm-pointers-outdated = O lys-wm em execução é mais antigo e não diz que dispositivos estão ligados, por isso todas as secções são mostradas até voltar a iniciar sessão.
wm-pointers-unreadable = Não foi possível ler que dispositivos estão ligados, por isso todas as secções são mostradas: { $reason }
wm-rules-pick = Escolher uma janela…
wm-rules-picking = Clique na janela para a qual quer criar uma regra. Escape cancela.
wm-rules-pick-none = O clique não foi numa janela.
wm-rules-pick-gone = A janela fechou-se antes de poder ser lida.
wm-rules-pick-unnamed = Essa janela não tem nem ID de aplicação nem título com que corresponder.
wm-rules-pick-not-running = Não há nenhum lys-wm em execução aqui para escolher uma janela.
wm-rules-pick-failed = Não foi possível escolher a janela: { $reason }
wm-rules-applies-to = Aplica-se a
wm-rules-then = Então
wm-rules-more-conditions = Mais condições
wm-rules-fewer-conditions = Menos condições
wm-rules-group-placement = Posicionamento
wm-rules-group-appearance = Aspeto
wm-rules-group-behaviour = Comportamento
wm-rules-group-performance = Desempenho
wm-rules-add-effect = Adicionar efeito…
wm-rules-no-effects = Esta regra ainda não altera nada: adicione um efeito.
wm-rules-remove-effect = Remover este efeito
wm-rules-enabled = Aplicar esta regra
wm-rules-apps-running = Em execução
wm-rules-border = Contorno
wm-rules-blur-look = Aspeto do desfoque
wm-preview-windows = Pré-visualização das janelas
wm-preview-unreadable = A pré-visualização não consegue desenhar { $setting } com o valor { $value }.
wm-source-file = 
    .description = Definido no ficheiro de configuração do lys-wm

wm-gaps-inner = Espaço interior
    .description = Píxeis entre as janelas em mosaico
    .keywords = espaçamento, preenchimento, margem, spacing, padding
wm-gaps-outer = Espaço exterior
    .description = Um número, ou [topo, direita, fundo, esquerda]
wm-borders-width = Largura do contorno
    .description = Em píxeis; 0 não desenha nenhum
wm-borders-active = Contorno com foco
    .description = Uma cor, ou cores e um ângulo para um gradiente
wm-borders-inactive = Contorno sem foco
    .description = Uma cor, ou cores e um ângulo para um gradiente
wm-decoration-rounding = Raio dos cantos
    .description = Em píxeis
wm-decoration-active-opacity = Opacidade com foco
    .description = De 0 a 1
wm-decoration-inactive-opacity = Opacidade sem foco
    .description = De 0 a 1
wm-decoration-fullscreen-opacity = Opacidade em ecrã inteiro
    .description = De 0 a 1
wm-decoration-dim-inactive = Escurecer janelas sem foco
    .description = Escurecer todas as janelas sem foco
wm-decoration-dim-strength = Intensidade do escurecimento
    .description = De 0 a 1
wm-decoration-shadow-enabled = Sombras
    .description = Desenhar uma sombra sob as janelas
wm-decoration-shadow-range = Alcance da sombra
    .description = Em píxeis
wm-decoration-shadow-render-power = Atenuação da sombra
    .description = De 1 a 4; mais alto esbate mais depressa
wm-decoration-shadow-sharp = Sombras nítidas
    .description = Sem margem suave
wm-decoration-shadow-scale = Escala da sombra
    .description = Acima de 0, até 1
wm-decoration-shadow-offset = Deslocamento da sombra
    .description = Quanto a sombra cai à direita e abaixo da janela, em píxeis; um valor negativo desloca-a para a esquerda ou para cima
wm-shadow-offset-x = Para a direita
wm-shadow-offset-y = Para baixo
wm-decoration-shadow-color = Cor da sombra
    .description = #rrggbb[aa] ou um nome da paleta; também define a cor sem foco, a menos que esta esteja definida
wm-decoration-shadow-color-inactive = Cor da sombra sem foco
    .description = #rrggbb[aa] ou um nome da paleta
wm-decoration-blur-enabled = Desfoque
    .description = Desfocar por trás de todas as janelas translúcidas, não só das que o pedem
    .keywords = vidro fosco, transparência, translúcido
wm-decoration-blur-size = Tamanho do desfoque
    .description = Em píxeis
wm-decoration-blur-passes = Passagens do desfoque
    .description = De 1 a 4
wm-decoration-blur-xray = Desfoque raio-X
    .description = Desfocar apenas a imagem de fundo, não as janelas por baixo
wm-decoration-blur-tint = Tonalidade do desfoque
    .description = #rrggbb[aa] ou um nome da paleta
wm-decoration-blur-saturation = Saturação do desfoque
    .description = De 0 a 4; 1 mantém inalterado
wm-decoration-blur-brightness = Brilho do desfoque
    .description = De 0 a 4; 1 mantém inalterado
wm-decoration-blur-contrast = Contraste do desfoque
    .description = De 0 a 4; 1 mantém inalterado
wm-decoration-blur-grain = Grão do desfoque
    .description = De 0 a 1
wm-animations-enabled = Animações
    .description = Animar janelas, áreas de trabalho e camadas
wm-animations-reduced-motion = Movimento reduzido
    .description = Esbater em vez de mover
    .keywords = acessibilidade, enjoo de movimento
wm-overview-blur = Desfocar a vista geral
    .description = Desfocar o que está por trás da vista geral
wm-overview-dim = Escurecimento da vista geral
    .description = De 0 a 1
wm-drag-preview-enabled = Pré-visualização do arrasto
    .description = Mostrar onde uma janela arrastada vai ficar
wm-drag-preview-opacity = Opacidade da pré-visualização
    .description = De 0 a 1
wm-drag-preview-blur = Desfocar a pré-visualização
    .description = Desfocar por trás da pré-visualização
wm-drag-preview-snap-distance = Distância de ajuste
    .description = Em píxeis a partir de uma margem
wm-autotile-enabled = Mosaico automático
    .description = Dividir uma janela pelo seu lado mais comprido
wm-autotile-ephemeral = Grupos efémeros
    .description = Um grupo de uma só janela dissolve-se
wm-autotile-min-trigger = Tamanho mínimo para dividir
    .description = Em píxeis
wm-autotile-divisor = Divisor de divisão
    .description = As janelas mais estreitas do que esta fração da saída deixam de se dividir
wm-tabs-height = Altura da barra de abas
    .description = Em píxeis
wm-tabs-padding = Preenchimento da barra de abas
    .description = Em píxeis
wm-tabs-text-height = Altura do texto das abas
    .description = Em píxeis
wm-tabs-text-padding = Preenchimento do texto das abas
    .description = Em píxeis
wm-tabs-font = Tipo de letra das abas
    .description = Caminho para um ficheiro de tipo de letra; vazio usa a predefinição
wm-tabs-icon-theme = Tema de ícones das abas
    .description = Vazio usa a predefinição
wm-input-keyboard-layout = Disposições do teclado
    .description = As disposições com que escreve, pela ordem em que se alternam; acrescente uma pelo nome, incluindo variantes como Dinamarquês (sem teclas mortas)
    .keywords = mapa de teclado, idioma, fonte de entrada, keymap
wm-input-sources-misaligned = Estas disposições e variantes não formam pares, há mais variantes do que disposições, por isso ambas são mostradas tal como estão escritas; escolha uma disposição ou variante para as definir de novo
wm-input-keyboard-variant = Variante do teclado
    .description = Variante XKB
wm-input-keyboard-options = Opções do teclado
    .description = Opções XKB, como caps:escape
wm-input-keyboard-model = Modelo do teclado
    .description = Modelo XKB
wm-input-keyboard-rules = Regras do teclado
    .description = Regras XKB
wm-input-keyboard-repeat-rate = Taxa de repetição
    .description = Teclas por segundo
    .keywords = repetição de teclas, typematic
wm-input-keyboard-repeat-delay = Atraso da repetição
    .description = Milissegundos antes de uma tecla premida começar a repetir
wm-input-pointer-natural-scroll = Deslocamento natural
    .description = O conteúdo segue os dedos
    .keywords = deslocamento inverso, deslocamento invertido, roda do rato
wm-input-pointer-middle-emulation = Emulação do clique do meio
    .description = Esquerdo e direito em simultâneo clicam no botão do meio
wm-input-pointer-accel-profile = Perfil de aceleração
    .description = Plano move o ponteiro tanto quanto o dispositivo; adaptativo acelera-o com movimentos mais rápidos
wm-input-pointer-accel-speed = Velocidade de aceleração
    .description = De -1 a 1
wm-input-pointer-scroll-factor = Velocidade de deslocamento
    .description = Acima de 0, até 100; 1 mantém inalterado
wm-input-trackpoint-natural-scroll = Deslocamento natural
    .description = O conteúdo segue os dedos
    .keywords = deslocamento inverso, deslocamento invertido, roda do rato
wm-input-trackpoint-middle-emulation = Emulação do clique do meio
    .description = Esquerdo e direito em simultâneo clicam no botão do meio
wm-input-trackpoint-accel-profile = Perfil de aceleração
    .description = Plano move o ponteiro tanto quanto o dispositivo; adaptativo acelera-o com movimentos mais rápidos
wm-input-trackpoint-accel-speed = Velocidade de aceleração
    .description = De -1 a 1
wm-input-trackpoint-scroll-factor = Velocidade de deslocamento
    .description = Acima de 0, até 100; 1 mantém inalterado
wm-input-trackpoint-scroll-method = Método de deslocamento
    .description = Como o trackpoint desloca: com um botão premido, ou de forma nenhuma
wm-input-touchpad-natural-scroll = Deslocamento natural
    .description = O conteúdo segue os dedos
    .keywords = deslocamento inverso, deslocamento invertido, trackpad
wm-input-touchpad-middle-emulation = Emulação do clique do meio
    .description = Esquerdo e direito em simultâneo clicam no botão do meio
wm-input-touchpad-accel-profile = Perfil de aceleração
    .description = Plano move o ponteiro tanto quanto o dedo; adaptativo acelera-o com movimentos mais rápidos
wm-input-touchpad-accel-speed = Velocidade de aceleração
    .description = De -1 a 1
wm-input-touchpad-scroll-factor = Velocidade de deslocamento
    .description = Acima de 0, até 100; 1 mantém inalterado
wm-input-touchpad-tap = Tocar para clicar
    .description = Um toque clica, sem premir o painel tátil
    .keywords = tocar para clicar, trackpad
wm-input-touchpad-drag-lock = Bloqueio de arrasto
    .description = Um arrasto com toque mantém-se ao levantar o dedo
wm-input-touchpad-disable-while-typing = Desativar ao escrever
    .description = Ignorar o painel tátil durante um momento após cada tecla premida
wm-input-touchpad-click-method = Método de clique
    .description = Como um toque escolhe o botão: pelo sítio onde cai, ou pelo número de dedos
wm-input-touchpad-scroll-method = Método de deslocamento
    .description = Como o painel tátil desloca: dois dedos, a margem, ou um botão premido
wm-binds = Atalhos
    .description = Uma lista de combinação, ação e opções; uma configuração sem nenhum não tem nenhum
wm-submaps = Submapas
    .description = Conjuntos de atalhos com nome
wm-gestures = Gestos
    .description = Deslizes, beliscões e toques prolongados no painel tátil
wm-outputs = Regras de saídas
    .description = Cada saída usa a primeira regra que lhe corresponde
wm-color-management = Gestão de cor
    .description = Oferecer aos clientes o protocolo de gestão de cor
wm-tearing = Tearing
    .description = Permitir tearing em ecrã inteiro às janelas que o pedem
wm-gpu = GPU
    .description = Uma placa ou nó de renderização a usar; vazio escolhe um
wm-lid-action = Ao fechar a tampa
    .description = O que fechar a tampa faz na bateria sem ecrã externo ligado: suspender, hibernar, suspensão híbrida, suspender e depois hibernar, ou nada
wm-rules = Regras
    .description = Uma lista de correspondência e efeito
wm-layer-rules = Regras
    .description = Cada regra altera o aspeto das superfícies da shell a que se aplica
wm-autostart = Arranque automático
    .description = Comandos executados quando a sessão está pronta
wm-env = Ambiente
    .description = Variáveis para o lys-wm e as aplicações que inicia; uma alteração chega às aplicações iniciadas depois, não às que já estão abertas
wm-env-name-placeholder = Variável, como GDK_BACKEND
wm-env-name-invalid = Um nome de variável só tem letras, algarismos e _, e não começa por um algarismo; não é guardado até que assim seja
wm-env-moz-wayland = O Firefox corre em Wayland
wm-env-qt-wayland = As apps Qt correm em Wayland
wm-env-gdk-wayland = As apps GTK correm em Wayland
wm-env-sdl-wayland = Os jogos SDL correm em Wayland
wm-env-electron-wayland = As apps Electron correm em Wayland quando podem
wm-env-java-tiling = As apps Java desenham bem as suas janelas
wm-xwayland-enabled = Xwayland
    .description = Executar aplicações X11
wm-xwayland-zero-scaling = Sem escala
    .description = As aplicações X11 desenham nos píxeis reais do ecrã em vez de serem ampliadas e desfocadas; num ecrã escalado parecem pequenas, a menos que se escalem sozinhas
wm-palette = Paleta
    .description = Cores com nome que as outras chaves podem usar

wm-section-gestures = Gestos
    .description = Gestos do painel tátil e o que controlam
wm-gestures-swipe = Deslizes
    .description = Deslizes no painel tátil e o que cada um controla ou executa
wm-gestures-pinch = Beliscões
    .description = Beliscões no painel tátil e o que cada um controla ou executa
wm-gestures-hold = Toques prolongados
    .description = Dedos mantidos no painel tátil e a ação que cada um executa
wm-gestures-distance = Distância do gesto
    .description = Unidades do painel tátil para um deslize completo, cerca de 12 por milímetro
wm-card-number = Item { $number }
wm-side-top = Topo
wm-side-right = Direita
wm-side-bottom = Fundo
wm-side-left = Esquerda
wm-border-add-color = Adicionar uma cor
wm-border-angle = Ângulo
wm-binds-action = Ação
wm-binds-chord = Combinação
wm-binds-flags = Opções
wm-binds-cooldown = Tempo de espera (ms)
wm-binds-description = Descrição
wm-binds-long-press = Pressão longa (ms)
wm-gestures-action = Ação
wm-gestures-direction = Direção
wm-gestures-drive = Controla
wm-gestures-fingers = Dedos
wm-gestures-modifiers = Modificadores
wm-gestures-scope = Âmbito da vista geral
wm-outputs-bitdepth = Profundidade de bits
    .description = 8 ou 10 bits por canal; 10 suaviza os gradientes com a luz noturna
wm-outputs-disabled = Desativado
wm-outputs-hdr = HDR
    .description = Usar o ecrã em HDR sempre, ou apenas para conteúdo HDR em ecrã inteiro
wm-outputs-hdr-peak = Brilho máximo HDR
    .description = Com HDR: o brilho máximo do ecrã em cd/m², se o EDID o indicar errado
wm-outputs-icc = Perfil ICC
    .description = Um perfil ICC matrix-shaper para um ecrã SDR
wm-outputs-hot-corners = Cantos ativos
    .description = Desligado desativa todos os cantos ativos neste ecrã; Ligado altera alguns deles aqui
wm-outputs-hot-corners-top-left = Canto superior esquerdo
wm-outputs-hot-corners-top-left-action = Ação
wm-outputs-hot-corners-top-left-modifiers = Modificadores
wm-outputs-hot-corners-top-right = Canto superior direito
wm-outputs-hot-corners-top-right-action = Ação
wm-outputs-hot-corners-top-right-modifiers = Modificadores
wm-outputs-hot-corners-bottom-left = Canto inferior esquerdo
wm-outputs-hot-corners-bottom-left-action = Ação
wm-outputs-hot-corners-bottom-left-modifiers = Modificadores
wm-outputs-hot-corners-bottom-right = Canto inferior direito
wm-outputs-hot-corners-bottom-right-action = Ação
wm-outputs-hot-corners-bottom-right-modifiers = Modificadores
wm-outputs-match = Correspondência
wm-outputs-mirror = Espelhar
    .description = Outro ecrã cuja imagem este mostra em vez do seu próprio ambiente de trabalho
wm-outputs-mode = Modo
wm-outputs-position = Posição
wm-outputs-reserved = Espaço reservado
    .description = Píxeis lógicos mantidos livres de janelas em mosaico e maximizadas: topo, direita, fundo, esquerda
wm-outputs-scale = Escala
wm-outputs-sdr-brightness = Brilho SDR
    .description = Com HDR: os cd/m² com que o branco SDR é mostrado
wm-outputs-shader = Shader
    .description = Um fragment shader para a imagem final do ecrã
wm-outputs-transform = Rotação
    .description = Como o painel está rodado ou invertido
wm-outputs-vrr = Taxa de atualização variável
    .description = Sincronização adaptativa: desligada, sempre ligada, ou apenas para um jogo ou vídeo em ecrã inteiro
wm-rules-animation = Animação
wm-rules-animation-in = Animação de abertura
wm-rules-animation-out = Animação de fecho
wm-rules-app-id = ID da aplicação
wm-rules-block-out-from = Ocultar das capturas
wm-rules-blur = Desfoque
wm-rules-blur-look-brightness = Brilho do desfoque
wm-rules-blur-look-contrast = Contraste do desfoque
wm-rules-blur-look-grain = Grão do desfoque
wm-rules-blur-look-refraction-amount = Quantidade de refração
wm-rules-blur-look-refraction-bezel = Moldura de refração
wm-rules-blur-look-saturation = Saturação do desfoque
wm-rules-blur-look-specular-strength = Intensidade especular
wm-rules-blur-look-specular-width = Largura especular
wm-rules-blur-look-tint = Tonalidade do desfoque
wm-rules-border-active = Contorno ativo
wm-rules-border-inactive = Contorno inativo
wm-rules-border-width = Largura do contorno
wm-rules-dim-around = Escurecer à volta
wm-rules-dim-inactive = Escurecer quando inativa
wm-rules-float = Flutuante
wm-rules-mica = Mica
wm-rules-opacity = Opacidade
wm-rules-size = Tamanho
wm-rules-tag = Etiqueta
wm-rules-tearing = Tearing
wm-rules-touch-gestures = Gestos táteis
wm-rules-tile = Mosaico
wm-rules-vrr = Taxa de atualização variável
wm-rules-title = Título
wm-rules-exclude-app-id = Exceto ID da aplicação
wm-rules-exclude-is-captured = Exceto capturadas
wm-rules-exclude-is-floating = Exceto flutuantes
wm-rules-exclude-is-focused = Exceto com foco
wm-rules-exclude-is-fullscreen = Exceto em ecrã inteiro
wm-rules-exclude-is-urgent = Exceto urgentes
wm-rules-exclude-initial-app-id = Exceto ID inicial da aplicação
wm-rules-exclude-initial-title = Exceto título inicial
wm-rules-exclude-tag = Exceto etiqueta
wm-rules-exclude-title = Exceto título
wm-rules-exclude-xwayland = Exceto Xwayland
wm-rules-focus = Foco
wm-rules-fullscreen = Ecrã inteiro
wm-rules-group = Grupo
wm-rules-idle-inhibit = Inibir inatividade
wm-rules-initial-app-id = ID inicial da aplicação
wm-rules-initial-title = Título inicial
wm-rules-is-captured = Está capturada
wm-rules-is-floating = É flutuante
wm-rules-is-focused = Tem foco
wm-rules-is-fullscreen = Está em ecrã inteiro
wm-rules-is-urgent = É urgente
wm-rules-keep-aspect = Manter proporção
wm-rules-max-size = Tamanho máximo
wm-rules-min-size = Tamanho mínimo
wm-rules-maximized = Maximizada
wm-rules-output = Saída
wm-rules-pin = Afixar
wm-rules-position = Posição
wm-rules-rounding = Raio dos cantos
wm-rules-scroll-factor = Fator de deslocamento
wm-rules-shadow = Sombra
wm-rules-suppress = Pedidos ignorados
wm-rules-workspace = Área de trabalho
wm-rules-xwayland = Xwayland
wm-section-focus = Foco
    .description = O que recebe uma janela que pede foco
wm-focus-on-activate = Na ativação
    .description = Dar foco a uma janela que o pede logo após ter usado a aplicação que pediu, caso contrário marcá-la como urgente; ou marcá-la sempre como urgente
wm-rules-on-activate = Na ativação
    .description = O que esta janela recebe quando pede foco: foco, urgente, ou nada
wm-section-app-shortcuts = Atalhos das aplicações
    .description = Atalhos que as aplicações registaram através do portal, cada um acionado por um atalho global
wm-app-shortcuts = Atalhos registados
    .description = Grave uma combinação para a atribuir, Backspace para a remover
wm-app-shortcuts-none = Nenhuma aplicação registou ainda um atalho
wm-app-shortcuts-hotkey = A própria aplicação detém esta tecla, definida no seu diálogo do portal
wm-rules-content = Conteúdo
    .description = O que a janela mostra (foto, vídeo ou jogo), em vez do que ela própria indica
wm-app-shortcuts-reload = Procurar novamente
wm-section-window-behavior = Comportamento das janelas
    .description = Como se comportam as janelas novas, flutuantes e em ecrã inteiro
wm-section-titlebars = Barras de título
    .description = Barras com botões sobre as janelas flutuantes
wm-section-mica = Mica
    .description = Uma desfocagem intensa do fundo atrás das janelas translúcidas
wm-section-hot-corners = Cantos ativos
    .description = O que acontece ao empurrar o ponteiro para um canto do ecrã
wm-section-layout = Disposição
    .description = Como uma área de trabalho nova organiza as janelas
wm-section-cursor = Ponteiro
    .description = Quando o ponteiro se esconde e como encontrá-lo
wm-focus-follows-mouse = O foco segue o ponteiro
    .description = Passar sobre uma janela dá-lhe o foco; flexível espera que o ponteiro entre noutra janela
wm-focus-raise-on-hover = Elevar ao passar
    .description = O foco ao passar também traz uma janela flutuante para a frente
wm-focus-warp-pointer = Mover o ponteiro com o foco
    .description = Centrar o ponteiro na janela que um atalho foca, ou só ao mudar de área de trabalho
wm-focus-on-close = Foco ao fechar
    .description = Que janela recebe o foco quando a janela focada fecha
wm-fullscreen-new-window = Nova janela sobre ecrã inteiro
    .description = Se uma janela que abre sobre outra em ecrã inteiro ou maximizada fica atrás, toma o lugar ou termina-a
wm-hold-opening = Esperar pelas vizinhas
    .description = Uma janela nova espera até 100 ms que as vizinhas redimensionadas se redesenhem
wm-floating-step = Passo de movimento flutuante
    .description = Píxeis que um atalho move uma janela flutuante
wm-titlebars-floating = Mostrar barras de título
    .description = Barras com botões sobre as janelas flutuantes
wm-titlebars-height = Altura da barra
    .description = Píxeis, retirados da janela flutuante
wm-titlebars-buttons = Botões
    .description = Que botões a barra mostra, por ordem
wm-titlebars-side = Lado dos botões
    .description = Em que extremo da barra ficam os botões
wm-titlebars-double-click = Duplo clique
    .description = O que faz um duplo clique na barra
wm-titlebars-active-background = Fundo de barra de título, focada
    .description = Cor: barra de título, focada
wm-titlebars-active-text = Texto de barra de título, focada
    .description = Cor do texto: barra de título, focada
wm-titlebars-inactive-background = Fundo de barra de título, sem foco
    .description = Cor: barra de título, sem foco
wm-titlebars-inactive-text = Texto de barra de título, sem foco
    .description = Cor do texto: barra de título, sem foco
wm-titlebars-urgent-background = Fundo de barra de título, urgente
    .description = Cor: barra de título, urgente
wm-titlebars-urgent-text = Texto de barra de título, urgente
    .description = Cor do texto: barra de título, urgente
wm-tabs-active-background = Fundo de separador, focada
    .description = Cor: separador, focada
wm-tabs-active-text = Texto de separador, focada
    .description = Cor do texto: separador, focada
wm-tabs-inactive-background = Fundo de separador, sem foco
    .description = Cor: separador, sem foco
wm-tabs-inactive-text = Texto de separador, sem foco
    .description = Cor do texto: separador, sem foco
wm-tabs-urgent-background = Fundo de separador, urgente
    .description = Cor: separador, urgente
wm-tabs-urgent-text = Texto de separador, urgente
    .description = Cor do texto: separador, urgente
wm-titlebars-lights-close = Cor de Fechar
    .description = Cor do ponto do botão Fechar
wm-titlebars-lights-minimize = Cor de Minimizar
    .description = Cor do ponto do botão Minimizar
wm-titlebars-lights-maximize = Cor de Maximizar
    .description = Cor do ponto do botão Maximizar
wm-decoration-blur-live-tiled = Desfocagem ao vivo em mosaico
    .description = Desfocar o que está atrás de cada janela em mosaico em vez da desfocagem partilhada
wm-decoration-blur-mica-enabled = Mica
    .description = Desenhar as janelas translúcidas sobre uma desfocagem intensa do fundo
wm-decoration-blur-mica-size = Tamanho da desfocagem mica
    .description = Distância entre amostras de desfocagem, em píxeis
wm-decoration-blur-mica-passes = Passagens da desfocagem mica
    .description = Cada passagem duplica o alcance
wm-decoration-blur-mica-tint = Tonalidade mica
    .description = Cor sobre o fundo mica
wm-drag-preview-snap-windows = Encaixar em janelas
    .description = As janelas flutuantes também encaixam ao lado ou alinhadas com outras
wm-drag-preview-drag-switch-width = Zona de mudança na margem
    .description = Píxeis numa margem sem vizinha onde segurar um arrasto muda de área
wm-drag-preview-drag-switch-frames = Atraso de mudança na margem
    .description = Fotogramas que o arrasto tem de ficar na margem
wm-hot-corners-top-left = Canto superior esquerdo
    .description = Ação ao empurrar o ponteiro para este canto
wm-hot-corners-top-right = Canto superior direito
    .description = Ação ao empurrar o ponteiro para este canto
wm-hot-corners-bottom-left = Canto inferior esquerdo
    .description = Ação ao empurrar o ponteiro para este canto
wm-hot-corners-bottom-right = Canto inferior direito
    .description = Ação ao empurrar o ponteiro para este canto
wm-hot-corners-corner-action = Ação
    .description = O que o canto faz
wm-hot-corners-corner-modifiers = Teclas premidas
    .description = Teclas que têm de estar premidas
wm-hot-corners-pressure = Pressão no canto
    .description = Quanto o ponteiro tem de empurrar para o canto
wm-hot-corners-fullscreen = Cantos em ecrã inteiro
    .description = Os cantos também atuam com uma janela em ecrã inteiro focada
wm-showdesktop-peek = Margem ao mostrar a secretária
    .description = Percentagem de cada janela que fica visível ao mostrar a secretária
wm-switcher-open-delay = Atraso do seletor de janelas
    .description = Milissegundos que as teclas têm de estar premidas
wm-layout = Disposição de áreas novas
    .description = Divisões e grupos de separadores, ou uma faixa de colunas deslizante
wm-scroll-center = Centrar a coluna focada
    .description = Quando a faixa desloca a coluna focada para o centro
wm-input-keyboard-numlock = Num Lock ligado
    .description = Ligar o Num Lock no arranque e após cada recarregamento
wm-input-keyboard-sticky-keys = Teclas fixas
    .description = Um modificador premido sozinho aplica-se à tecla seguinte; duas vezes bloqueia-o
wm-input-keyboard-slow-keys = Teclas lentas
    .description = Milissegundos que uma tecla tem de estar premida; 0 desliga
wm-input-pointer-tap = Tocar para clicar
    .description = Um toque clica, sem pressionar
wm-input-pointer-drag-lock = Bloqueio de arrasto
    .description = Manter o arrasto depois de levantar o dedo
wm-input-pointer-disable-while-typing = Desligado ao escrever
    .description = Ignorar o dispositivo enquanto se escreve
wm-input-pointer-disabled-on-external-mouse = Desligado com rato ligado
    .description = Desligar este dispositivo enquanto houver outro ponteiro ligado
wm-input-pointer-left-handed = Canhoto
    .description = Trocar os botões principal e secundário
wm-input-pointer-click-method = Método de clique
    .description = Como um clique é lido: zonas de botão ou número de dedos
wm-input-pointer-scroll-method = Método de deslocamento
    .description = Dois dedos, a margem ou um botão premido
wm-input-pointer-scroll-button-lock = Botão de deslocamento alterna
    .description = Um clique no botão de deslocamento liga e desliga
wm-input-pointer-tap-button-map = Botões ao tocar
    .description = Que botões os toques com um, dois e três dedos premem
wm-input-touchpad-disabled-on-external-mouse = Desligado com rato ligado
    .description = Desligar este dispositivo enquanto houver outro ponteiro ligado
wm-input-touchpad-left-handed = Canhoto
    .description = Trocar os botões principal e secundário
wm-input-touchpad-scroll-button-lock = Botão de deslocamento alterna
    .description = Um clique no botão de deslocamento liga e desliga
wm-input-touchpad-tap-button-map = Botões ao tocar
    .description = Que botões os toques com um, dois e três dedos premem
wm-input-trackpoint-tap = Tocar para clicar
    .description = Um toque clica, sem pressionar
wm-input-trackpoint-drag-lock = Bloqueio de arrasto
    .description = Manter o arrasto depois de levantar o dedo
wm-input-trackpoint-disable-while-typing = Desligado ao escrever
    .description = Ignorar o dispositivo enquanto se escreve
wm-input-trackpoint-disabled-on-external-mouse = Desligado com rato ligado
    .description = Desligar este dispositivo enquanto houver outro ponteiro ligado
wm-input-trackpoint-left-handed = Canhoto
    .description = Trocar os botões principal e secundário
wm-input-trackpoint-click-method = Método de clique
    .description = Como um clique é lido: zonas de botão ou número de dedos
wm-input-trackpoint-scroll-button-lock = Botão de deslocamento alterna
    .description = Um clique no botão de deslocamento liga e desliga
wm-input-trackpoint-tap-button-map = Botões ao tocar
    .description = Que botões os toques com um, dois e três dedos premem
wm-gestures-edge-width = Zona tátil da margem
    .description = Píxeis a partir da margem onde começa um deslize
wm-cursor-hide-while-typing = Esconder ao escrever
    .description = Esconder o ponteiro enquanto se escreve numa janela
wm-cursor-hide-after = Esconder em repouso
    .description = Milissegundos sem movimento antes de esconder; 0 nunca
wm-cursor-shake-to-find = Agitar para encontrar
    .description = Agitar o rato aumenta o ponteiro por instantes
wm-hidden-frame-rate = Taxa de janelas escondidas
    .description = Fotogramas por segundo para janelas tapadas ou fora do ecrã
wm-power-saver-frame-rate = Taxa em poupança de energia
    .description = Taxa máxima com o perfil de poupança; 0 desliga
wm-client-hotkeys = Atalhos globais de apps
    .description = Permitir que as apps definam atalhos globais que a janela focada não vê
wm-side-all = Todos os lados
wm-side-same = Igual em todos os lados
wm-side-window = Janela
enum-wm-accel-profile-adaptive = Adaptativa
enum-wm-accel-profile-flat = Plana
enum-wm-center-always = Sempre
enum-wm-center-never = Nunca
enum-wm-center-on-overflow = Quando transborda
enum-wm-click-method-button_areas = Áreas de botão
enum-wm-click-method-clickfinger = Número de dedos
enum-wm-click-method-none = Nenhum
enum-wm-double-click-float = Tornar flutuante
enum-wm-double-click-maximize = Maximizar
enum-wm-follows-mouse-always = Sempre sob o ponteiro
enum-wm-follows-mouse-loose = Quando o ponteiro entra noutra janela
enum-wm-follows-mouse-off = Clicar para focar
enum-wm-layout-scroll = Colunas deslizantes
enum-wm-layout-tree = Árvore em mosaico
enum-wm-lid-action-hibernate = Hibernar
enum-wm-lid-action-hybrid-sleep = Suspensão híbrida
enum-wm-lid-action-ignore = Não fazer nada
enum-wm-lid-action-suspend = Suspender
enum-wm-lid-action-suspend-then-hibernate = Suspender e depois hibernar
enum-wm-new-window-behind = Abre por trás
enum-wm-new-window-exit = Sai do ecrã inteiro
enum-wm-new-window-take_over = Assume o ecrã inteiro
enum-wm-on-activate-focus = Focá-la
enum-wm-on-activate-urgent = Marcá-la como urgente
enum-wm-on-close-next = A janela seguinte
enum-wm-on-close-under_pointer = A janela sob o ponteiro
enum-wm-scroll-method-edge = Borda
enum-wm-scroll-method-none = Nenhum
enum-wm-scroll-method-on_button_down = Com um botão premido
enum-wm-scroll-method-two_finger = Dois dedos
enum-wm-side-left = Esquerda
enum-wm-side-right = Direita
enum-wm-tap-button-map-lmr = Esquerdo, meio, direito
enum-wm-tap-button-map-lrm = Esquerdo, direito, meio
enum-wm-warp-pointer-off = Nunca
enum-wm-warp-pointer-on_keyboard_focus = Quando o teclado move o foco
enum-wm-warp-pointer-on_workspace_switch = Ao mudar de área de trabalho
settings-nav-wm-keyboard = Teclado
settings-page-wm-keyboard = Teclado
settings-nav-wm-pointer = Rato e painel tátil
settings-page-wm-pointer = Rato e painel tátil
wm-section-typing = Escrita
    .description = A rapidez com que as teclas premidas se repetem e o Num Lock ao iniciar
wm-section-keyboard-accessibility = Acessibilidade
    .description = Teclas que ficam premidas ou esperam, para escrever uma tecla de cada vez
wm-section-keyboard-model = Modelo de teclado
    .description = O modelo e as regras de teclado para os quais as disposições são construídas
wm-palette-new-name = Nome da nova cor
wm-palette-add = Adicionar cor
wm-palette-reset = Voltar à cor do lys-wm
wm-palette-taken = Já existe uma cor chamada { $name }

## What a bind does, by lys-wm action: $first is the first argument, or default without any; $args all of them; $rest all after the first
wm-action-focus = { $first ->
        [left] Focar a janela à esquerda
        [right] Focar a janela à direita
        [up] Focar a janela acima
        [down] Focar a janela abaixo
       *[other] Focar { $args }
    }
wm-action-move = { $first ->
        [left] Mover a janela para a esquerda
        [right] Mover a janela para a direita
        [up] Mover a janela para cima
        [down] Mover a janela para baixo
       *[other] Mover a janela { $args }
    }
wm-action-workspace = { $first ->
        [previous] Mudar para a área de trabalho anterior
        [empty] Mudar para uma área de trabalho vazia
       *[other] Mudar para a área de trabalho { $args }
    }
wm-action-movetoworkspace = { $first ->
        [previous] Mover a janela para a área de trabalho anterior
        [empty] Mover a janela para uma área de trabalho vazia
       *[other] Mover a janela para a área de trabalho { $args }
    }
wm-action-focusoutput = { $first ->
        [left] Focar o ecrã à esquerda
        [right] Focar o ecrã à direita
        [up] Focar o ecrã acima
        [down] Focar o ecrã abaixo
       *[other] Focar o ecrã { $args }
    }
wm-action-movetooutput = { $first ->
        [left] Mover a janela para o ecrã à esquerda
        [right] Mover a janela para o ecrã à direita
        [up] Mover a janela para o ecrã acima
        [down] Mover a janela para o ecrã abaixo
       *[other] Mover a janela para o ecrã { $args }
    }
wm-action-moveworkspacetooutput = { $first ->
        [left] Mover a área de trabalho para o ecrã à esquerda
        [right] Mover a área de trabalho para o ecrã à direita
        [up] Mover a área de trabalho para o ecrã acima
        [down] Mover a área de trabalho para o ecrã abaixo
       *[other] Mover a área de trabalho para o ecrã { $args }
    }
wm-action-swapworkspaces = Trocar as áreas de trabalho dos ecrãs { $args }
wm-action-scratchpad = { $first ->
        [hide] Ocultar o scratchpad
       *[other] Alternar o scratchpad { $rest }
    }
wm-action-movetoscratchpad = Mover a janela para o scratchpad { $args }
wm-action-kill = Fechar a janela
wm-action-focuswindow = Focar a janela { $args }
wm-action-minimize = { $first ->
        [default] Minimizar a janela
       *[other] Minimizar a janela { $args }
    }
wm-action-restore = { $first ->
        [default] Restaurar a última janela minimizada
       *[other] Restaurar a janela { $args }
    }
wm-action-overview = { $first ->
        [show] Mostrar a vista geral
        [hide] Ocultar a vista geral
       *[other] Alternar a vista geral
    }
wm-action-closewindow = Fechar a janela { $args }
wm-action-dpms = { $first ->
        [on] Ligar os ecrãs
        [off] Desligar os ecrãs
       *[other] Ligar ou desligar os ecrãs
    }
wm-action-reload = Recarregar a configuração
wm-action-dismissconfignotice = Dispensar o aviso de configuração
wm-action-exit = Terminar a sessão
wm-action-nextlayout = Mudar para a disposição de teclado seguinte
wm-action-stickykeys = { $first ->
        [on] Ligar as teclas de aderência
        [off] Desligar as teclas de aderência
       *[other] Teclas de aderência { $args }
    }
wm-action-slowkeys = Definir as teclas lentas para { $args } ms
wm-action-global = Atalho da aplicação { $args }
wm-action-exec = Executar { $args }
wm-action-launch = Iniciar { $app }
wm-action-makegroup = { $first ->
        [h] Agrupar a janela lado a lado
        [v] Agrupar a janela empilhada
        [tab] Agrupar a janela em abas
       *[other] Agrupar a janela { $args }
    }
wm-action-setlayout = { $first ->
        [tree] Usar a disposição em mosaico
        [scroll] Usar a disposição com deslocamento
       *[other] Definir a disposição { $args }
    }
wm-action-columnwidth = Definir a largura da coluna { $args }
wm-action-consume = { $first ->
        [left] Mover a janela para a coluna à esquerda
        [right] Mover a janela para a coluna à direita
       *[other] Mover a janela para a coluna { $args }
    }
wm-action-expel = Mover a janela para uma coluna própria
wm-action-center = Centrar a coluna focada
wm-action-columndisplay = { $first ->
        [stack] Empilhar as janelas da coluna
        [tab] Mostrar a coluna em abas
       *[other] Mostrar a coluna como { $args }
    }
wm-action-changegroup = { $first ->
        [h] Dispor o grupo lado a lado
        [v] Dispor o grupo empilhado
        [tab] Dispor o grupo em abas
        [toggletab] Alternar as abas do grupo
        [opposite] Virar o grupo para o outro lado
       *[other] Alterar o grupo { $args }
    }
wm-action-focustab = { $first ->
        [prev] Focar a aba anterior
        [next] Focar a aba seguinte
       *[other] Focar a aba { $args }
    }
wm-action-focusparent = Focar o grupo que envolve
wm-action-focuschild = Focar de volta dentro do grupo
wm-action-swap = { $first ->
        [left] Trocar a janela com a da esquerda
        [right] Trocar a janela com a da direita
        [up] Trocar a janela com a de cima
        [down] Trocar a janela com a de baixo
       *[other] Trocar a janela { $args }
    }
wm-action-moveout = Mover a janela para fora do seu grupo
wm-action-equalize = { $first ->
        [all] Igualar todas as divisões
       *[other] Igualar o grupo
    }
wm-action-locktab = { $first ->
        [on] Bloquear o grupo de abas
        [off] Desbloquear o grupo de abas
       *[other] Bloquear ou desbloquear o grupo de abas
    }
wm-action-submap = { $first ->
        [reset] Sair do modo
       *[other] Entrar no modo { $args }
    }
wm-action-togglefloating = Tornar a janela flutuante ou em mosaico
wm-action-place = { $first ->
        [left] Colocar a janela na metade esquerda
        [right] Colocar a janela na metade direita
        [top] Colocar a janela na metade superior
        [bottom] Colocar a janela na metade inferior
        [topleft] Colocar a janela no quarto superior esquerdo
        [topright] Colocar a janela no quarto superior direito
        [bottomleft] Colocar a janela no quarto inferior esquerdo
        [bottomright] Colocar a janela no quarto inferior direito
        [fill] Fazer a janela preencher o ecrã
        [center] Centrar a janela
        [restore] Pôr a janela onde estava
       *[other] Colocar a janela { $args }
    }
wm-action-focusfloating = { $first ->
        [on] Focar as janelas flutuantes
        [off] Focar as janelas em mosaico
       *[other] Alternar o foco entre janelas flutuantes e em mosaico
    }
wm-action-fullscreen = { $first ->
        [on] Pôr a janela em ecrã inteiro
        [off] Sair do ecrã inteiro
       *[other] Alternar o ecrã inteiro
    }
wm-action-setprop = Definir a propriedade da janela { $args }
wm-action-showdesktop = { $first ->
        [on] Mostrar o ambiente de trabalho
        [off] Trazer as janelas de volta
       *[other] Mostrar ou ocultar o ambiente de trabalho
    }
wm-action-swallow = { $first ->
        [on] Ocultar o terminal na janela
        [off] Mostrar o terminal oculto
       *[other] Mostrar ou ocultar o terminal absorvido
    }
wm-action-pin = { $first ->
        [on] Afixar a janela
        [off] Desafixar a janela
       *[other] Afixar ou desafixar a janela
    }
wm-action-maximize = { $first ->
        [on] Maximizar a janela
        [off] Desmaximizar a janela
       *[other] Alternar a maximização
    }
wm-action-resizeactive = Redimensionar a janela em { $args }
wm-action-movewindow = Arrastar a janela
wm-action-resizewindow = Redimensionar a janela por arrasto
wm-action-zoom = { $first ->
        [in] Aumentar o zoom
        [out] Diminuir o zoom
        [reset] Repor o zoom
       *[other] Definir o zoom para { $rest }
    }
wm-action-debug = Depurar { $args }
wm-action-casttarget = { $first ->
        [window] Partilhar uma janela
        [output] Partilhar um ecrã
        [clear] Parar a partilha
       *[other] Partilhar { $args }
    }
wm-action-focuslast = Focar a janela anterior
wm-action-focusurgent = Focar a janela que pede atenção
wm-action-cycle = { $first ->
        [next] Percorrer até à janela seguinte
        [prev] Percorrer até à janela anterior
       *[other] Percorrer as janelas { $args }
    }
wm-action-switcher = { $first ->
        [next] Mudar para a janela seguinte
        [prev] Mudar para a janela anterior
       *[other] Mudar de janela { $args }
    }
wm-binds-unknown-action = Ação desconhecida
    .description = O lys-wm não tem nenhuma ação com este nome; o atalho é mostrado como foi escrito
wm-binds-category-windows = Janelas
wm-binds-category-workspaces = Áreas de trabalho
wm-binds-category-layout = Disposição
wm-binds-category-apps = Aplicações
wm-binds-category-screenshots = Capturas de ecrã
wm-binds-category-media = Multimédia
wm-binds-category-system = Sistema
wm-binds-category-custom = Personalizados
wm-binds-search = Pesquisar atalhos, ações ou teclas
## The arguments of a lys-wm action, by the form lys-wm describes
wm-action-pick = Escolha uma ação
wm-action-args-outside = Não é uma forma desta ação
    .description = O lys-wm descreve os argumentos desta ação de outra maneira; edite-os como texto
enum-wm-arg-default = Predefinido
enum-wm-arg-all = Todas
enum-wm-arg-app = Aplicação focada
enum-wm-arg-app_id = ID da aplicação
enum-wm-arg-app_id-colon-id = Aplicação e atalho
enum-wm-arg-bottom = Metade inferior
enum-wm-arg-bottomleft = Inferior esquerdo
enum-wm-arg-bottomright = Inferior direito
enum-wm-arg-center = Centro
enum-wm-arg-clear = Nada
enum-wm-arg-command = Comando
enum-wm-arg-down = Para baixo
enum-wm-arg-dx = Alteração da largura (px)
enum-wm-arg-dy = Alteração da altura (px)
enum-wm-arg-e-minus-n = Abrir área de trabalho anterior, qualquer ecrã
enum-wm-arg-e-plus-n = Abrir área de trabalho seguinte, qualquer ecrã
enum-wm-arg-empty = Primeira vazia
enum-wm-arg-ephemeral = Dissolver quando restar uma janela
enum-wm-arg-factor = Fator
enum-wm-arg-fake = Só dentro do seu mosaico
enum-wm-arg-fill = Preencher
enum-wm-arg-follow = Seguir a janela
enum-wm-arg-h = Lado a lado
enum-wm-arg-hide = Ocultar
enum-wm-arg-id = ID da janela
enum-wm-arg-in = Entrar
enum-wm-arg-key = Propriedade
enum-wm-arg-left = Esquerda
enum-wm-arg-m-minus-n = Abrir área de trabalho anterior, este ecrã
enum-wm-arg-m-plus-n = Abrir área de trabalho seguinte, este ecrã
enum-wm-arg-minus-n-percent = Mais estreita em percentagem
enum-wm-arg-minus-n-px = Mais estreita em píxeis
enum-wm-arg-ms = Milissegundos
enum-wm-arg-n = Número
enum-wm-arg-n-percent = Percentagem do ecrã
enum-wm-arg-n-px = Píxeis
enum-wm-arg-name = Nome
enum-wm-arg-next = Seguinte
enum-wm-arg-off = Desligado
enum-wm-arg-on = Ligado
enum-wm-arg-opposite = Ao contrário
enum-wm-arg-out = Sair
enum-wm-arg-output = Ecrã
enum-wm-arg-output-name = Ecrã
enum-wm-arg-plus-n-percent = Mais larga em percentagem
enum-wm-arg-plus-n-px = Mais larga em píxeis
enum-wm-arg-preset-next = Predefinição seguinte
enum-wm-arg-preset-prev = Predefinição anterior
enum-wm-arg-prev = Anterior
enum-wm-arg-previous = Anterior
enum-wm-arg-reset = Repor
enum-wm-arg-restore = Onde estava
enum-wm-arg-right = Direita
enum-wm-arg-scroll = Com deslocamento
enum-wm-arg-set = Definir
enum-wm-arg-show = Mostrar
enum-wm-arg-stack = Empilhadas
enum-wm-arg-tab = Abas
enum-wm-arg-toggle = Alternar
enum-wm-arg-toggle-name = Alternar
enum-wm-arg-toggletab = Alternar abas
enum-wm-arg-top = Metade superior
enum-wm-arg-topleft = Superior esquerdo
enum-wm-arg-topright = Superior direito
enum-wm-arg-tree = Mosaico
enum-wm-arg-up = Para cima
enum-wm-arg-v = Empilhadas
enum-wm-arg-value = Valor
enum-wm-arg-window = Janela
enum-wm-arg-window-id = Janela
enum-wm-arg-workspace = Área de trabalho
enum-wm-arg-wrap = Voltar ao início
wm-binds-clash-mode = { $label } no modo { $mode }
wm-binds-reset = Voltar ao atalho do lys-wm
wm-mode = Modo { $mode }
wm-mode-enter = Atalho que entra no modo { $mode }
wm-mode-rename = Mudar o nome do modo
wm-mode-rename-confirm = Mudar o nome
wm-mode-delete-button = Eliminar o modo
wm-mode-delete-title = Eliminar o modo { $mode }?
wm-mode-delete-description = Os seus atalhos, e os atalhos que entram nele, também são removidos.
wm-mode-delete-cancel = Cancelar
wm-mode-delete-confirm = Eliminar
wm-mode-add = Adicionar um modo
wm-mode-add-confirm = Adicionar
wm-mode-cancel = Cancelar
wm-mode-name = Nome
wm-mode-name-placeholder = Nome, como resize
wm-mode-chord = Atalho que entra no modo
wm-mode-name-missing = Um modo precisa de um nome
wm-mode-name-reset = { $name } é a forma como um atalho sai de um modo, por isso não pode dar nome a um
wm-mode-name-taken = Já existe um modo chamado { $name }
wm-mode-chord-missing = Grave o atalho que entra no modo
wm-mode-unreachable = Nenhum atalho entra neste modo; grave um ao lado do seu nome
wm-section-refraction = Refração
    .description = Vidro desfocado que curva o que está por trás junto às suas margens
wm-section-specular = Brilho da margem
    .description = Um reflexo ao longo da margem do vidro desfocado, do lado virado para o ponteiro
wm-decoration-blur-refraction = Refração
    .description = Curvar o fundo junto às margens das janelas desfocadas
wm-decoration-blur-refraction-bezel = Margem de curvatura
    .description = Até onde, a partir da margem para dentro, o fundo se curva, em píxeis
wm-decoration-blur-refraction-amount = Intensidade da refração
    .description = Até onde o vidro olha para lá da margem, em píxeis
wm-decoration-blur-specular = Brilho da margem
    .description = Iluminar a margem das janelas desfocadas
wm-decoration-blur-specular-strength = Intensidade do brilho
    .description = Com que intensidade brilha a margem
wm-decoration-blur-specular-width = Largura do brilho
    .description = Em píxeis
wm-scroll-presets = Larguras de coluna
    .description = As larguras que o atalho da largura da coluna percorre na disposição com deslocamento, por esta ordem
    .keywords = predefinições, columnwidth, deslocamento
wm-scroll-presets-share = Parte do ecrã
wm-scroll-presets-pixels = Píxeis
wm-scroll-presets-add = Adicionar largura
wm-scroll-presets-add-width = Adicionar uma coluna com { $width } de largura
wm-scroll-presets-remove = Remover esta largura
wm-section-touch-gestures = Gestos do ecrã tátil
    .description = Deslizes num ecrã tátil e o que fazem
wm-gestures-touch = Deslizes táteis
    .description = Deslizar a partir de uma margem, ou com três ou mais dedos em qualquer parte do ecrã
wm-gestures-touch-add = Adicionar deslize tátil
wm-gestures-touch-start = Começa
wm-gestures-touch-does = Faz
wm-gestures-touch-fingers-anywhere = Dedos em qualquer parte
wm-gestures-touch-fingers = Dedos
wm-gestures-touch-direction = Direção
wm-gestures-touch-run = Executar uma ação
wm-gestures-touch-follow = Seguir os dedos
wm-gestures-touch-edge-title = { $edge ->
        [top] Deslizar a partir da margem superior
        [bottom] Deslizar a partir da margem inferior
        [left] Deslizar a partir da margem esquerda
        [right] Deslizar a partir da margem direita
       *[other] Deslizar a partir de uma margem
    }
wm-gestures-touch-fingers-title = { $direction ->
        [left] Deslizar com { $fingers } dedos para a esquerda
        [right] Deslizar com { $fingers } dedos para a direita
        [up] Deslizar com { $fingers } dedos para cima
        [down] Deslizar com { $fingers } dedos para baixo
        [horizontal] Deslizar com { $fingers } dedos para o lado
        [vertical] Deslizar com { $fingers } dedos para cima ou para baixo
        [any] Deslizar com { $fingers } dedos em qualquer sentido
       *[other] Deslizar com { $fingers } dedos
    }
enum-wm-edge-top = Margem superior
enum-wm-edge-bottom = Margem inferior
enum-wm-edge-left = Margem esquerda
enum-wm-edge-right = Margem direita
enum-wm-direction-left = Esquerda
enum-wm-direction-right = Direita
enum-wm-direction-up = Cima
enum-wm-direction-down = Baixo
enum-wm-direction-horizontal = Para o lado
enum-wm-direction-vertical = Cima ou baixo
enum-wm-direction-any = Qualquer sentido
enum-wm-drive-workspaces = Áreas de trabalho
enum-wm-drive-overview = Vista geral
enum-wm-drive-view = Deslocar as colunas
enum-wm-drive-move = Mover a janela
enum-wm-drive-resize = Redimensionar a janela
wm-input-pointer-scroll-button = Botão de deslocamento
    .description = O botão mantido premido para deslocar movendo o dispositivo
wm-input-touchpad-scroll-button = Botão de deslocamento
    .description = O botão mantido premido para deslocar movendo o dispositivo
wm-input-trackpoint-scroll-button = Botão de deslocamento
    .description = O botão mantido premido para deslocar movendo o dispositivo
wm-button-code = Código do botão
    .description = Outro botão, pelo seu código evdev (BTN_SIDE é 275)
enum-wm-scroll-button-back = Retroceder
enum-wm-scroll-button-extra = Extra
enum-wm-scroll-button-forward = Avançar
enum-wm-scroll-button-left = Esquerdo
enum-wm-scroll-button-middle = Central
enum-wm-scroll-button-right = Direito
enum-wm-scroll-button-side = Lateral
enum-wm-scroll-button-task = Tarefa
wm-preview-tabs = Pré-visualização da barra de separadores
wm-preview-tab-active = Em foco
wm-preview-tab-inactive = Outra janela
wm-preview-tab-urgent = Pede atenção
wm-tabs-font-built-in = O tipo de letra próprio do lys-wm
wm-tabs-font-unresolved = Não foi encontrado nenhum ficheiro de tipo de letra de { $family }
wm-tabs-font-unreadable = O lys-wm não consegue ler o ficheiro de tipo de letra de { $family }
wm-section-workspaces = Áreas de trabalho
    .description = Regras para áreas de trabalho concretas, ou para todas as que correspondam
wm-section-scratchpads = Janelas de rascunho
    .description = Janelas com nome mostradas e ocultadas por cima das outras com um atalho
wm-workspaces = Regras das áreas de trabalho
    .description = Uma área pelo número, ou todas as que correspondam, e como se apresentam e arrancam
wm-workspace-number = Área de trabalho { $number }
wm-workspaces-id = Número
wm-workspaces-match = Todas as áreas
wm-workspaces-name = Nome
wm-workspaces-output = Ecrã
wm-workspaces-default = Mostrada primeiro no seu ecrã
wm-workspaces-persistent = Mantida mesmo vazia
wm-workspaces-on-created-empty = Aplicação a abrir quando é criada vazia
wm-workspaces-layout = Disposição
wm-workspaces-gaps-inner = Espaço entre janelas
wm-workspaces-gaps-outer = Espaço nas margens do ecrã
wm-workspaces-borders-width = Largura do contorno
wm-workspaces-rounding = Arredondamento dos cantos
wm-workspaces-shadow = Sombra
wm-scratchpads = Janelas de rascunho
    .description = A aplicação e o tamanho de cada janela de rascunho; um atalho mostra-a ou oculta-a pelo nome
wm-scratchpads-name = Nome da janela de rascunho
wm-scratchpads-exec = Aplicação
wm-scratchpads-size = Tamanho
wm-scratchpads-hide-on-workspace-switch = Ocultada ao mudar de área
wm-size-width = Largura %
wm-size-height = Altura %
enum-wm-match-any = Qualquer área
enum-wm-match-single_tiled = Com uma janela em mosaico
enum-wm-match-fullscreen = Com uma janela em ecrã inteiro
