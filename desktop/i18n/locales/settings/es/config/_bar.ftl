### lys-shell Configuration - Bar Settings

## Bar Configuration - Bar Settings

settings-bar-layout = Disposición
    .description = Disposiciones de la barra por monitor que definen la colocación de los módulos

settings-bar-scale = Escala
    .description = Multiplicador de escala propio de la barra para espaciado, radio y otros elementos

settings-bar-inset-edge = Margen del borde
    .description = Separación entre la barra y el borde de la pantalla al que está unida

settings-bar-inset-ends = Margen de los extremos
    .description = Separación en los extremos de la barra (izquierda/derecha en horizontal, arriba/abajo en vertical)

settings-bar-padding = Relleno
    .description = Espaciado interno a lo largo del grosor de la barra

settings-bar-padding-ends = Relleno de los extremos
    .description = Espaciado interno en los extremos de la barra

settings-bar-module-gap = Separación entre módulos
    .description = Separación entre los módulos y grupos de la barra

settings-bar-location = Ubicación
    .description = Posición de la barra en el borde de la pantalla (arriba, abajo, izquierda, derecha)

settings-bar-exclusive = Exclusiva
    .description = Reserva espacio de pantalla para la barra en lugar de dibujarla sobre las ventanas

settings-bar-auto-hide = Ocultación automática
    .description = Cuándo se oculta la barra, dejando una franja fina en su borde que la muestra bajo el puntero

settings-bar-auto-hide-delay-ms = Retardo de ocultación automática
    .description = Cuánto tiempo permanece la barra tras salir el puntero antes de ocultarse (milisegundos)

settings-bar-layer = Capa
    .description = Capa de layer-shell en la que se coloca la barra

settings-bar-bg = Fondo
    .description = Color de fondo de la barra

settings-bar-background-opacity = Opacidad del fondo
    .description = Transparencia del fondo de la barra (0-100%)

settings-bar-border-location = Ubicación del borde
    .description = Colocación del borde de la barra

settings-bar-border-width = Anchura del borde
    .description = Grosor del borde en píxeles

settings-bar-border-color = Color del borde
    .description = Color del borde de la barra

settings-bar-rounding = Redondeo
    .description = Nivel de redondeo de las esquinas de la barra

settings-bar-shadow-enabled = Sombra
    .description = Activa la sombra paralela de la barra

## Bar Configuration - Button/Module Settings

settings-bar-button-opacity = Opacidad de los botones
    .description = Transparencia de los botones de la barra (0-100%)

settings-bar-button-bg-opacity = Opacidad del fondo de los botones
    .description = Transparencia del color de fondo de los botones de la barra (0-100%)

settings-bar-button-icon-size = Tamaño de los iconos de los botones
    .description = Multiplicador de tamaño de los iconos de los botones

settings-bar-button-icon-padding = Relleno de los iconos de los botones
    .description = Relleno alrededor de los iconos de los botones. Solo se aplica a las variantes block-prefix e icon-square.

settings-bar-button-label-size = Tamaño de las etiquetas de los botones
    .description = Multiplicador de tamaño del texto de los botones

settings-bar-button-label-weight = Peso de las etiquetas de los botones
    .description = Peso de la tipografía de las etiquetas de los botones

settings-bar-button-label-padding = Relleno de las etiquetas de los botones
    .description = Relleno alrededor de las etiquetas de los botones

settings-bar-button-gap = Separación en los botones
    .description = Separación entre el icono y la etiqueta en los botones

settings-bar-button-icon-position = Posición del icono de los botones
    .description = Colocación del icono respecto a la etiqueta (inicio o final)

settings-bar-button-border-location = Ubicación del borde de los botones
    .description = Colocación del borde de los botones de la barra

settings-bar-button-border-width = Anchura del borde de los botones
    .description = Grosor del borde de los botones de la barra en píxeles

## Bar Configuration - Button Group Settings

settings-bar-button-group-border-location = Ubicación del borde de los grupos
    .description = Colocación del borde de los grupos de botones

settings-bar-button-group-border-width = Anchura del borde de los grupos
    .description = Grosor del borde de los grupos de botones en píxeles

settings-bar-button-group-padding = Relleno de los grupos
    .description = Relleno interno de los grupos de botones

settings-bar-group-module-gap = Separación entre módulos del grupo
    .description = Separación entre los módulos de un grupo

settings-bar-button-group-background = Fondo de los grupos
    .description = Color de fondo de los grupos de botones

settings-bar-button-group-opacity = Opacidad de los grupos
    .description = Transparencia de los grupos de botones (0-100%)

settings-bar-button-group-border-color = Color del borde de los grupos
    .description = Color del borde de los grupos de botones

## Bar Configuration - Dropdown Settings

settings-bar-dropdown-size = Tamaño de los desplegables
    .description = Multiplicador de tamaño de los paneles desplegables

settings-bar-dropdown-shadow = Sombra de los desplegables
    .description = Activa la sombra paralela de los menús desplegables

settings-bar-dropdown-opacity = Opacidad de los desplegables
    .description = Transparencia de los paneles desplegables (0-100%)

settings-bar-dropdown-autohide = Ocultación automática de los desplegables
    .description = Cierra el desplegable al pulsar fuera de él

settings-bar-dropdown-freeze-label = Congelar la etiqueta
    .description = Impide que la etiqueta del botón de la barra se actualice mientras su desplegable está abierto

settings-bar-shadow = Sombra
    .description = Activa la sombra paralela

settings-bar-button-variant = Variante de los botones
    .description = Estilo visual de los botones de la barra

settings-bar-button-rounding = Redondeo de los botones
    .description = Redondeo de las esquinas de los botones de la barra

settings-bar-button-group-module-gap = Separación entre módulos del grupo
    .description = Separación entre los módulos de un grupo

settings-bar-button-group-rounding = Redondeo de los grupos
    .description = Redondeo de las esquinas de los grupos de botones

## Location enum variants
enum-location-top = Arriba
enum-location-bottom = Abajo
enum-location-left = Izquierda
enum-location-right = Derecha


## BarAutoHide variants
enum-bar-auto-hide-never = Nunca
enum-bar-auto-hide-always = Siempre
enum-bar-auto-hide-windows-overlap = Cuando las ventanas se superponen
enum-bar-auto-hide-fullscreen = A pantalla completa

## ShadowPreset variants
enum-shadow-preset-none = Ninguna
enum-shadow-preset-drop = Paralela
enum-shadow-preset-floating = Flotante

## BorderLocation variants
enum-border-location-none = Ninguno
enum-border-location-top = Arriba
enum-border-location-bottom = Abajo
enum-border-location-left = Izquierda
enum-border-location-right = Derecha
enum-border-location-all = Todos

## BarButtonVariant variants
enum-bar-button-variant-basic = Básico
enum-bar-button-variant-block-prefix = Prefijo en bloque
enum-bar-button-variant-icon-square = Icono cuadrado

## IconPosition variants
enum-icon-position-start = Inicio
enum-icon-position-end = Final
