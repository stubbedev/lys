settings-launcher-location = Posição
    .description = Onde o lançador aparece no ecrã

settings-launcher-width = Largura
    .description = Largura da superfície — um multiplicador da predefinição (1.0) ou píxeis (ex.: "800px")

settings-launcher-lines = Linhas
    .description = Linhas de resultados visíveis

settings-launcher-monitor = Monitor
    .description = Conector de saída onde mostrar (vazio = saída com foco)

settings-launcher-modes = Modos
    .description = Modos ativados, pela ordem das abas (drun, run, window, ssh, filebrowser, combi, keys, ou o nome de um script)

settings-launcher-cycle = Seleção cíclica
    .description = Dar a volta à seleção nos extremos da lista

settings-launcher-matching = Método de correspondência
    .description = Como o texto escrito é comparado com as entradas

settings-launcher-tokenize = Separar em palavras
    .description = Corresponder cada palavra da pesquisa de forma independente

settings-launcher-negate-char = Carácter de negação
    .description = Uma palavra escrita depois deste carácter esconde as correspondências que a contêm

settings-launcher-normalize-match = Normalizar correspondência
    .description = Ignorar acentos e variantes Unicode na correspondência

settings-launcher-sort = Ordenar resultados
    .description = Classificar os resultados pela qualidade da correspondência em vez da ordem da lista

settings-launcher-sorting-method = Método de ordenação
    .description = Algoritmo de classificação usado quando a ordenação está ativada

settings-launcher-case = Maiúsculas e minúsculas
    .description = Como as maiúsculas e minúsculas afetam a correspondência

settings-launcher-terminal = Terminal
    .description = Emulador de terminal para aplicações de terminal (vazio = deteção automática)

settings-launcher-show-icons = Mostrar ícones
    .description = Mostrar ícones ao lado das entradas

settings-launcher-icon-theme = Tema de ícones
    .description = Substituição do tema de ícones (vazio = tema do sistema)

settings-launcher-sidebar-mode = Modo de barra lateral
    .description = Mostrar as abas dos modos no fundo do lançador

settings-launcher-auto-select = Seleção automática
    .description = Aceitar automaticamente quando resta exatamente um resultado

settings-launcher-hover-select = Selecionar ao passar o rato
    .description = Selecionar a linha sob o cursor do rato

settings-launcher-fixed-num-lines = Altura fixa
    .description = Manter a altura da lista fixa no número de linhas configurado

settings-launcher-display-names = Nomes apresentados dos modos
    .description = Substituições do nome apresentado por modo (ex.: drun = "apps")

settings-launcher-scripts = Modos de script
    .description = Modos de script personalizados: nome → executável (protocolo de scripts do rofi)

settings-launcher-keybindings = Atalhos de teclado
    .description = Substituições de atalhos: ação → teclas separadas por vírgulas (as ações não definidas mantêm as predefinições do rofi)

settings-launcher-mouse-bindings = Atalhos do rato
    .description = Que botões do rato e direções de deslocamento executam cada ação do lançador, em vez das predefinidas

settings-launcher-font = Tipo de letra
    .description = Tipo de letra do lançador como descrição Pango (ex.: "Inter 12"); vazio mantém o tipo de letra da shell

settings-launcher-styles = Predefinições de estilo
    .description = Aspetos com nome que um lançamento pode pedir pelo nome: cada nome → o CSS aplicado enquanto esse lançamento é mostrado

settings-launcher-preview-cmd = Comando de pré-visualização
    .description = Comando que cria a miniatura de uma linha: {"{"}input{"}"} o ficheiro, {"{"}output{"}"} a imagem a escrever, {"{"}size{"}"} o tamanho em píxeis; vazio usa os geradores de miniaturas do sistema

settings-launcher-history = Histórico
    .description = Histórico de lançamentos e classificação por frequência e recência

settings-launcher-drun = Aplicações
    .description = Modo drun (aplicações)

settings-launcher-run = Comandos
    .description = Modo run (comandos)

settings-launcher-window = Janelas
    .description = Modo de alternância de janelas

settings-launcher-ssh = SSH
    .description = Modo de anfitriões SSH

settings-launcher-filebrowser = Navegador de ficheiros
    .description = Modo de navegador de ficheiros

settings-launcher-combi = Combi
    .description = Modo de modos combinados

settings-launcher-history-enable = Ativar histórico
    .description = Registar lançamentos e classificar primeiro as entradas usadas com frequência

settings-launcher-history-max-size = Tamanho do histórico
    .description = Máximo de entradas lembradas por modo

settings-launcher-drun-categories = Categorias
    .description = Mostrar apenas aplicações nestas categorias (vazio = todas)

settings-launcher-drun-exclude-categories = Categorias excluídas
    .description = Ocultar aplicações nestas categorias

settings-launcher-drun-match-fields = Campos de correspondência
    .description = Campos da entrada de ambiente de trabalho pesquisados ao escrever

settings-launcher-drun-display-format = Formato de apresentação
    .description = Modelo da linha: {"{name}"}, {"{generic}"}, {"{exec}"}, {"{categories}"}, {"{comment}"}; [..] só é apresentado quando preenchido

settings-launcher-drun-show-actions = Mostrar ações
    .description = Expor as ações do ficheiro .desktop como linhas extra

settings-launcher-drun-url-launcher = Lançador de URL
    .description = Comando que abre entradas Type=Link (vazio = o abridor integrado)

settings-launcher-run-run-command = Comando de execução
    .description = Modelo de aceitação simples ({"{cmd}"})

settings-launcher-run-shell-command = Comando de terminal
    .description = Modelo de execução no terminal ({"{terminal}"}, {"{cmd}"})

settings-launcher-run-list-command = Comando de listagem
    .description = Comando extra cujas linhas de stdout adicionam entradas

settings-launcher-window-format = Formato de apresentação
    .description = Modelo da linha: {"{w}"} área de trabalho, {"{c}"} classe, {"{t}"} título, {"{n}"} nome, {"{r}"} papel

settings-launcher-window-match-fields = Campos de correspondência
    .description = Campos da janela pesquisados ao escrever

settings-launcher-window-hide-active = Ocultar janela ativa
    .description = Ocultar da lista a janela com foco

settings-launcher-window-close-on-delete = Fechar ao eliminar
    .description = Shift+Delete fecha a janela selecionada

settings-launcher-ssh-client = Cliente SSH
    .description = Binário do cliente SSH

settings-launcher-ssh-command = Comando de ligação
    .description = Modelo de ligação ({"{terminal}"}, {"{ssh-client}"}, {"{host}"})

settings-launcher-ssh-parse-hosts = Analisar /etc/hosts
    .description = Incluir anfitriões de /etc/hosts

settings-launcher-ssh-parse-known-hosts = Analisar known_hosts
    .description = Incluir anfitriões de ~/.ssh/known_hosts

settings-launcher-filebrowser-directory = Pasta inicial
    .description = Pasta em que o navegador abre (vazio = pasta pessoal)

settings-launcher-filebrowser-sorting-method = Ordenação
    .description = Ordem dos ficheiros

settings-launcher-filebrowser-directories-first = Pastas primeiro
    .description = Listar as pastas antes dos ficheiros

settings-launcher-filebrowser-show-hidden = Mostrar ocultos
    .description = Mostrar ficheiros ocultos

settings-launcher-filebrowser-command = Comando de abertura
    .description = Comando que abre o ficheiro escolhido (vazio = o abridor integrado)

settings-launcher-combi-modes = Modos combinados
    .description = Modos reunidos na lista combinada

settings-launcher-combi-display-format = Formato de apresentação
    .description = Modelo da linha ({"{mode}"}, {"{text}"})

enum-launcher-location-center = Centro
enum-launcher-location-north-west = Superior esquerdo
enum-launcher-location-north = Topo
enum-launcher-location-north-east = Superior direito
enum-launcher-location-east = Direita
enum-launcher-location-south-east = Inferior direito
enum-launcher-location-south = Fundo
enum-launcher-location-south-west = Inferior esquerdo
enum-launcher-location-west = Esquerda

enum-launcher-matching-normal = Normal
enum-launcher-matching-regex = Expressão regular
enum-launcher-matching-glob = Glob
enum-launcher-matching-fuzzy = Aproximada
enum-launcher-matching-prefix = Prefixo

enum-launcher-sorting-levenshtein = Levenshtein
enum-launcher-sorting-fzf = FZF

enum-launcher-case-insensitive = Indiferente
enum-launcher-case-smart = Inteligente
enum-launcher-case-sensitive = Sensível

enum-launcher-drun-field-name = Nome
enum-launcher-drun-field-generic = Nome genérico
enum-launcher-drun-field-exec = Comando
enum-launcher-drun-field-categories = Categorias
enum-launcher-drun-field-comment = Comentário
enum-launcher-drun-field-keywords = Palavras-chave

enum-launcher-window-field-title = Título
enum-launcher-window-field-class = Classe
enum-launcher-window-field-name = Nome
enum-launcher-window-field-role = Papel
enum-launcher-window-field-desktop = Área de trabalho

enum-launcher-file-sort-name = Nome
enum-launcher-file-sort-mtime = Modificado
enum-launcher-file-sort-atime = Acedido
enum-launcher-file-sort-ctime = Criado

settings-animations-launcher = Lançador
    .description = Substituição da animação de entrada/saída do lançador de aplicações

settings-launcher-search = Painel de pesquisa
    .description = O painel que lys-shell launcher abre sem argumentos

settings-launcher-search-providers = Fontes
    .description = O que a lista pesquisa

settings-launcher-search-web-engine = Nome da pesquisa web
    .description = O nome do motor na linha de recurso

settings-launcher-search-web-url = URL da pesquisa web
    .description = O URL, com {"{"}query{"}"} onde vai a pesquisa

settings-launcher-search-file-roots = Pastas do índice de ficheiros
    .description = Pastas que o índice de ficheiros percorre; vazio indexa a pasta pessoal

settings-launcher-search-file-exclude = Exclusões do índice de ficheiros
    .description = Nomes de pastas que o índice de ficheiros ignora

settings-launcher-search-commands-dir = Pasta de comandos de script
    .description = Vazio usa launcher/commands na pasta de configuração

settings-launcher-search-currency-rates = Taxas de câmbio
    .description = Obter as taxas de referência diárias do Banco Central Europeu, no máximo uma vez por dia, para respostas de câmbio

settings-launcher-search-previews = Pré-visualizações
    .description = Mostrar o painel lateral para ficheiros, excertos, janelas e entradas da área de transferência, no painel de pesquisa e nos modos de janelas e área de transferência

settings-launcher-search-quicklinks = Ligações rápidas
    .description = Ligações a abrir pelo nome ou palavra-chave

settings-launcher-search-snippets = Excertos
    .description = Texto a copiar pelo nome ou palavra-chave

enum-launcher-search-provider-apps = Aplicações
enum-launcher-search-provider-windows = Janelas abertas
enum-launcher-search-provider-files = Ficheiros
enum-launcher-search-provider-calculator = Calculadora
enum-launcher-search-provider-system = Comandos do sistema
enum-launcher-search-provider-settings = Páginas de definições
enum-launcher-search-provider-clipboard = Histórico da área de transferência
enum-launcher-search-provider-emoji = Emoji
enum-launcher-search-provider-characters = Caracteres Unicode
enum-launcher-search-provider-vault = Cofre do Bitwarden
enum-launcher-search-provider-links = Ligações rápidas e excertos
enum-launcher-search-provider-browser-bookmarks = Marcadores do navegador
enum-launcher-search-provider-browser-history = Histórico do navegador
enum-launcher-search-provider-quick-toggles = Alternâncias rápidas
enum-launcher-search-provider-media = Controlos multimédia
enum-launcher-search-provider-capture = Capturas de ecrã e gravação
enum-launcher-search-provider-processes = Terminar processo
enum-launcher-search-provider-scripts = Comandos de script
enum-launcher-search-provider-shell = Comandos da shell
enum-launcher-search-provider-web = Pesquisa web

settings-launcher-drun-display-format-placeholder-name = Nome da aplicação

settings-launcher-drun-display-format-placeholder-generic = Nome genérico (ex.: Navegador web)

settings-launcher-drun-display-format-placeholder-exec = Comando que a entrada executa

settings-launcher-drun-display-format-placeholder-categories = Categorias da entrada de ambiente de trabalho

settings-launcher-drun-display-format-placeholder-comment = Comentário da entrada de ambiente de trabalho

settings-launcher-window-format-placeholder-w = Área de trabalho

settings-launcher-window-format-placeholder-c = Classe da janela

settings-launcher-window-format-placeholder-t = Título da janela

settings-launcher-window-format-placeholder-n = Nome da janela (a sua classe)

settings-launcher-window-format-placeholder-r = Papel da janela (sempre vazio)

settings-launcher-combi-display-format-placeholder-mode = Modo de onde a linha vem

settings-launcher-combi-display-format-placeholder-text = O texto da própria linha

settings-launcher-emoji = Emoji
    .description = Emoji no painel de pesquisa, na grelha de emoji e no modo de emoji

settings-launcher-emoji-skin-tone = Tom de pele
    .description = O tom de pele dos emoji de pessoas e mãos; escolher um nas ações de um emoji torna-o a predefinição

enum-launcher-skin-tone-none = Predefinido (amarelo)
enum-launcher-skin-tone-light = Claro
enum-launcher-skin-tone-medium-light = Médio-claro
enum-launcher-skin-tone-medium = Médio
enum-launcher-skin-tone-medium-dark = Médio-escuro
enum-launcher-skin-tone-dark = Escuro

settings-launcher-run-run-command-placeholder-cmd = O comando escrito
settings-launcher-run-shell-command-placeholder-terminal = O terminal
settings-launcher-run-shell-command-placeholder-cmd = O comando escrito
settings-launcher-ssh-command-placeholder-terminal = O terminal
settings-launcher-ssh-command-placeholder-ssh-client = O cliente SSH
settings-launcher-ssh-command-placeholder-host = O anfitrião
settings-launcher-preview-cmd-placeholder-input = O ficheiro a pré-visualizar
settings-launcher-preview-cmd-placeholder-output = Onde escrever a miniatura
settings-launcher-preview-cmd-placeholder-size = O tamanho da miniatura em píxeis

## Quick links and snippets

settings-launcher-quicklink-name = Nome
settings-launcher-quicklink-url = Endereço
settings-launcher-quicklink-keyword = Palavra-chave
settings-launcher-quicklink-icon = Ícone
settings-launcher-quicklink-url-placeholder-query = As palavras escritas depois da palavra-chave
settings-launcher-snippet-name = Nome
settings-launcher-snippet-text = Texto
settings-launcher-snippet-keyword = Palavra-chave
