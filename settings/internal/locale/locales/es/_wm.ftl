### lys-settings: the lys-wm pages

settings-nav-wm-windows = Ventanas
settings-page-wm-windows = Ventanas
settings-nav-wm-effects = Sombra y desenfoque
settings-page-wm-effects = Sombra y desenfoque
settings-nav-wm-tiling = Mosaico y pestañas
settings-page-wm-tiling = Mosaico y pestañas
settings-nav-wm-binds = Atajos de teclado
settings-page-wm-binds = Atajos de teclado
settings-nav-wm-outputs = Pantallas
settings-page-wm-outputs = Pantallas
settings-nav-wm-session = Reglas y sesión
settings-page-wm-session = Reglas y sesión

wm-section-gaps = Separaciones
    .description = Espacio alrededor y entre las ventanas en mosaico
wm-section-borders = Bordes
    .description = La línea dibujada alrededor de cada ventana
wm-section-shape = Forma y transparencia
    .description = Redondeo de las esquinas, y la opacidad de las ventanas según el foco
wm-section-shadow = Sombra
    .description = La sombra bajo las ventanas
wm-section-blur = Desenfoque
    .description = Lo que se ve a través de las ventanas translúcidas
wm-section-overview = Vista general
    .description = La vista de todas las áreas de trabajo a la vez
wm-section-drag = Vista previa al arrastrar
    .description = La ventana que sigue al puntero al arrastrar
wm-section-autotile = Mosaico automático
    .description = Dónde se abre una ventana nueva en una disposición en mosaico
wm-section-tabs = Pestañas
    .description = La barra sobre los grupos de pestañas
wm-section-keyboard = Teclado
    .description = Distribución y repetición de teclas
wm-section-pointer = Ratón
    .description = Ratones y trackballs
wm-section-touchpad = Panel táctil
    .description = Pulsaciones, desplazamiento y velocidad
wm-section-trackpoint = Trackpoint
    .description = Palancas de puntero en el teclado de un portátil
wm-section-binds = Teclas y gestos
    .description = Combinaciones de teclas y ratón y las acciones que ejecutan
wm-section-outputs = Salidas
    .description = Modo, escala, posición y color de cada pantalla
wm-section-display = Pantalla
    .description = Ajustes compartidos por todas las pantallas
wm-section-rules = Reglas de ventanas
    .description = Cómo se abren y se ven las ventanas coincidentes
wm-section-layer-rules = Paneles y superposiciones
    .description = Cómo se ven la barra, el dock, las ventanas emergentes y otras superficies del shell
wm-rules-namespace = Superficie
wm-rules-blur-alpha-threshold = Umbral alfa del desenfoque
wm-layer-lys-shell-bar = Barra
wm-layer-lys-shell-dock = Dock
wm-layer-lys-shell-wallpaper = Fondo de pantalla
wm-layer-lys-shell-launcher = Lanzador
wm-layer-lys-shell-notification-popup = Notificaciones emergentes
wm-layer-lys-shell-osd = Indicador de volumen y brillo
wm-layer-lys-shell-power-menu = Menú de apagado
wm-layer-lys-shell-portal-dialog = Diálogos del portal
wm-layer-lys-shell-file-chooser = Selector de archivos
wm-layer-lys-shell-print = Diálogo de impresión
wm-layer-lys-shell-share-picker = Selector para compartir pantalla
wm-layer-lys-shell-screenshot-reading = Reconocimiento de texto
wm-layer-lys-shell-screenshot-thumbnail = Miniatura de captura
wm-layer-lys-shell-color-picker = Selector de color
wm-layer-lys-shell-region-overlay = Selección de región
wm-layer-lys-shell-window-pick = Selector de ventana
wm-layer-lys-shell-quick-look = Vista rápida
wm-section-start = Inicio
    .description = Lo que se ejecuta con la sesión, y su entorno
wm-section-xwayland = Xwayland
    .description = Aplicaciones X11
wm-section-palette = Paleta
    .description = Nombres de colores que pueden usar las demás claves

enum-wm-toggle-on = Activado
enum-wm-toggle-off = Desactivado
wm-status-not-running = Guardado. No hay ningún lys-wm ejecutándose aquí; el próximo lo leerá.
wm-status-unreadable = No se puede leer la configuración de lys-wm
wm-status-read-only = La configuración de lys-wm no se puede cambiar aquí, así que los cambios no se guardan
wm-status-not-applied = lys-wm no aceptó el cambio
wm-column-widths-unreadable = Los anchos de columna no son de los que acepta lys-wm; edítalos como texto
wm-pointers-connected = Conectados: { $devices }
wm-pointers-device = { $name } ({ $kind })
wm-pointers-class-pointer = ratón
wm-pointers-class-touchpad = panel táctil
wm-pointers-class-trackpoint = trackpoint
wm-pointers-none = No hay ningún ratón, panel táctil ni trackpoint conectado.
wm-pointers-not-running = No hay ningún lys-wm ejecutándose aquí que indique qué dispositivos están conectados, así que se muestran todas las secciones.
wm-pointers-outdated = El lys-wm en ejecución es más antiguo y no indica qué dispositivos están conectados, así que se muestran todas las secciones hasta que vuelvas a iniciar sesión.
wm-pointers-unreadable = No se pudo leer qué dispositivos están conectados, así que se muestran todas las secciones
wm-section-devices = Dispositivos individuales
    .description = Ajustes que un dispositivo guarda para sí
wm-input-devices = Cada dispositivo señalador
    .description = Los ajustes propios de un dispositivo prevalecen sobre los de su tipo de arriba; lo que deja sin definir los sigue.
wm-input-devices-keyboard = Cada teclado
    .description = La distribución propia de un teclado prevalece sobre la de arriba; lo que deja sin definir la sigue.
wm-device-absent = No conectado
    .description = Este dispositivo guarda ajustes propios; vuelven a aplicarse cuando se conecta.
wm-device-forget = Olvidar los ajustes de este dispositivo
wm-devices-none = Ningún dispositivo está conectado ni guarda ajustes propios.
wm-devices-not-running = No hay ningún lys-wm en ejecución aquí que diga qué dispositivos están conectados; se muestran los dispositivos con ajustes propios.
wm-devices-outdated = El lys-wm en ejecución es más antiguo y no dice qué dispositivos están conectados; se muestran los dispositivos con ajustes propios hasta que vuelvas a iniciar sesión.
wm-devices-unreadable = No se pudo leer qué dispositivos están conectados, así que solo se muestran los dispositivos con ajustes propios
wm-devices-reload = Buscar dispositivos de nuevo
wm-rules-pick = Elegir una ventana…
wm-rules-picking = Haz clic en la ventana para la que quieres una regla. Escape cancela.
wm-rules-pick-none = El clic no fue sobre ninguna ventana.
wm-rules-pick-gone = La ventana se cerró antes de poder leerla.
wm-rules-pick-unnamed = Esa ventana no tiene ni ID de aplicación ni título con los que coincidir.
wm-rules-pick-not-running = No hay ningún lys-wm ejecutándose aquí del que elegir una ventana.
wm-rules-pick-failed = No se pudo elegir la ventana
wm-rules-applies-to = Se aplica a
wm-rules-then = Entonces
wm-rules-more-conditions = Más condiciones
wm-rules-fewer-conditions = Menos condiciones
wm-rules-group-placement = Ubicación
wm-rules-group-appearance = Apariencia
wm-rules-group-behaviour = Comportamiento
wm-rules-group-performance = Rendimiento
wm-rules-add-effect = Añadir efecto…
wm-rules-no-effects = Esta regla aún no cambia nada: añade un efecto.
wm-rules-remove-effect = Quitar este efecto
wm-rules-enabled = Aplicar esta regla
wm-rules-apps-running = En ejecución
wm-rules-border = Borde
wm-rules-blur-look = Aspecto del desenfoque
wm-preview-windows = Vista previa de las ventanas
wm-preview-unreadable = La vista previa no puede dibujar { $setting } con el valor { $value }.
wm-source-file = 
    .description = Definido en el archivo de configuración de lys-wm

wm-gaps-inner = Separación interior
    .description = Píxeles entre las ventanas en mosaico
    .keywords = espaciado, relleno, margen
wm-gaps-outer = Separación exterior
    .description = Píxeles entre las ventanas en mosaico y el borde de la pantalla: un número para todos los lados, o cuatro para arriba, derecha, abajo e izquierda
wm-borders-width = Anchura del borde
    .description = En píxeles; 0 no dibuja ninguno
wm-borders-active = Borde con el foco
    .description = Un color, o colores y un ángulo para un degradado
wm-borders-inactive = Borde sin el foco
    .description = Un color, o colores y un ángulo para un degradado
wm-decoration-rounding = Radio de las esquinas
    .description = En píxeles
wm-decoration-active-opacity = Opacidad con el foco
    .description = Lo opaca que es la ventana enfocada; el 100 % oculta lo que hay detrás
wm-decoration-inactive-opacity = Opacidad sin el foco
    .description = Lo opacas que son las ventanas sin foco
wm-decoration-fullscreen-opacity = Opacidad a pantalla completa
    .description = Lo opaca que es una ventana a pantalla completa
wm-decoration-dim-inactive = Atenuar las ventanas sin el foco
    .description = Oscurece todas las ventanas que no tienen el foco
wm-decoration-dim-strength = Intensidad de la atenuación
    .description = Cuánto se oscurecen las ventanas sin foco
wm-decoration-shadow-enabled = Sombras
    .description = Dibuja una sombra bajo las ventanas
wm-decoration-shadow-range = Alcance de la sombra
    .description = En píxeles
wm-decoration-shadow-render-power = Caída de la sombra
    .description = De 1 a 4; cuanto más alto, más rápido se desvanece
wm-decoration-shadow-sharp = Sombras nítidas
    .description = Sin borde suave
wm-decoration-shadow-scale = Escala de la sombra
    .description = Mayor que 0, hasta 1
wm-decoration-shadow-offset = Desplazamiento de la sombra
    .description = Cuánto cae la sombra a la derecha y por debajo de la ventana, en píxeles; un valor negativo la mueve a la izquierda o arriba
wm-shadow-offset-x = A la derecha
wm-shadow-offset-y = Hacia abajo
wm-decoration-shadow-color = Color de la sombra
    .description = Un color de la paleta o uno propio; también el color de la sombra sin el foco salvo que este esté definido
wm-decoration-shadow-color-inactive = Color de la sombra sin el foco
    .description = Un color de la paleta o uno propio
wm-decoration-blur-enabled = Desenfoque
    .description = Desenfoca detrás de todas las ventanas translúcidas, no solo de las que lo piden
    .keywords = cristal esmerilado, transparencia, translúcido
wm-decoration-blur-size = Tamaño del desenfoque
    .description = En píxeles
wm-decoration-blur-passes = Pasadas de desenfoque
    .description = De 1 a 4
wm-decoration-blur-xray = Desenfoque de rayos X
    .description = Desenfoca solo el fondo de escritorio, no las ventanas de debajo
wm-decoration-blur-tint = Tinte del desenfoque
    .description = Un color de la paleta o uno propio
wm-decoration-blur-saturation = Saturación del desenfoque
    .description = De 0 a 4; 1 la deja sin cambios
wm-decoration-blur-brightness = Brillo del desenfoque
    .description = De 0 a 4; 1 lo deja sin cambios
wm-decoration-blur-contrast = Contraste del desenfoque
    .description = De 0 a 4; 1 lo deja sin cambios
wm-decoration-blur-grain = Grano del desenfoque
    .description = Ruido sobre el desenfoque; el 0 % no añade ninguno
wm-animations-enabled = Animaciones
    .description = Anima las ventanas, las áreas de trabajo y las capas
wm-animations-reduced-motion = Movimiento reducido
    .description = Funde en lugar de mover
    .keywords = accesibilidad, mareo por movimiento
wm-overview-blur = Desenfocar la vista general
    .description = Desenfoca lo que hay detrás de la vista general
wm-overview-dim = Atenuación de la vista general
    .description = Cuánto se oscurece el escritorio detrás de la vista general
wm-drag-preview-enabled = Vista previa al arrastrar
    .description = Muestra dónde caerá una ventana arrastrada
wm-drag-preview-opacity = Opacidad de la vista previa
    .description = Lo opaca que es la vista previa
wm-drag-preview-blur = Desenfocar la vista previa
    .description = Desenfoca detrás de la vista previa
wm-drag-preview-snap-distance = Distancia de ajuste
    .description = En píxeles desde un borde
wm-autotile-enabled = Mosaico automático
    .description = Divide una ventana por su lado más largo
wm-autotile-ephemeral = Grupos efímeros
    .description = Un grupo de una sola ventana se disuelve
wm-autotile-min-trigger = Tamaño mínimo para dividir
    .description = En píxeles
wm-autotile-divisor = Divisor de división
    .description = Las ventanas más estrechas que esta fracción de la salida dejan de dividirse
wm-tabs-height = Altura de la barra de pestañas
    .description = En píxeles
wm-tabs-padding = Relleno de la barra de pestañas
    .description = En píxeles
wm-tabs-text-height = Altura del texto de las pestañas
    .description = En píxeles
wm-tabs-text-padding = Relleno del texto de las pestañas
    .description = En píxeles
wm-tabs-font = Tipografía de las pestañas
    .description = Ruta a un archivo de tipografía; vacío usa la predeterminada
wm-tabs-icon-theme = Tema de iconos de las pestañas
    .description = Vacío usa el predeterminado
wm-input-keyboard-layout = Distribuciones del teclado
    .description = Las distribuciones con las que escribes, en el orden en que se alternan; añade una por su nombre, incluidas variantes como Danés (sin teclas muertas)
    .keywords = mapa de teclado, idioma, fuente de entrada
wm-input-sources-misaligned = Estas distribuciones y variantes no se emparejan, hay más variantes que distribuciones, así que ambas se muestran tal cual; elige una distribución o variante para volver a fijarlas
wm-input-keyboard-variant = Variante del teclado
    .description = Variante XKB
wm-input-keyboard-options = Opciones del teclado
    .description = Opciones XKB, como caps:escape
wm-input-keyboard-model = Modelo del teclado
    .description = Modelo XKB
wm-input-keyboard-rules = Reglas del teclado
    .description = Reglas XKB
wm-input-keyboard-repeat-rate = Velocidad de repetición
    .description = Teclas por segundo
    .keywords = repetición de teclas, typematic
wm-input-keyboard-repeat-delay = Retardo de repetición
    .description = Milisegundos antes de que se repita una tecla mantenida
wm-input-pointer-natural-scroll = Desplazamiento natural
    .description = El contenido sigue a los dedos
    .keywords = desplazamiento inverso, desplazamiento invertido, rueda del ratón
wm-input-pointer-middle-emulation = Emulación del clic central
    .description = Pulsar izquierdo y derecho a la vez hace clic con el botón central
wm-input-pointer-accel-profile = Perfil de aceleración
    .description = Plano mueve el puntero tanto como el dispositivo; adaptativo lo acelera con movimientos más rápidos
wm-input-pointer-accel-speed = Velocidad de aceleración
    .description = De -1 a 1
wm-input-pointer-accel-points = Curva de aceleración propia
    .description = La velocidad del puntero en cada paso de la velocidad del dispositivo, en lugar del perfil
wm-input-pointer-scroll-factor = Velocidad de desplazamiento
    .description = Mayor que 0, hasta 100; 1 la deja sin cambios
wm-input-trackpoint-natural-scroll = Desplazamiento natural
    .description = El contenido sigue a los dedos
    .keywords = desplazamiento inverso, desplazamiento invertido, rueda del ratón
wm-input-trackpoint-middle-emulation = Emulación del clic central
    .description = Pulsar izquierdo y derecho a la vez hace clic con el botón central
wm-input-trackpoint-accel-profile = Perfil de aceleración
    .description = Plano mueve el puntero tanto como el dispositivo; adaptativo lo acelera con movimientos más rápidos
wm-input-trackpoint-accel-speed = Velocidad de aceleración
    .description = De -1 a 1
wm-input-trackpoint-accel-points = Curva de aceleración propia
    .description = La velocidad del puntero en cada paso de la velocidad del dispositivo, en lugar del perfil
wm-input-trackpoint-scroll-factor = Velocidad de desplazamiento
    .description = Mayor que 0, hasta 100; 1 la deja sin cambios
wm-input-trackpoint-scroll-method = Método de desplazamiento
    .description = Cómo se desplaza el trackpoint: con un botón mantenido, o de ninguna forma
wm-input-touchpad-natural-scroll = Desplazamiento natural
    .description = El contenido sigue a los dedos
    .keywords = desplazamiento inverso, desplazamiento invertido, panel táctil
wm-input-touchpad-middle-emulation = Emulación del clic central
    .description = Pulsar izquierdo y derecho a la vez hace clic con el botón central
wm-input-touchpad-accel-profile = Perfil de aceleración
    .description = Plano mueve el puntero tanto como el dedo; adaptativo lo acelera con movimientos más rápidos
wm-input-touchpad-accel-speed = Velocidad de aceleración
    .description = De -1 a 1
wm-input-touchpad-accel-points = Curva de aceleración propia
    .description = La velocidad del puntero en cada paso de la velocidad del dispositivo, en lugar del perfil
wm-input-touchpad-scroll-factor = Velocidad de desplazamiento
    .description = Mayor que 0, hasta 100; 1 la deja sin cambios
wm-input-touchpad-tap = Tocar para hacer clic
    .description = Un toque hace clic, sin presionar el panel táctil
    .keywords = tocar para hacer clic, panel táctil
wm-input-touchpad-drag-lock = Bloqueo de arrastre
    .description = Un arrastre con toque continúa aunque se levante el dedo
wm-input-touchpad-disable-while-typing = Desactivar al escribir
    .description = Ignora el panel táctil un momento después de cada pulsación de tecla
wm-input-touchpad-click-method = Método de clic
    .description = Cómo elige el botón una pulsación: por dónde se produce, o por cuántos dedos pulsan
wm-input-touchpad-scroll-method = Método de desplazamiento
    .description = Cómo se desplaza el panel táctil: con dos dedos, por el borde o con un botón mantenido
wm-binds = Atajos
    .description = Una lista de combinación, acción y opciones; una configuración sin ninguno no tiene ninguno
wm-submaps = Submapas
    .description = Conjuntos de atajos con nombre
wm-gestures = Gestos
    .description = Deslizamientos, pellizcos y pulsaciones mantenidas en el panel táctil
wm-outputs = Reglas de salidas
    .description = Cada salida toma la primera regla que coincide con ella
wm-color-management = Gestión del color
    .description = Ofrece a los clientes el protocolo de gestión del color
wm-tearing = Tearing
    .description = Permite que las ventanas que lo piden hagan tearing a pantalla completa
wm-gpu = GPU
    .description = Una tarjeta o nodo de renderizado que usar; vacío elige uno
wm-lid-action = Al cerrar la tapa
    .description = Lo que hace cerrar la tapa con batería y sin pantalla externa conectada: suspender, hibernar, suspensión híbrida, suspender y luego hibernar, nada, o lo que diga el ajuste de tapa del propio sistema
wm-rules = Reglas
    .description = Una lista de coincidencia y efecto
wm-layer-rules = Reglas
    .description = Cada regla cambia el aspecto de las superficies del shell a las que se aplica
wm-autostart = Inicio automático
    .description = Órdenes que se ejecutan una vez iniciada la sesión
wm-env = Entorno
    .description = Variables para lys-wm y las aplicaciones que inicia; un cambio llega a las aplicaciones iniciadas después, no a las que ya se ejecutan
wm-env-name-placeholder = Variable, como GDK_BACKEND
wm-env-name-invalid = Un nombre de variable solo tiene letras, dígitos y _, y no empieza por un dígito; no se guarda hasta que lo cumpla
wm-env-moz-wayland = Firefox se ejecuta en Wayland
wm-env-qt-wayland = Las apps Qt se ejecutan en Wayland
wm-env-gdk-wayland = Las apps GTK se ejecutan en Wayland
wm-env-sdl-wayland = Los juegos SDL se ejecutan en Wayland
wm-env-electron-wayland = Las apps Electron se ejecutan en Wayland cuando pueden
wm-env-java-tiling = Las apps Java dibujan bien sus ventanas
wm-xwayland-enabled = Xwayland
    .description = Ejecuta aplicaciones X11
wm-xwayland-zero-scaling = Escalado cero
    .description = Las aplicaciones X11 dibujan en los píxeles reales de la pantalla en lugar de escalarse borrosas; en una pantalla escalada se ven pequeñas salvo que se escalen solas
wm-palette = Paleta
    .description = Colores con nombre que pueden usar otras claves

## Structured keys
wm-section-gestures = Gestos
    .description = Gestos del panel táctil y lo que controlan
wm-gestures-swipe = Deslizamientos
    .description = Deslizamientos en el panel táctil y lo que controla o ejecuta cada uno
wm-gestures-pinch = Pellizcos
    .description = Pellizcos en el panel táctil y lo que controla o ejecuta cada uno
wm-gestures-hold = Pulsaciones mantenidas
    .description = Dedos mantenidos sobre el panel táctil y la acción que ejecuta cada una
wm-gestures-distance = Distancia del gesto
    .description = Unidades del panel táctil para un deslizamiento completo, unas 12 por milímetro
wm-card-number = Elemento { $number }
wm-side-top = Arriba
wm-side-right = Derecha
wm-side-bottom = Abajo
wm-side-left = Izquierda
wm-border-add-color = Añadir color
wm-border-angle = Ángulo
wm-binds-action = Acción
wm-binds-chord = Combinación
wm-binds-flags = Opciones
wm-binds-cooldown = Tiempo de espera (ms)
wm-binds-description = Descripción
wm-binds-new = Nuevo atajo
wm-binds-long-press = Pulsación larga (ms)
wm-gestures-action = Acción
wm-gestures-direction = Dirección
wm-gestures-drive = Controla
wm-gestures-fingers = Dedos
wm-gestures-modifiers = Modificadores
wm-gestures-scope = Alcance de la vista general
wm-outputs-bitdepth = Profundidad de bits
    .description = 8 o 10 bits por canal; 10 suaviza los degradados con la luz nocturna
wm-outputs-disabled = Desactivada
wm-outputs-hdr = HDR
    .description = Usa la pantalla en HDR siempre, o solo para contenido HDR a pantalla completa
wm-outputs-hdr-peak = Brillo máximo HDR
    .description = Con HDR: el brillo máximo de la pantalla en cd/m², si su EDID lo indica mal
wm-outputs-icc = Perfil ICC
    .description = Un perfil ICC de tipo matriz para una pantalla SDR
wm-outputs-hot-corners = Esquinas activas
    .description = Desactivado desactiva todas las esquinas activas en esta pantalla; Activado cambia algunas de ellas aquí
wm-outputs-hot-corners-top-left = Esquina superior izquierda
wm-outputs-hot-corners-top-left-action = Acción
wm-outputs-hot-corners-top-left-modifiers = Modificadores
wm-outputs-hot-corners-top-right = Esquina superior derecha
wm-outputs-hot-corners-top-right-action = Acción
wm-outputs-hot-corners-top-right-modifiers = Modificadores
wm-outputs-hot-corners-bottom-left = Esquina inferior izquierda
wm-outputs-hot-corners-bottom-left-action = Acción
wm-outputs-hot-corners-bottom-left-modifiers = Modificadores
wm-outputs-hot-corners-bottom-right = Esquina inferior derecha
wm-outputs-hot-corners-bottom-right-action = Acción
wm-outputs-hot-corners-bottom-right-modifiers = Modificadores
wm-outputs-match = Coincidencia
wm-outputs-mirror = Duplicar
    .description = Otra pantalla cuya imagen muestra esta en lugar de su propio escritorio
wm-outputs-mode = Modo
wm-outputs-position = Posición
wm-outputs-reserved = Espacio reservado
    .description = Píxeles lógicos que se mantienen libres de ventanas en mosaico y maximizadas: arriba, derecha, abajo, izquierda
wm-outputs-scale = Escala
wm-outputs-sdr-brightness = Brillo SDR
    .description = Con HDR: los cd/m² con los que se muestra el blanco SDR
wm-outputs-shader = Sombreador
    .description = Un sombreador de fragmentos para la imagen final de la pantalla
wm-outputs-transform = Rotación
    .description = Cómo está girado o volteado el panel
wm-outputs-vrr = Frecuencia de actualización variable
    .description = Sincronización adaptativa: desactivada, siempre activada, o solo para un juego o vídeo a pantalla completa
wm-rules-animation = Animación
wm-rules-animation-in = Animación de apertura
wm-rules-animation-out = Animación de cierre
wm-rules-app-id = ID de la aplicación
wm-rules-block-out-from = Ocultar en las capturas
wm-rules-blur = Desenfoque
wm-rules-blur-look-brightness = Brillo del desenfoque
wm-rules-blur-look-contrast = Contraste del desenfoque
wm-rules-blur-look-grain = Grano del desenfoque
wm-rules-blur-look-refraction-amount = Cantidad de refracción
wm-rules-blur-look-refraction-bezel = Bisel de refracción
wm-rules-blur-look-saturation = Saturación del desenfoque
wm-rules-blur-look-specular-strength = Intensidad especular
wm-rules-blur-look-specular-width = Anchura especular
wm-rules-blur-look-tint = Tinte del desenfoque
wm-rules-border-active = Borde activo
wm-rules-border-inactive = Borde inactivo
wm-rules-border-width = Anchura del borde
wm-rules-dim-around = Atenuar alrededor
wm-rules-dim-inactive = Atenuar cuando está inactiva
wm-rules-float = Flotante
wm-rules-mica = Mica
wm-rules-opacity = Opacidad
wm-rules-size = Tamaño
wm-rules-tag = Etiqueta
wm-rules-tearing = Tearing
wm-rules-touch-gestures = Gestos táctiles
wm-rules-tile = Mosaico
wm-rules-vrr = Frecuencia de actualización variable
wm-rules-title = Título
wm-rules-exclude-app-id = Excepto ID de la aplicación
wm-rules-exclude-is-captured = Excepto capturada
wm-rules-exclude-is-floating = Excepto flotante
wm-rules-exclude-is-focused = Excepto con el foco
wm-rules-exclude-is-fullscreen = Excepto a pantalla completa
wm-rules-exclude-is-urgent = Excepto urgente
wm-rules-exclude-initial-app-id = Excepto ID inicial de la aplicación
wm-rules-exclude-initial-title = Excepto título inicial
wm-rules-exclude-tag = Excepto etiqueta
wm-rules-exclude-title = Excepto título
wm-rules-exclude-xwayland = Excepto Xwayland
wm-rules-focus = Foco
wm-rules-fullscreen = Pantalla completa
wm-rules-group = Grupo
wm-rules-idle-inhibit = Inhibir inactividad
wm-rules-initial-app-id = ID inicial de la aplicación
wm-rules-initial-title = Título inicial
wm-rules-is-captured = Está capturada
wm-rules-is-floating = Es flotante
wm-rules-is-focused = Tiene el foco
wm-rules-is-fullscreen = Está a pantalla completa
wm-rules-is-urgent = Es urgente
wm-rules-keep-aspect = Mantener la relación de aspecto
wm-rules-max-size = Tamaño máximo
wm-rules-min-size = Tamaño mínimo
wm-rules-maximized = Maximizada
wm-rules-output = Salida
wm-rules-pin = Fijar
wm-rules-position = Posición
wm-rules-rounding = Radio de las esquinas
wm-rules-scroll-factor = Factor de desplazamiento
wm-rules-shadow = Sombra
wm-rules-suppress = Solicitudes ignoradas
wm-rules-workspace = Área de trabajo
wm-rules-xwayland = Xwayland
wm-section-focus = Foco
    .description = Lo que obtiene una ventana que pide el foco
wm-focus-on-activate = Al activarse
    .description = Da el foco a una ventana que lo pide justo después de usar la aplicación que lo pidió y, si no, la marca como urgente; o la marca siempre como urgente
wm-rules-on-activate = Al activarse
    .description = Lo que obtiene esta ventana cuando pide el foco: el foco, urgente o nada
wm-section-app-shortcuts = Atajos de aplicaciones
    .description = Atajos que las aplicaciones han registrado a través del portal, cada uno pulsado mediante un atajo global
wm-app-shortcuts = Atajos registrados
    .description = Grabe una combinación para asignarlo, Retroceso para quitar la asignación
wm-app-shortcuts-none = Ninguna aplicación ha registrado aún un atajo
wm-app-shortcuts-hotkey = La aplicación gestiona esta tecla por sí misma, definida en su diálogo del portal
wm-rules-content = Contenido
    .description = Lo que muestra la ventana (foto, vídeo o juego), por encima de lo que ella misma indique
wm-app-shortcuts-reload = Volver a buscar
wm-section-window-behavior = Comportamiento de ventanas
    .description = Cómo se comportan las ventanas nuevas, flotantes y a pantalla completa
wm-section-titlebars = Barras de título
    .description = Barras con botones sobre las ventanas flotantes
wm-section-mica = Mica
    .description = Un desenfoque intenso del fondo detrás de las ventanas translúcidas
wm-section-hot-corners = Esquinas activas
    .description = Qué ocurre al empujar el puntero contra una esquina de la pantalla
wm-section-layout = Disposición
    .description = Cómo organiza sus ventanas un espacio de trabajo nuevo
wm-section-cursor = Puntero
    .description = Cuándo se oculta el puntero y cómo encontrarlo
wm-focus-follows-mouse = El foco sigue al puntero
    .description = Pasar sobre una ventana la enfoca; flexible espera a que el puntero entre en otra ventana
wm-focus-raise-on-hover = Elevar al pasar
    .description = El foco al pasar también trae al frente una ventana flotante
wm-focus-warp-pointer = Mover el puntero con el foco
    .description = Centrar el puntero en la ventana que enfoca un atajo, o solo al cambiar de espacio de trabajo
wm-focus-on-close = Foco al cerrar
    .description = Qué ventana recibe el foco cuando se cierra la enfocada
wm-fullscreen-new-window = Ventana nueva sobre pantalla completa
    .description = Si una ventana que se abre sobre otra a pantalla completa o maximizada queda detrás, la sustituye o la termina
wm-hold-opening = Esperar a las vecinas
    .description = Una ventana nueva espera hasta 100 ms a que las vecinas redimensionadas se redibujen
wm-floating-step = Paso de movimiento flotante
    .description = Píxeles que un atajo mueve una ventana flotante
wm-section-swallow = Absorción por el terminal
    .description = Un terminal se oculta tras la ventana que abre hasta que esta se cierra
wm-swallow-terminals = Terminales que absorben
    .description = Una ventana abierta desde uno de estos terminales ocupa su lugar hasta que se cierra
wm-swallow-except-title = Títulos que nunca absorben
    .description = Una expresión regular sobre el título de la ventana nueva; las ventanas que coinciden se abren junto al terminal
wm-swallow-terminals-none = Ningún terminal absorbe las ventanas que abre
wm-swallow-terminals-add = Añadir terminal
wm-swallow-terminals-written = Un patrón escrito a mano que una lista de terminales no puede expresar
wm-swallow-terminals-replace = Sustituir por una lista
wm-titlebars-floating = Mostrar barras de título
    .description = Barras con botones sobre las ventanas flotantes
wm-titlebars-height = Altura de la barra
    .description = Píxeles, tomados de la ventana flotante
wm-titlebars-buttons = Botones
    .description = Qué botones muestra la barra, en orden
wm-titlebars-side = Lado de los botones
    .description = En qué extremo de la barra están los botones
wm-titlebars-double-click = Doble clic
    .description = Qué hace un doble clic en la barra
wm-titlebars-active-background = Fondo de barra de título, enfocada
    .description = Color: barra de título, enfocada
wm-titlebars-active-text = Texto de barra de título, enfocada
    .description = Color del texto: barra de título, enfocada
wm-titlebars-inactive-background = Fondo de barra de título, sin foco
    .description = Color: barra de título, sin foco
wm-titlebars-inactive-text = Texto de barra de título, sin foco
    .description = Color del texto: barra de título, sin foco
wm-titlebars-urgent-background = Fondo de barra de título, urgente
    .description = Color: barra de título, urgente
wm-titlebars-urgent-text = Texto de barra de título, urgente
    .description = Color del texto: barra de título, urgente
wm-tabs-active-background = Fondo de pestaña, enfocada
    .description = Color: pestaña, enfocada
wm-tabs-active-text = Texto de pestaña, enfocada
    .description = Color del texto: pestaña, enfocada
wm-tabs-inactive-background = Fondo de pestaña, sin foco
    .description = Color: pestaña, sin foco
wm-tabs-inactive-text = Texto de pestaña, sin foco
    .description = Color del texto: pestaña, sin foco
wm-tabs-urgent-background = Fondo de pestaña, urgente
    .description = Color: pestaña, urgente
wm-tabs-urgent-text = Texto de pestaña, urgente
    .description = Color del texto: pestaña, urgente
wm-titlebars-lights-close = Color de Cerrar
    .description = Color del punto del botón Cerrar
wm-titlebars-lights-minimize = Color de Minimizar
    .description = Color del punto del botón Minimizar
wm-titlebars-lights-maximize = Color de Maximizar
    .description = Color del punto del botón Maximizar
wm-decoration-blur-live-tiled = Desenfoque en vivo en mosaico
    .description = Desenfocar lo que hay detrás de cada ventana en mosaico en lugar del desenfoque compartido
wm-decoration-blur-mica-enabled = Mica
    .description = Dibujar las ventanas translúcidas sobre un desenfoque intenso del fondo
wm-decoration-blur-mica-size = Tamaño del desenfoque mica
    .description = Distancia entre muestras de desenfoque, en píxeles
wm-decoration-blur-mica-passes = Pasadas del desenfoque mica
    .description = Cada pasada duplica el alcance
wm-decoration-blur-mica-tint = Tinte mica
    .description = Color sobre el fondo mica
wm-drag-preview-snap-windows = Ajustar a ventanas
    .description = Las ventanas flotantes también se ajustan junto a otras o alineadas con ellas
wm-drag-preview-drag-switch-width = Zona de cambio en el borde
    .description = Píxeles en un borde sin vecina donde mantener un arrastre cambia de espacio
wm-drag-preview-drag-switch-frames = Retardo de cambio en el borde
    .description = Fotogramas que hay que mantener el arrastre en el borde
wm-hot-corners-corners = Esquinas
    .description = Elige una esquina y luego lo que hace empujar el puntero hacia ella
wm-hot-corners-top-left = Arriba a la izquierda
    .description = Acción al empujar el puntero contra esta esquina
wm-hot-corners-top-right = Arriba a la derecha
    .description = Acción al empujar el puntero contra esta esquina
wm-hot-corners-bottom-left = Abajo a la izquierda
    .description = Acción al empujar el puntero contra esta esquina
wm-hot-corners-bottom-right = Abajo a la derecha
    .description = Acción al empujar el puntero contra esta esquina
wm-hot-corners-corner-action = Acción
    .description = Qué hace la esquina
wm-hot-corners-corner-modifiers = Teclas mantenidas
    .description = Teclas que hay que mantener pulsadas
wm-hot-corners-pressure = Empuje en la esquina
    .description = Cuánto debe empujar el puntero contra la esquina
wm-hot-corners-fullscreen = Esquinas a pantalla completa
    .description = Las esquinas también actúan con una ventana a pantalla completa enfocada
wm-showdesktop-peek = Borde al mostrar escritorio
    .description = Porcentaje de cada ventana que queda visible al mostrar el escritorio
wm-switcher-open-delay = Retardo del selector de ventanas
    .description = Milisegundos que hay que mantener las teclas antes de mostrar el selector
wm-layout = Disposición de espacios nuevos
    .description = Divisiones y grupos de pestañas, o una tira de columnas desplazable
wm-scroll-center = Centrar la columna enfocada
    .description = Cuándo la tira desplaza la columna enfocada al centro
wm-input-keyboard-numlock = Bloq Num activado
    .description = Activar Bloq Num al iniciar y tras cada recarga
wm-input-keyboard-sticky-keys = Teclas persistentes
    .description = Un modificador pulsado solo se aplica a la tecla siguiente; dos veces lo bloquea
wm-input-keyboard-slow-keys = Teclas lentas
    .description = Milisegundos que hay que mantener una tecla para que cuente; 0 desactiva
wm-input-pointer-tap = Tocar para hacer clic
    .description = Un toque hace clic sin presionar
wm-input-pointer-drag-lock = Bloqueo de arrastre
    .description = Mantener el arrastre tras levantar el dedo
wm-input-pointer-disable-while-typing = Desactivar al escribir
    .description = Ignorar el dispositivo mientras se escribe
wm-input-pointer-disabled-on-external-mouse = Desactivar con ratón conectado
    .description = Apagar este dispositivo mientras haya otro puntero conectado
wm-input-pointer-left-handed = Para zurdos
    .description = Intercambiar los botones principal y secundario
wm-input-pointer-click-method = Método de clic
    .description = Cómo se lee un clic: zonas de botón o número de dedos
wm-input-pointer-scroll-method = Método de desplazamiento
    .description = Dos dedos, el borde o manteniendo un botón
wm-input-pointer-scroll-button-lock = Botón de desplazamiento alterna
    .description = Un clic en el botón de desplazamiento lo activa y desactiva
wm-input-pointer-tap-button-map = Botones al tocar
    .description = Qué botones pulsan los toques con uno, dos y tres dedos
wm-input-touchpad-disabled-on-external-mouse = Desactivar con ratón conectado
    .description = Apagar este dispositivo mientras haya otro puntero conectado
wm-input-touchpad-left-handed = Para zurdos
    .description = Intercambiar los botones principal y secundario
wm-input-touchpad-scroll-button-lock = Botón de desplazamiento alterna
    .description = Un clic en el botón de desplazamiento lo activa y desactiva
wm-input-touchpad-tap-button-map = Botones al tocar
    .description = Qué botones pulsan los toques con uno, dos y tres dedos
wm-input-trackpoint-tap = Tocar para hacer clic
    .description = Un toque hace clic sin presionar
wm-input-trackpoint-drag-lock = Bloqueo de arrastre
    .description = Mantener el arrastre tras levantar el dedo
wm-input-trackpoint-disable-while-typing = Desactivar al escribir
    .description = Ignorar el dispositivo mientras se escribe
wm-input-trackpoint-disabled-on-external-mouse = Desactivar con ratón conectado
    .description = Apagar este dispositivo mientras haya otro puntero conectado
wm-input-trackpoint-left-handed = Para zurdos
    .description = Intercambiar los botones principal y secundario
wm-input-trackpoint-click-method = Método de clic
    .description = Cómo se lee un clic: zonas de botón o número de dedos
wm-input-trackpoint-scroll-button-lock = Botón de desplazamiento alterna
    .description = Un clic en el botón de desplazamiento lo activa y desactiva
wm-input-trackpoint-tap-button-map = Botones al tocar
    .description = Qué botones pulsan los toques con uno, dos y tres dedos
wm-gestures-edge-width = Zona táctil del borde
    .description = Píxeles desde el borde donde empieza un deslizamiento
wm-cursor-hide-while-typing = Ocultar al escribir
    .description = Ocultar el puntero mientras se escribe en una ventana
wm-cursor-hide-after = Ocultar en reposo
    .description = Milisegundos sin mover el puntero antes de ocultarlo; 0 nunca
wm-cursor-shake-to-find = Agitar para encontrar
    .description = Agitar el ratón agranda el puntero un momento
wm-hidden-frame-rate = Fotogramas de ventanas ocultas
    .description = Fotogramas por segundo para ventanas tapadas o fuera de pantalla
wm-power-saver-frame-rate = Fotogramas en ahorro de energía
    .description = Fotogramas máximos con el perfil de ahorro; 0 desactiva
wm-client-hotkeys = Atajos globales de apps
    .description = Permitir que las apps definan atajos globales que la ventana enfocada no ve
wm-side-all = Todos los lados
wm-side-same = Igual en todos los lados
wm-side-window = Ventana
enum-wm-accel-profile-adaptive = Adaptativa
enum-wm-accel-profile-flat = Plana
enum-wm-buttons-close = Cerrar
enum-wm-buttons-maximize = Maximizar
enum-wm-buttons-minimize = Minimizar
enum-wm-center-always = Siempre
enum-wm-center-never = Nunca
enum-wm-center-on-overflow = Cuando desborda
enum-wm-click-method-button_areas = Áreas de botón
enum-wm-click-method-clickfinger = Número de dedos
enum-wm-click-method-none = Ninguno
enum-wm-double-click-float = Flotar
enum-wm-double-click-maximize = Maximizar
enum-wm-follows-mouse-always = Siempre bajo el puntero
enum-wm-follows-mouse-loose = Al entrar el puntero en otra ventana
enum-wm-follows-mouse-off = Clic para enfocar
enum-wm-layout-scroll = Columnas desplazables
enum-wm-layout-tree = Árbol de mosaico
enum-wm-lid-action-auto = Automático
enum-wm-lid-action-hibernate = Hibernar
enum-wm-lid-action-hybrid-sleep = Suspensión híbrida
enum-wm-lid-action-ignore = No hacer nada
enum-wm-lid-action-logind = Dejarlo al sistema
enum-wm-lid-action-suspend = Suspender
enum-wm-lid-action-suspend-then-hibernate = Suspender y luego hibernar
enum-wm-new-window-behind = Se abre detrás
enum-wm-new-window-exit = Sale de pantalla completa
enum-wm-new-window-take_over = Toma la pantalla completa
enum-wm-on-activate-focus = Enfocarla
enum-wm-on-activate-urgent = Marcarla como urgente
enum-wm-on-close-next = La ventana siguiente
enum-wm-on-close-under_pointer = La ventana bajo el puntero
enum-wm-scroll-method-edge = Borde
enum-wm-scroll-method-none = Ninguno
enum-wm-scroll-method-on_button_down = Con un botón pulsado
enum-wm-scroll-method-two_finger = Dos dedos
enum-wm-side-left = Izquierda
enum-wm-side-right = Derecha
enum-wm-tap-button-map-lmr = Izquierdo, central, derecho
enum-wm-tap-button-map-lrm = Izquierdo, derecho, central
enum-wm-warp-pointer-off = Nunca
enum-wm-warp-pointer-on_keyboard_focus = Cuando el teclado mueve el foco
enum-wm-warp-pointer-on_workspace_switch = Al cambiar de espacio de trabajo
enum-wm-flags-locked = Funciona con la sesión bloqueada
enum-wm-flags-repeating = Se repite al mantener
enum-wm-flags-release = Al soltar
enum-wm-flags-pass = También llega a la ventana
enum-wm-flags-inhibit_proof = Funciona aunque se inhiban los atajos
enum-wm-scope-workspace = Este espacio de trabajo
enum-wm-scope-output = Esta pantalla
enum-wm-scope-app = Esta aplicación
enum-wm-toggle-fullscreen = Solo a pantalla completa
enum-wm-vrr-off = Desactivado
enum-wm-vrr-on = Siempre
enum-wm-vrr-fullscreen = Solo a pantalla completa
enum-wm-transform-normal = Normal
enum-wm-transform-90 = Girada 90°
enum-wm-transform-180 = Girada 180°
enum-wm-transform-270 = Girada 270°
enum-wm-transform-flipped = Volteada
enum-wm-transform-flipped-90 = Volteada, girada 90°
enum-wm-transform-flipped-180 = Volteada, girada 180°
enum-wm-transform-flipped-270 = Volteada, girada 270°
enum-wm-content-none = Ninguno
enum-wm-content-game = Juego
enum-wm-content-photo = Foto
enum-wm-content-video = Vídeo
enum-wm-on-activate-ignore = Ignorarla
settings-nav-wm-keyboard = Teclado
settings-page-wm-keyboard = Teclado
settings-nav-wm-pointer = Ratón y panel táctil
settings-page-wm-pointer = Ratón y panel táctil
wm-section-typing = Escritura
    .description = La rapidez con que se repiten las teclas mantenidas y Bloq Num al iniciar
wm-section-keyboard-accessibility = Accesibilidad
    .description = Teclas que se quedan pulsadas o esperan, para escribir tecla a tecla
wm-section-keyboard-model = Modelo de teclado
    .description = El modelo y las reglas de teclado para los que se construyen las distribuciones
wm-palette-new-name = Nombre del color nuevo
wm-palette-add = Añadir color
wm-palette-reset = Volver al color de lys-wm
wm-palette-taken = Ya hay un color llamado { $name }
wm-palette-add-failed = El color { $name } no se añadió
wm-palette-reset-failed = El color { $name } no se restableció

## What a bind does, by lys-wm action: $first is the first argument, or default without any; $args all of them; $rest all after the first
wm-action-focus = { $first ->
        [left] Enfocar la ventana de la izquierda
        [right] Enfocar la ventana de la derecha
        [up] Enfocar la ventana de arriba
        [down] Enfocar la ventana de abajo
       *[other] Enfocar { $args }
    }
wm-action-move = { $first ->
        [left] Mover la ventana a la izquierda
        [right] Mover la ventana a la derecha
        [up] Mover la ventana hacia arriba
        [down] Mover la ventana hacia abajo
       *[other] Mover la ventana { $args }
    }
wm-action-workspace = { $first ->
        [previous] Cambiar al área de trabajo anterior
        [empty] Cambiar a un área de trabajo vacía
       *[other] Cambiar al área de trabajo { $args }
    }
wm-action-movetoworkspace = { $first ->
        [previous] Mover la ventana al área de trabajo anterior
        [empty] Mover la ventana a un área de trabajo vacía
       *[other] Mover la ventana al área de trabajo { $args }
    }
wm-action-focusoutput = { $first ->
        [left] Enfocar la pantalla de la izquierda
        [right] Enfocar la pantalla de la derecha
        [up] Enfocar la pantalla de arriba
        [down] Enfocar la pantalla de abajo
       *[other] Enfocar la pantalla { $args }
    }
wm-action-movetooutput = { $first ->
        [left] Mover la ventana a la pantalla de la izquierda
        [right] Mover la ventana a la pantalla de la derecha
        [up] Mover la ventana a la pantalla de arriba
        [down] Mover la ventana a la pantalla de abajo
       *[other] Mover la ventana a la pantalla { $args }
    }
wm-action-moveworkspacetooutput = { $first ->
        [left] Mover el área de trabajo a la pantalla de la izquierda
        [right] Mover el área de trabajo a la pantalla de la derecha
        [up] Mover el área de trabajo a la pantalla de arriba
        [down] Mover el área de trabajo a la pantalla de abajo
       *[other] Mover el área de trabajo a la pantalla { $args }
    }
wm-action-swapworkspaces = Intercambiar las áreas de trabajo de las pantallas { $args }
wm-action-scratchpad = { $first ->
        [hide] Ocultar el bloc temporal
       *[other] Alternar el bloc temporal { $rest }
    }
wm-action-movetoscratchpad = Mover la ventana al bloc temporal { $args }
wm-action-kill = Cerrar la ventana
wm-action-focuswindow = Enfocar la ventana { $args }
wm-action-minimize = { $first ->
        [default] Minimizar la ventana
       *[other] Minimizar la ventana { $args }
    }
wm-action-restore = { $first ->
        [default] Restaurar la última ventana minimizada
       *[other] Restaurar la ventana { $args }
    }
wm-action-overview = { $first ->
        [show] Mostrar la vista general
        [hide] Ocultar la vista general
       *[other] Alternar la vista general
    }
wm-action-closewindow = Cerrar la ventana { $args }
wm-action-dpms = { $first ->
        [on] Encender las pantallas
        [off] Apagar las pantallas
       *[other] Encender o apagar las pantallas
    }
wm-action-reload = Recargar la configuración
wm-action-dismissconfignotice = Descartar el aviso de configuración
wm-action-exit = Terminar la sesión
wm-action-nextlayout = Cambiar a la siguiente distribución de teclado
wm-action-stickykeys = { $first ->
        [on] Activar las teclas especiales persistentes
        [off] Desactivar las teclas especiales persistentes
       *[other] Teclas especiales persistentes { $args }
    }
wm-action-slowkeys = Ajustar las teclas lentas a { $args } ms
wm-action-global = Atajo de aplicación { $args }
wm-action-exec = Ejecutar { $args }
wm-action-launch = Iniciar { $app }
wm-action-makegroup = { $first ->
        [h] Agrupar la ventana en paralelo
        [v] Agrupar la ventana apilada
        [tab] Agrupar la ventana como pestañas
       *[other] Agrupar la ventana { $args }
    }
wm-action-setlayout = { $first ->
        [tree] Usar la disposición en mosaico
        [scroll] Usar la disposición con desplazamiento
       *[other] Establecer la disposición { $args }
    }
wm-action-columnwidth = Establecer el ancho de columna { $args }
wm-action-consume = { $first ->
        [left] Mover la ventana a la columna de la izquierda
        [right] Mover la ventana a la columna de la derecha
       *[other] Mover la ventana a la columna { $args }
    }
wm-action-expel = Mover la ventana a una columna propia
wm-action-center = Centrar la columna enfocada
wm-action-columndisplay = { $first ->
        [stack] Apilar las ventanas de la columna
        [tab] Mostrar la columna como pestañas
       *[other] Mostrar la columna como { $args }
    }
wm-action-changegroup = { $first ->
        [h] Disponer el grupo en paralelo
        [v] Disponer el grupo apilado
        [tab] Disponer el grupo como pestañas
        [toggletab] Alternar las pestañas del grupo
        [opposite] Girar el grupo al otro lado
       *[other] Cambiar el grupo { $args }
    }
wm-action-focustab = { $first ->
        [prev] Enfocar la pestaña anterior
        [next] Enfocar la pestaña siguiente
       *[other] Enfocar la pestaña { $args }
    }
wm-action-focusparent = Enfocar el grupo que lo rodea
wm-action-focuschild = Volver a enfocar dentro del grupo
wm-action-swap = { $first ->
        [left] Intercambiar la ventana con la de la izquierda
        [right] Intercambiar la ventana con la de la derecha
        [up] Intercambiar la ventana con la de arriba
        [down] Intercambiar la ventana con la de abajo
       *[other] Intercambiar la ventana { $args }
    }
wm-action-moveout = Sacar la ventana de su grupo
wm-action-equalize = { $first ->
        [all] Igualar todas las divisiones
       *[other] Igualar el grupo
    }
wm-action-locktab = { $first ->
        [on] Bloquear el grupo de pestañas
        [off] Desbloquear el grupo de pestañas
       *[other] Bloquear o desbloquear el grupo de pestañas
    }
wm-action-submap = { $first ->
        [reset] Salir del modo
       *[other] Entrar en el modo { $args }
    }
wm-action-togglefloating = Poner la ventana flotante o en mosaico
wm-action-place = { $first ->
        [left] Colocar la ventana en la mitad izquierda
        [right] Colocar la ventana en la mitad derecha
        [top] Colocar la ventana en la mitad superior
        [bottom] Colocar la ventana en la mitad inferior
        [topleft] Colocar la ventana en el cuarto superior izquierdo
        [topright] Colocar la ventana en el cuarto superior derecho
        [bottomleft] Colocar la ventana en el cuarto inferior izquierdo
        [bottomright] Colocar la ventana en el cuarto inferior derecho
        [fill] Hacer que la ventana llene la pantalla
        [center] Centrar la ventana
        [restore] Devolver la ventana a donde estaba
       *[other] Colocar la ventana { $args }
    }
wm-action-focusfloating = { $first ->
        [on] Enfocar las ventanas flotantes
        [off] Enfocar las ventanas en mosaico
       *[other] Cambiar el foco entre ventanas flotantes y en mosaico
    }
wm-action-fullscreen = { $first ->
        [on] Poner la ventana a pantalla completa
        [off] Salir de pantalla completa
       *[other] Alternar pantalla completa
    }
wm-action-setprop = Establecer la propiedad de ventana { $args }
wm-action-showdesktop = { $first ->
        [on] Mostrar el escritorio
        [off] Devolver las ventanas
       *[other] Mostrar u ocultar el escritorio
    }
wm-action-swallow = { $first ->
        [on] Ocultar el terminal en la ventana
        [off] Mostrar el terminal oculto
       *[other] Mostrar u ocultar el terminal absorbido
    }
wm-action-pin = { $first ->
        [on] Fijar la ventana
        [off] Soltar la ventana
       *[other] Fijar o soltar la ventana
    }
wm-action-maximize = { $first ->
        [on] Maximizar la ventana
        [off] Restaurar el tamaño de la ventana
       *[other] Alternar maximizar
    }
wm-action-resizeactive = Redimensionar la ventana en { $args }
wm-action-movewindow = Arrastrar la ventana
wm-action-resizewindow = Redimensionar la ventana arrastrando
wm-action-zoom = { $first ->
        [in] Acercar
        [out] Alejar
        [reset] Restablecer el zoom
       *[other] Ajustar el zoom a { $rest }
    }
wm-action-debug = Depurar { $args }
wm-action-casttarget = { $first ->
        [window] Compartir una ventana
        [output] Compartir una pantalla
        [clear] Dejar de compartir
       *[other] Compartir { $args }
    }
wm-action-focuslast = Enfocar la ventana anterior
wm-action-focusurgent = Enfocar la ventana que pide atención
wm-action-cycle = { $first ->
        [next] Pasar a la ventana siguiente
        [prev] Pasar a la ventana anterior
       *[other] Recorrer las ventanas { $args }
    }
wm-action-switcher = { $first ->
        [next] Cambiar a la ventana siguiente
        [prev] Cambiar a la ventana anterior
       *[other] Cambiar de ventana { $args }
    }
wm-binds-unknown-action = Acción desconocida
    .description = lys-wm no tiene ninguna acción con este nombre; el atajo se muestra tal como está escrito
wm-binds-category-windows = Ventanas
wm-binds-category-workspaces = Áreas de trabajo
wm-binds-category-layout = Disposición
wm-binds-category-apps = Apps
wm-binds-category-screenshots = Capturas
wm-binds-category-media = Multimedia
wm-binds-category-system = Sistema
wm-binds-category-custom = Personalizados
wm-binds-search = Buscar atajos, acciones o teclas

## The arguments of a lys-wm action, by the form lys-wm describes
wm-action-pick = Elegir una acción
wm-action-args-outside = No es una forma de esta acción
    .description = lys-wm describe los argumentos de esta acción de otro modo; edítalos como texto
enum-wm-arg-default = Predeterminado
enum-wm-arg-all = Todo
enum-wm-arg-app = App enfocada
enum-wm-arg-app_id = ID de app
enum-wm-arg-app_id-colon-id = App y atajo
enum-wm-arg-bottom = Mitad inferior
enum-wm-arg-bottomleft = Inferior izquierda
enum-wm-arg-bottomright = Inferior derecha
enum-wm-arg-center = Centro
enum-wm-arg-clear = Ninguno
enum-wm-arg-command = Orden
enum-wm-arg-down = Abajo
enum-wm-arg-dx = Cambio de ancho (px)
enum-wm-arg-dy = Cambio de alto (px)
enum-wm-arg-e-minus-n = Abrir área anterior, cualquier pantalla
enum-wm-arg-e-plus-n = Abrir área siguiente, cualquier pantalla
enum-wm-arg-empty = Primera vacía
enum-wm-arg-ephemeral = Disolver al quedar una ventana
enum-wm-arg-factor = Factor
enum-wm-arg-fake = Solo dentro de su mosaico
enum-wm-arg-fill = Llenar
enum-wm-arg-follow = Seguir la ventana
enum-wm-arg-h = En paralelo
enum-wm-arg-hide = Ocultar
enum-wm-arg-id = ID de ventana
enum-wm-arg-in = Dentro
enum-wm-arg-key = Propiedad
enum-wm-arg-left = Izquierda
enum-wm-arg-m-minus-n = Abrir área anterior, esta pantalla
enum-wm-arg-m-plus-n = Abrir área siguiente, esta pantalla
enum-wm-arg-minus-n-percent = Más estrecha en porcentaje
enum-wm-arg-minus-n-px = Más estrecha en píxeles
enum-wm-arg-ms = Milisegundos
enum-wm-arg-n = Número
enum-wm-arg-n-percent = Porcentaje de la pantalla
enum-wm-arg-n-px = Píxeles
enum-wm-arg-name = Nombre
enum-wm-arg-next = Siguiente
enum-wm-arg-off = Desactivado
enum-wm-arg-on = Activado
enum-wm-arg-opposite = Al otro lado
enum-wm-arg-out = Fuera
enum-wm-arg-output = Pantalla
enum-wm-arg-output-name = Pantalla
enum-wm-arg-plus-n-percent = Más ancha en porcentaje
enum-wm-arg-plus-n-px = Más ancha en píxeles
enum-wm-arg-preset-next = Preajuste siguiente
enum-wm-arg-preset-prev = Preajuste anterior
enum-wm-arg-prev = Anterior
enum-wm-arg-previous = Anterior
enum-wm-arg-reset = Restablecer
enum-wm-arg-restore = Donde estaba
enum-wm-arg-right = Derecha
enum-wm-arg-scroll = Con desplazamiento
enum-wm-arg-set = Establecer
enum-wm-arg-show = Mostrar
enum-wm-arg-stack = Apiladas
enum-wm-arg-tab = Pestañas
enum-wm-arg-toggle = Alternar
enum-wm-arg-toggle-name = Alternar
enum-wm-arg-toggletab = Alternar pestañas
enum-wm-arg-top = Mitad superior
enum-wm-arg-topleft = Superior izquierda
enum-wm-arg-topright = Superior derecha
enum-wm-arg-tree = Mosaico
enum-wm-arg-up = Arriba
enum-wm-arg-v = Apiladas
enum-wm-arg-value = Valor
enum-wm-arg-window = Ventana
enum-wm-arg-window-id = Ventana
enum-wm-arg-workspace = Área de trabajo
enum-wm-arg-wrap = Dar la vuelta
wm-binds-clash-mode = { $label } en el modo { $mode }
wm-binds-reset = Volver al atajo de lys-wm
wm-mode = Modo { $mode }
wm-mode-enter = Atajo que entra en el modo { $mode }
wm-mode-rename = Cambiar el nombre del modo
wm-mode-rename-confirm = Cambiar nombre
wm-mode-delete-button = Eliminar el modo
wm-mode-delete-title = ¿Eliminar el modo { $mode }?
wm-mode-delete-description = También se quitan sus atajos y los atajos que entran en él.
wm-mode-delete-confirm = Eliminar
wm-mode-add = Añadir modo
wm-mode-add-confirm = Añadir
wm-mode-name = Nombre
wm-mode-name-placeholder = Nombre, como resize
wm-mode-chord = Atajo que entra en el modo
wm-mode-name-missing = Un modo necesita un nombre
wm-mode-name-reset = { $name } es como un atajo sale de un modo, así que no puede nombrar uno
wm-mode-name-taken = Ya existe un modo llamado { $name }
wm-mode-chord-missing = Graba el atajo que entra en el modo
wm-mode-refused = lys-wm no acepta este cambio
wm-mode-not-saved = El cambio no se guardó
wm-mode-delete-failed = No se pudo eliminar el modo
wm-mode-unreachable = Ningún atajo entra en este modo; graba uno junto a su nombre
wm-section-refraction = Refracción
    .description = Cristal desenfocado que curva lo que hay detrás cerca de sus bordes
wm-section-specular = Brillo del borde
    .description = Un reflejo a lo largo del borde del cristal desenfocado, en el lado que mira al puntero
wm-decoration-blur-refraction = Refracción
    .description = Curvar el fondo cerca de los bordes de las ventanas desenfocadas
wm-decoration-blur-refraction-bezel = Borde de curvatura
    .description = Hasta dónde, desde el borde hacia dentro, se curva el fondo, en píxeles
wm-decoration-blur-refraction-amount = Intensidad de la refracción
    .description = Hasta dónde mira el cristal más allá del borde, en píxeles
wm-decoration-blur-specular = Brillo del borde
    .description = Iluminar el borde de las ventanas desenfocadas
wm-decoration-blur-specular-strength = Intensidad del brillo
    .description = Con qué intensidad brilla el borde
wm-decoration-blur-specular-width = Anchura del brillo
    .description = En píxeles
wm-scroll-presets = Anchuras de columna
    .description = Las anchuras que recorre el atajo de anchura de columna en la disposición con desplazamiento, en este orden
    .keywords = preajustes, columnwidth, desplazamiento
wm-scroll-presets-share = Parte de la pantalla
wm-scroll-presets-pixels = Píxeles
wm-scroll-presets-add = Añadir anchura
wm-scroll-presets-add-width = Añadir una columna de { $width } de ancho
wm-scroll-presets-remove = Quitar esta anchura
wm-section-touch-gestures = Gestos de pantalla táctil
    .description = Deslizamientos en una pantalla táctil y lo que hacen
wm-gestures-touch = Deslizamientos táctiles
    .description = Deslizar desde un borde, o con tres o más dedos en cualquier parte de la pantalla
wm-gestures-touch-add = Añadir deslizamiento táctil
wm-gestures-touch-start = Empieza
wm-gestures-touch-does = Hace
wm-gestures-touch-fingers-anywhere = Dedos en cualquier parte
wm-gestures-touch-fingers = Dedos
wm-gestures-touch-direction = Dirección
wm-gestures-touch-run = Ejecutar una acción
wm-gestures-touch-follow = Seguir los dedos
wm-gestures-touch-edge-title = { $edge ->
        [top] Deslizar desde el borde superior
        [bottom] Deslizar desde el borde inferior
        [left] Deslizar desde el borde izquierdo
        [right] Deslizar desde el borde derecho
       *[other] Deslizar desde un borde
    }
wm-gestures-touch-fingers-title = { $direction ->
        [left] Deslizar con { $fingers } dedos a la izquierda
        [right] Deslizar con { $fingers } dedos a la derecha
        [up] Deslizar con { $fingers } dedos hacia arriba
        [down] Deslizar con { $fingers } dedos hacia abajo
        [horizontal] Deslizar con { $fingers } dedos de lado
        [vertical] Deslizar con { $fingers } dedos hacia arriba o abajo
        [any] Deslizar con { $fingers } dedos en cualquier dirección
       *[other] Deslizar con { $fingers } dedos
    }
enum-wm-edge-top = Borde superior
enum-wm-edge-bottom = Borde inferior
enum-wm-edge-left = Borde izquierdo
enum-wm-edge-right = Borde derecho
enum-wm-direction-left = Izquierda
enum-wm-direction-right = Derecha
enum-wm-direction-up = Arriba
enum-wm-direction-down = Abajo
enum-wm-direction-horizontal = De lado
enum-wm-direction-vertical = Arriba o abajo
enum-wm-direction-any = Cualquier dirección
enum-wm-drive-workspaces = Espacios de trabajo
enum-wm-drive-overview = Vista general
enum-wm-drive-view = Desplazar columnas
enum-wm-drive-move = Mover ventana
enum-wm-drive-resize = Redimensionar ventana
wm-input-pointer-scroll-button = Botón de desplazamiento
    .description = El botón que se mantiene pulsado para desplazarse moviendo el dispositivo
wm-input-touchpad-scroll-button = Botón de desplazamiento
    .description = El botón que se mantiene pulsado para desplazarse moviendo el dispositivo
wm-input-trackpoint-scroll-button = Botón de desplazamiento
    .description = El botón que se mantiene pulsado para desplazarse moviendo el dispositivo
wm-button-code = Código del botón
    .description = Otro botón, por su código evdev (BTN_SIDE es 275)
enum-wm-scroll-button-back = Atrás
enum-wm-scroll-button-extra = Extra
enum-wm-scroll-button-forward = Adelante
enum-wm-scroll-button-left = Izquierdo
enum-wm-scroll-button-middle = Central
enum-wm-scroll-button-right = Derecho
enum-wm-scroll-button-side = Lateral
enum-wm-scroll-button-task = Tarea
wm-preview-tabs = Vista previa de la barra de pestañas
wm-preview-tab-active = Enfocada
wm-preview-tab-inactive = Otra ventana
wm-preview-tab-urgent = Pide atención
wm-tabs-font-built-in = La fuente propia de lys-wm
wm-tabs-font-unresolved = No se encontró ningún archivo de fuente de { $family }
wm-tabs-font-unreadable = lys-wm no puede leer el archivo de fuente de { $family }
wm-tabs-font-file-unreadable = No se puede leer el archivo de fuente
wm-section-workspaces = Espacios de trabajo
    .description = Reglas para espacios de trabajo concretos o para todos los que coincidan
wm-section-scratchpads = Ventanas rápidas
    .description = Ventanas con nombre que se muestran y ocultan sobre las demás con un atajo
wm-workspaces = Reglas de espacios de trabajo
    .description = Un espacio por su número, o todos los que coincidan, y cómo se ven y empiezan
wm-workspace-number = Espacio de trabajo { $number }
wm-workspaces-id = Número
wm-workspaces-match = Todos los espacios
wm-workspaces-name = Nombre
wm-workspaces-output = Pantalla
wm-workspaces-default = Se muestra primero en su pantalla
wm-workspaces-persistent = Se conserva aunque esté vacío
wm-workspaces-on-created-empty = App que abrir al crearse vacío
wm-workspaces-layout = Disposición
wm-workspaces-gaps-inner = Espacio entre ventanas
wm-workspaces-gaps-outer = Espacio en los bordes de la pantalla
wm-workspaces-borders-width = Ancho del borde
wm-workspaces-rounding = Redondeo de esquinas
wm-workspaces-shadow = Sombra
wm-scratchpads = Ventanas rápidas
    .description = La app y el tamaño de cada ventana rápida; un atajo la muestra u oculta por su nombre
wm-scratchpads-name = Nombre de la ventana rápida
wm-scratchpads-exec = App
wm-scratchpads-size = Tamaño
wm-scratchpads-hide-on-workspace-switch = Se oculta al cambiar de espacio
wm-size-width = Ancho %
wm-size-height = Alto %
enum-wm-match-any = Cualquier espacio
enum-wm-match-single_tiled = Con una ventana en mosaico
enum-wm-match-fullscreen = Con una ventana a pantalla completa
wm-section-animation-events = Animaciones del gestor de ventanas
    .description = Cómo se mueve cada animación de lys-wm; un valor vacío sigue a la animación de arriba
wm-section-curves = Curvas de animación
    .description = Tus propias curvas de aceleración, que las animaciones usan por su nombre
wm-animations-events = Animaciones
    .description = Si cada animación se reproduce, su curva, su duración y su estilo
wm-animations-curves = Curvas
    .description = Una curva dibujada o un muelle cada una, con nombre para las animaciones
wm-animations-curves-name = Nombre de la curva
wm-animations-curves-shape = Forma
wm-curve-kind-bezier = Curva
wm-curve-kind-spring = Muelle
wm-curve-canvas = Forma de la curva; arrastra los dos tiradores para cambiarla
wm-curve-x1 = Primer tirador, tiempo
wm-curve-y1 = Primer tirador, progreso
wm-curve-x2 = Segundo tirador, tiempo
wm-curve-y2 = Segundo tirador, progreso
wm-curve-stiffness = Rigidez
wm-curve-damping = Amortiguación
wm-curve-mass = Masa
wm-curve-presets = Partir de
wm-accel-curve-none = El perfil de aceleración decide cómo se acelera el puntero
wm-accel-curve-start = Dibujar una curva propia
wm-accel-curve-canvas = Velocidad del puntero según la del dispositivo; arrastra un punto hacia arriba o hacia abajo para cambiarlo
wm-accel-curve-step = Velocidad del dispositivo entre puntos
wm-accel-curve-add = Añadir punto
wm-accel-curve-remove = Quitar el último punto
wm-accel-curve-stop = Usar el perfil de aceleración
wm-curve-own = Tus curvas
wm-curve-linear = Lineal
wm-curve-ease = Suave
wm-curve-ease_in = Suave al entrar
wm-curve-ease_out = Suave al salir
wm-curve-ease_in_out = Suave al entrar y salir
wm-curve-spring = Muelle
wm-anim-column-enabled = Se reproduce
wm-anim-column-curve = Curva
wm-anim-column-duration = Duración (ms)
wm-anim-column-style = Estilo
wm-anim-event-global = Todo
wm-anim-event-windows = Ventanas
wm-anim-event-windows_move = Mover ventanas
wm-anim-event-windows_in = Abrir ventanas
wm-anim-event-windows_out = Cerrar ventanas
wm-anim-event-windows_minimize = Minimizar ventanas
wm-anim-event-snap = Ajustar ventanas
wm-anim-event-fade = Fundidos
wm-anim-event-fade_in = Aparecer
wm-anim-event-fade_out = Desaparecer
wm-anim-event-fade_switch = Cambios de foco
wm-anim-event-fade_dim = Atenuación
wm-anim-event-fade_shadow = Sombras
wm-anim-event-fade_snap = Vista previa del ajuste
wm-anim-event-workspaces = Cambiar de espacio de trabajo
wm-anim-event-layers = Paneles y superposiciones
wm-anim-event-layers_in = Paneles que aparecen
wm-anim-event-layers_out = Paneles que desaparecen
wm-anim-event-borderangle = Degradado del borde
wm-anim-event-zoom = Zoom
wm-anim-event-overview = Vista general
wm-anim-event-scratchpad = Ventanas rápidas
wm-anim-style-popin = Emerger
wm-anim-style-slide = Deslizar
wm-anim-style-slidevert = Deslizar en vertical
wm-anim-style-fade = Fundido
wm-anim-style-once = Una vez
wm-anim-style-loop = En bucle
wm-anim-style-morph = Transformar
wm-anim-style-elastic = Elástico
wm-anim-style-goo = Viscoso
wm-anim-style-bud = Brote
wm-anim-style-genie = Genio
wm-app-shortcuts-unreadable = No se pueden leer los atajos registrados
wm-devices-reading = Preguntando a lys-wm qué dispositivos están conectados…
