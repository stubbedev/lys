### lys-shell Configuration - Styling Settings

## Styling Configuration

settings-styling-scale = Escala
    .description = Multiplicador de escala para desplegables, ventanas emergentes y diálogos

settings-styling-rounding = Redondeo
    .description = Redondeo de las esquinas de desplegables, ventanas emergentes y diálogos

settings-styling-appearance = Apariencia
    .description = Modo claro u oscuro. Automático sigue la paleta o el proveedor; Claro y Oscuro lo fuerzan

settings-styling-high-contrast = Contraste alto
    .description = Una paleta de contraste alto, bordes sólidos y superficies opacas para el shell, y más contraste solicitado a las aplicaciones a través del portal de escritorio

settings-styling-reduce-transparency = Reducir la transparencia
    .description = Hace opacas todas las superficies del shell, sin desenfoque de fondo

settings-styling-theme-provider = Proveedor del tema
    .description = Origen de la paleta de colores (lys-shell, wallpaper, matugen, pywal, wallust)

settings-styling-theming-monitor = Monitor del tema
    .description = Monitor cuyo fondo de escritorio determina la extracción de colores

## Matugen Configuration

settings-styling-matugen-scheme = Esquema de Matugen
    .description = Tipo de esquema de colores para matugen y el proveedor de fondo de escritorio

settings-styling-matugen-contrast = Contraste de Matugen
    .description = Nivel de contraste para matugen y el proveedor de fondo de escritorio (-1.0 a 1.0)

settings-styling-matugen-source-color = Color de origen de Matugen
    .description = Índice del color de origen para matugen y el proveedor de fondo de escritorio

settings-styling-matugen-light = Modo claro de Matugen
    .description = Genera un esquema de colores claro con matugen o el proveedor de fondo de escritorio

## Wallust Configuration

settings-styling-wallust-palette = Paleta de Wallust
    .description = Modo de paleta para la generación de colores de wallust

settings-styling-wallust-saturation = Saturación de Wallust
    .description = Saturación de color personalizada para wallust (0 para desactivar)

settings-styling-wallust-check-contrast = Comprobar contraste de Wallust
    .description = Si wallust impone un contraste mínimo entre colores

settings-styling-wallust-backend = Motor de Wallust
    .description = Método de muestreo de la imagen para la extracción de colores de wallust

settings-styling-wallust-colorspace = Espacio de color de Wallust
    .description = Espacio de color usado para seleccionar el color dominante en wallust

settings-styling-wallust-apply-globally = Aplicar Wallust globalmente
    .description = Aplica los colores de wallust a los terminales y a herramientas externas

## Pywal Configuration

settings-styling-pywal-saturation = Saturación de Pywal
    .description = Saturación de color para pywal (0.0 a 1.0)

settings-styling-pywal-contrast = Contraste de Pywal
    .description = Relación de contraste mínima para pywal (1.0 a 21.0)

settings-styling-pywal-light = Modo claro de Pywal
    .description = Genera un esquema de colores claro con pywal

settings-styling-pywal-apply-globally = Aplicar Pywal globalmente
    .description = Aplica los colores de pywal a los terminales y a herramientas externas

## Palette Configuration

settings-palette-bg = Fondo
    .description = Color de fondo base (capa más oscura)

settings-palette-surface = Superficie
    .description = Color de fondo de tarjetas y barras laterales

settings-palette-elevated = Elevado
    .description = Color de fondo de los elementos elevados

settings-palette-fg = Primer plano
    .description = Color principal del texto

settings-palette-fg-muted = Primer plano atenuado
    .description = Color secundario del texto

settings-palette-primary = Principal
    .description = Color de acento para los elementos interactivos

settings-palette-red = Rojo
    .description = Color semántico rojo

settings-palette-yellow = Amarillo
    .description = Color semántico amarillo

settings-palette-green = Verde
    .description = Color semántico verde

settings-palette-blue = Azul
    .description = Color semántico azul


## Appearance variants
enum-appearance-auto = Automático
enum-appearance-light = Claro
enum-appearance-dark = Oscuro

## ThemeProvider variants
enum-theme-provider-lys-shell = lys-shell
enum-theme-provider-matugen = Matugen
enum-theme-provider-pywal = Pywal
enum-theme-provider-wallust = Wallust
enum-theme-provider-wallpaper = Fondo de escritorio

## MatugenScheme variants
enum-matugen-scheme-content = Contenido
enum-matugen-scheme-expressive = Expresivo
enum-matugen-scheme-fidelity = Fidelidad
enum-matugen-scheme-fruit-salad = Macedonia
enum-matugen-scheme-monochrome = Monocromo
enum-matugen-scheme-neutral = Neutro
enum-matugen-scheme-rainbow = Arcoíris
enum-matugen-scheme-tonal-spot = Punto tonal
enum-matugen-scheme-vibrant = Vibrante

## WallustPalette variants
enum-wallust-palette-dark16 = Dark 16
enum-wallust-palette-dark = Dark
enum-wallust-palette-darkcomp = Darkcomp
enum-wallust-palette-darkcomp16 = Darkcomp16
enum-wallust-palette-harddark = Harddark
enum-wallust-palette-harddark16 = Harddark16
enum-wallust-palette-harddarkcomp = Harddarkcomp
enum-wallust-palette-harddarkcomp16 = Harddarkcomp16
enum-wallust-palette-light = Light
enum-wallust-palette-light16 = Light16
enum-wallust-palette-lightcomp = Lightcomp
enum-wallust-palette-lightcomp16 = Lightcomp16
enum-wallust-palette-softdark = Softdark
enum-wallust-palette-softdark16 = Softdark16
enum-wallust-palette-softdarkcomp = Softdarkcomp
enum-wallust-palette-softdarkcomp16 = Softdarkcomp16
enum-wallust-palette-softlight = Softlight
enum-wallust-palette-softlight16 = Softlight16
enum-wallust-palette-softlightcomp = Softlightcomp
enum-wallust-palette-softlightcomp16 = Softlightcomp16
enum-wallust-palette-ansidark = Ansidark
enum-wallust-palette-ansidark16 = Ansidark16

## WallustBackend variants
enum-wallust-backend-full = Completo
enum-wallust-backend-resized = Redimensionado
enum-wallust-backend-wal = Wal
enum-wallust-backend-thumb = Miniatura
enum-wallust-backend-fastresize = Redimensionado rápido
enum-wallust-backend-kmeans = K-Means

## WallustColorspace variants
enum-wallust-colorspace-lab = Lab
enum-wallust-colorspace-labmixed = Labmixed
enum-wallust-colorspace-lch = Lch
enum-wallust-colorspace-lchmixed = Lchmixed
enum-wallust-colorspace-lchansi = Lchansi

## FontWeightClass variants
enum-font-weight-class-normal = Normal
enum-font-weight-class-medium = Media
enum-font-weight-class-semibold = Seminegrita
enum-font-weight-class-bold = Negrita

## RoundingLevel variants
enum-rounding-level-default = Predeterminado
enum-rounding-level-none = Ninguno
enum-rounding-level-sm = Pequeño
enum-rounding-level-md = Mediano
enum-rounding-level-lg = Grande
enum-rounding-level-full = Completo
