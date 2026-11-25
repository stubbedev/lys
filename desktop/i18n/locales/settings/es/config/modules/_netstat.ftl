### lys-shell Configuration - Netstat Module

## Netstat Module Configuration

settings-modules-netstat-poll-interval-ms = Intervalo de sondeo
    .description = Intervalo de sondeo en milisegundos

settings-modules-netstat-interface = Interfaz
    .description = La interfaz de red que vigilar: la activa, elegida automáticamente, o una por su nombre

settings-modules-netstat-format = Formato de visualización
    .description = Cadena de formato con marcadores: {"{{ down_kib }}"}, {"{{ down_mib }}"}, {"{{ down_gib }}"}, {"{{ down_auto }}"}, {"{{ up_kib }}"}, {"{{ up_mib }}"}, {"{{ up_gib }}"}, {"{{ up_auto }}"}, {"{{ interface }}"}

settings-modules-netstat-icon-name = Nombre del icono
    .description = Nombre del icono simbólico

settings-modules-netstat-border-show = Mostrar el borde
    .description = Muestra un borde alrededor del botón

settings-modules-netstat-border-color = Color del borde
    .description = Token de color del borde

settings-modules-netstat-icon-show = Mostrar el icono
    .description = Muestra el icono del módulo

settings-modules-netstat-tooltip = Mostrar descripción emergente
    .description = Muestra el estado del módulo en una descripción emergente al pasar el puntero

settings-modules-netstat-icon-color = Color del icono
    .description = Color de primer plano del icono

settings-modules-netstat-icon-bg-color = Fondo del icono
    .description = Color de fondo del contenedor del icono

settings-modules-netstat-label-show = Mostrar la etiqueta
    .description = Muestra la etiqueta de texto

settings-modules-netstat-label-color = Color de la etiqueta
    .description = Color del texto de la etiqueta

settings-modules-netstat-label-max-length = Longitud máxima de la etiqueta
    .description = Máximo de caracteres antes de truncar

settings-modules-netstat-button-bg-color = Fondo del botón
    .description = Color de fondo del botón

settings-modules-netstat-left-click = Clic izquierdo
    .description = Orden de shell al hacer clic izquierdo

settings-modules-netstat-right-click = Clic derecho
    .description = Orden de shell al hacer clic derecho

settings-modules-netstat-middle-click = Clic central
    .description = Orden de shell al hacer clic central

settings-modules-netstat-scroll-up = Desplazar hacia arriba
    .description = Orden de shell al desplazar hacia arriba

settings-modules-netstat-scroll-down = Desplazar hacia abajo
    .description = Orden de shell al desplazar hacia abajo

## Format placeholders

settings-modules-netstat-format-placeholder-down-kib = Velocidad de descarga en KiB/s

settings-modules-netstat-format-placeholder-down-mib = Velocidad de descarga en MiB/s

settings-modules-netstat-format-placeholder-down-gib = Velocidad de descarga en GiB/s

settings-modules-netstat-format-placeholder-down-auto = Velocidad de descarga con su unidad (p. ej. 1.5 MiB/s)

settings-modules-netstat-format-placeholder-up-kib = Velocidad de subida en KiB/s

settings-modules-netstat-format-placeholder-up-mib = Velocidad de subida en MiB/s

settings-modules-netstat-format-placeholder-up-gib = Velocidad de subida en GiB/s

settings-modules-netstat-format-placeholder-up-auto = Velocidad de subida con su unidad (p. ej. 256 KiB/s)

settings-modules-netstat-format-placeholder-interface = Nombre de la interfaz (p. ej. wlan0)
