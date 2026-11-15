settings-launcher-location = Posición
    .description = Dónde aparece el lanzador en la pantalla

settings-launcher-width = Anchura
    .description = Anchura de la superficie: un multiplicador del valor predeterminado (1.0) o píxeles (p. ej. "800px")

settings-launcher-lines = Líneas
    .description = Líneas de resultados visibles

settings-launcher-monitor = Monitor
    .description = Conector de salida en el que mostrarse (vacío = salida con el foco)

settings-launcher-modes = Modos
    .description = Modos activados, en orden de pestañas (drun, run, window, ssh, filebrowser, combi, keys, o un nombre de script)

settings-launcher-cycle = Selección cíclica
    .description = Pasa la selección al otro extremo al llegar al borde de la lista

settings-launcher-matching = Método de coincidencia
    .description = Cómo se compara el texto escrito con las entradas

settings-launcher-tokenize = Separar en palabras
    .description = Busca coincidencias de cada palabra de la consulta por separado

settings-launcher-negate-char = Carácter de negación
    .description = Prefijo de palabra que excluye coincidencias (rofi -matching-negate-char)

settings-launcher-normalize-match = Normalizar coincidencias
    .description = Ignora los acentos y las variantes Unicode al buscar coincidencias

settings-launcher-sort = Ordenar resultados
    .description = Clasifica los resultados por calidad de coincidencia en lugar del orden de la lista

settings-launcher-sorting-method = Método de ordenación
    .description = Algoritmo de clasificación usado cuando la ordenación está activada

settings-launcher-case = Distinción de mayúsculas
    .description = Cómo afectan las mayúsculas y minúsculas a las coincidencias

settings-launcher-terminal = Terminal
    .description = Emulador de terminal para las aplicaciones de terminal (vacío = detección automática)

settings-launcher-show-icons = Mostrar iconos
    .description = Muestra iconos junto a las entradas

settings-launcher-icon-theme = Tema de iconos
    .description = Tema de iconos personalizado (vacío = tema del sistema)

settings-launcher-sidebar-mode = Modo de barra lateral
    .description = Muestra las pestañas de modos en la parte inferior del lanzador

settings-launcher-auto-select = Selección automática
    .description = Acepta automáticamente cuando queda exactamente un resultado

settings-launcher-hover-select = Seleccionar al pasar el puntero
    .description = Selecciona la fila bajo el cursor del ratón

settings-launcher-fixed-num-lines = Altura fija
    .description = Mantiene la altura de la lista fija en el número de líneas configurado

settings-launcher-display-names = Nombres visibles de los modos
    .description = Nombres visibles personalizados por modo (p. ej. drun = "apps")

settings-launcher-scripts = Modos de script
    .description = Modos de script personalizados: nombre → ejecutable (protocolo de scripts de rofi)

settings-launcher-keybindings = Atajos de teclado
    .description = Atajos personalizados: acción → teclas separadas por comas (las acciones sin definir mantienen los valores predeterminados de rofi)

settings-launcher-mouse-bindings = Asignaciones del ratón
    .description = Asignaciones del ratón personalizadas: acción (botones me-*, desplazamiento ml-*) → botones separados por comas (p. ej. "MouseDPrimary", "ScrollDown")

settings-launcher-font = Tipografía
    .description = Tipografía del lanzador como descripción de Pango (p. ej. "Inter 12"); vacío mantiene la tipografía del shell

settings-launcher-styles = Estilos predefinidos
    .description = Aspectos con nombre seleccionables en cada invocación con -style <name>: nombre → el CSS aplicado en esa sesión

settings-launcher-preview-cmd = Orden de vista previa
    .description = Orden que genera la miniatura de una fila: {"{"}input{"}"} el archivo, {"{"}output{"}"} la imagen que escribir, {"{"}size{"}"} el tamaño en píxeles; vacío usa los generadores de miniaturas del sistema

settings-launcher-history = Historial
    .description = Historial de ejecuciones y clasificación por frecuencia y antigüedad

settings-launcher-drun = Aplicaciones
    .description = Modo drun (aplicaciones)

settings-launcher-run = Órdenes
    .description = Modo run (órdenes)

settings-launcher-window = Ventanas
    .description = Modo de cambio de ventanas

settings-launcher-ssh = SSH
    .description = Modo de equipos SSH

settings-launcher-filebrowser = Explorador de archivos
    .description = Modo de explorador de archivos

settings-launcher-combi = Combinado
    .description = Modo de modos combinados

settings-launcher-history-enable = Activar el historial
    .description = Registra las ejecuciones y muestra primero las entradas usadas con frecuencia

settings-launcher-history-max-size = Tamaño del historial
    .description = Máximo de entradas recordadas por modo

settings-launcher-drun-categories = Categorías
    .description = Muestra solo las aplicaciones de estas categorías (vacío = todas)

settings-launcher-drun-exclude-categories = Categorías excluidas
    .description = Oculta las aplicaciones de estas categorías

settings-launcher-drun-match-fields = Campos de coincidencia
    .description = Campos de la entrada de escritorio en los que se busca al escribir

settings-launcher-drun-display-format = Formato de visualización
    .description = Plantilla de fila: {"{name}"}, {"{generic}"}, {"{exec}"}, {"{categories}"}, {"{comment}"}; [..] solo se muestra cuando tiene contenido

settings-launcher-drun-show-actions = Mostrar acciones
    .description = Muestra las acciones del archivo de escritorio como filas adicionales

settings-launcher-drun-url-launcher = Lanzador de URL
    .description = Orden que abre las entradas de escritorio Type=Link (vacío = el abridor integrado)

settings-launcher-run-run-command = Orden de ejecución
    .description = Plantilla de aceptación simple ({"{cmd}"})

settings-launcher-run-shell-command = Orden de terminal
    .description = Plantilla de ejecución en terminal ({"{terminal}"}, {"{cmd}"})

settings-launcher-run-list-command = Orden de lista
    .description = Orden adicional cuyas líneas de stdout añaden entradas

settings-launcher-window-format = Formato de visualización
    .description = Plantilla de fila: {"{w}"} área de trabajo, {"{c}"} clase, {"{t}"} título, {"{n}"} nombre, {"{r}"} rol

settings-launcher-window-match-fields = Campos de coincidencia
    .description = Campos de ventana en los que se busca al escribir

settings-launcher-window-hide-active = Ocultar la ventana activa
    .description = Oculta de la lista la ventana que tiene el foco

settings-launcher-window-close-on-delete = Cerrar al suprimir
    .description = Mayús+Supr cierra la ventana seleccionada

settings-launcher-ssh-client = Cliente SSH
    .description = Binario del cliente SSH

settings-launcher-ssh-command = Orden de conexión
    .description = Plantilla de conexión ({"{terminal}"}, {"{ssh-client}"}, {"{host}"})

settings-launcher-ssh-parse-hosts = Analizar /etc/hosts
    .description = Incluye los equipos de /etc/hosts

settings-launcher-ssh-parse-known-hosts = Analizar known_hosts
    .description = Incluye los equipos de ~/.ssh/known_hosts

settings-launcher-filebrowser-directory = Carpeta inicial
    .description = Carpeta en la que se abre el explorador (vacío = carpeta personal)

settings-launcher-filebrowser-sorting-method = Ordenación
    .description = Orden de los archivos

settings-launcher-filebrowser-directories-first = Carpetas primero
    .description = Muestra las carpetas antes que los archivos

settings-launcher-filebrowser-show-hidden = Mostrar ocultos
    .description = Muestra los archivos ocultos

settings-launcher-filebrowser-command = Orden de apertura
    .description = Orden que abre el archivo elegido (vacío = el abridor integrado)

settings-launcher-combi-modes = Modos combinados
    .description = Modos fusionados en la lista combinada

settings-launcher-combi-display-format = Formato de visualización
    .description = Plantilla de fila ({"{mode}"}, {"{text}"})

## LauncherLocation variants
enum-launcher-location-center = Centro
enum-launcher-location-north-west = Arriba a la izquierda
enum-launcher-location-north = Arriba
enum-launcher-location-north-east = Arriba a la derecha
enum-launcher-location-east = Derecha
enum-launcher-location-south-east = Abajo a la derecha
enum-launcher-location-south = Abajo
enum-launcher-location-south-west = Abajo a la izquierda
enum-launcher-location-west = Izquierda

## LauncherMatching variants
enum-launcher-matching-normal = Normal
enum-launcher-matching-regex = Expresión regular
enum-launcher-matching-glob = Glob
enum-launcher-matching-fuzzy = Difusa
enum-launcher-matching-prefix = Prefijo

## LauncherSorting variants
enum-launcher-sorting-levenshtein = Levenshtein
enum-launcher-sorting-fzf = FZF

## LauncherCase variants
enum-launcher-case-insensitive = No distinguir
enum-launcher-case-smart = Inteligente
enum-launcher-case-sensitive = Distinguir

## LauncherDrunField variants
enum-launcher-drun-field-name = Nombre
enum-launcher-drun-field-generic = Nombre genérico
enum-launcher-drun-field-exec = Orden
enum-launcher-drun-field-categories = Categorías
enum-launcher-drun-field-comment = Comentario
enum-launcher-drun-field-keywords = Palabras clave

## LauncherWindowField variants
enum-launcher-window-field-title = Título
enum-launcher-window-field-class = Clase
enum-launcher-window-field-name = Nombre
enum-launcher-window-field-role = Rol
enum-launcher-window-field-desktop = Área de trabajo

## LauncherFileSort variants
enum-launcher-file-sort-name = Nombre
enum-launcher-file-sort-mtime = Modificación
enum-launcher-file-sort-atime = Último acceso
enum-launcher-file-sort-ctime = Creación

## Animations
settings-animations-launcher = Lanzador
    .description = Animación de entrada y salida específica para el lanzador de aplicaciones

settings-launcher-search = Panel de búsqueda
    .description = El panel que abre `lys-shell launcher` sin argumentos

settings-launcher-search-providers = Fuentes
    .description = Qué busca la lista

settings-launcher-search-web-engine = Nombre del buscador web
    .description = El nombre del buscador en la fila de reserva

settings-launcher-search-web-url = URL del buscador web
    .description = La URL, con {"{"}query{"}"} donde va la búsqueda

settings-launcher-search-file-roots = Carpetas del índice de archivos
    .description = Carpetas que recorre el índice de archivos; vacío indexa la carpeta personal

settings-launcher-search-file-exclude = Exclusiones del índice de archivos
    .description = Nombres de carpeta que el índice de archivos omite

settings-launcher-search-commands-dir = Carpeta de órdenes de script
    .description = Vacío usa launcher/commands en la carpeta de configuración

settings-launcher-search-currency-rates = Tipos de cambio
    .description = Obtiene los tipos de referencia diarios del Banco Central Europeo, como máximo una vez al día, para las respuestas de divisas

settings-launcher-search-previews = Vistas previas
    .description = Muestra el panel lateral para archivos, fragmentos, ventanas y entradas del portapapeles, en el panel de búsqueda y en los modos de ventanas y portapapeles

settings-launcher-search-quicklinks = Enlaces rápidos
    .description = Enlaces que abrir por nombre o palabra clave

settings-launcher-search-snippets = Fragmentos
    .description = Texto que copiar por nombre o palabra clave

enum-launcher-search-provider-apps = Aplicaciones
enum-launcher-search-provider-windows = Ventanas abiertas
enum-launcher-search-provider-files = Archivos
enum-launcher-search-provider-calculator = Calculadora
enum-launcher-search-provider-system = Órdenes del sistema
enum-launcher-search-provider-settings = Páginas de configuración
enum-launcher-search-provider-clipboard = Historial del portapapeles
enum-launcher-search-provider-emoji = Emoji
enum-launcher-search-provider-characters = Caracteres Unicode
enum-launcher-search-provider-vault = Caja fuerte de Bitwarden
enum-launcher-search-provider-links = Enlaces rápidos y fragmentos
enum-launcher-search-provider-browser-bookmarks = Marcadores del navegador
enum-launcher-search-provider-browser-history = Historial del navegador
enum-launcher-search-provider-quick-toggles = Conmutadores rápidos
enum-launcher-search-provider-media = Controles multimedia
enum-launcher-search-provider-capture = Capturas de pantalla y grabación
enum-launcher-search-provider-processes = Matar proceso
enum-launcher-search-provider-scripts = Órdenes de script
enum-launcher-search-provider-shell = Órdenes de shell
enum-launcher-search-provider-web = Búsqueda web

## Format placeholders

settings-launcher-drun-display-format-placeholder-name = Nombre de la aplicación

settings-launcher-drun-display-format-placeholder-generic = Nombre genérico (p. ej. Navegador web)

settings-launcher-drun-display-format-placeholder-exec = Orden que ejecuta la entrada

settings-launcher-drun-display-format-placeholder-categories = Categorías de la entrada de escritorio

settings-launcher-drun-display-format-placeholder-comment = Comentario de la entrada de escritorio

settings-launcher-window-format-placeholder-w = Área de trabajo

settings-launcher-window-format-placeholder-c = Clase de la ventana

settings-launcher-window-format-placeholder-t = Título de la ventana

settings-launcher-window-format-placeholder-n = Nombre de la ventana (su clase)

settings-launcher-window-format-placeholder-r = Rol de la ventana (siempre vacío)

settings-launcher-combi-display-format-placeholder-mode = Modo del que procede la fila

settings-launcher-combi-display-format-placeholder-text = El texto propio de la fila

settings-launcher-emoji = Emoji
    .description = Emojis en el panel de búsqueda, la cuadrícula de emojis y el modo emoji

settings-launcher-emoji-skin-tone = Tono de piel
    .description = El tono de piel de los emojis de personas y manos; elegir uno en las acciones de un emoji lo convierte en el predeterminado

## LauncherSkinTone variants
enum-launcher-skin-tone-none = Predeterminado (amarillo)
enum-launcher-skin-tone-light = Claro
enum-launcher-skin-tone-medium-light = Claro medio
enum-launcher-skin-tone-medium = Medio
enum-launcher-skin-tone-medium-dark = Oscuro medio
enum-launcher-skin-tone-dark = Oscuro
