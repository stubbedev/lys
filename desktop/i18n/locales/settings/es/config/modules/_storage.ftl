### lys-shell Configuration - Storage Module

## Storage Module Configuration

settings-modules-storage-poll-interval-ms = Intervalo de sondeo
    .description = Intervalo de sondeo en milisegundos

settings-modules-storage-mount-point = Puntos de montaje
    .description = Punto o puntos de montaje que vigilar (p. ej. "/" o ["/", "/mnt/drive1"])

settings-modules-storage-format = Formato de visualización
    .description = Cadena de formato con marcadores: {"{{ percent }}"}, {"{{ used_tib }}"}, {"{{ used_gib }}"}, {"{{ used_mib }}"}, {"{{ used_auto }}"}, {"{{ total_tib }}"}, {"{{ total_gib }}"}, {"{{ total_mib }}"}, {"{{ total_auto }}"}, {"{{ free_tib }}"}, {"{{ free_gib }}"}, {"{{ free_mib }}"}, {"{{ free_auto }}"}, {"{{ filesystem }}"}

settings-modules-storage-icon-name = Nombre del icono
    .description = Nombre del icono simbólico

settings-modules-storage-border-show = Mostrar el borde
    .description = Muestra un borde alrededor del botón

settings-modules-storage-border-color = Color del borde
    .description = Token de color del borde

settings-modules-storage-icon-show = Mostrar el icono
    .description = Muestra el icono del módulo

settings-modules-storage-tooltip = Mostrar descripción emergente
    .description = Muestra el estado del módulo en una descripción emergente al pasar el puntero

settings-modules-storage-icon-color = Color del icono
    .description = Color de primer plano del icono

settings-modules-storage-icon-bg-color = Fondo del icono
    .description = Color de fondo del contenedor del icono

settings-modules-storage-label-show = Mostrar la etiqueta
    .description = Muestra la etiqueta de texto

settings-modules-storage-label-color = Color de la etiqueta
    .description = Color del texto de la etiqueta

settings-modules-storage-label-max-length = Longitud máxima de la etiqueta
    .description = Máximo de caracteres antes de truncar

settings-modules-storage-button-bg-color = Fondo del botón
    .description = Color de fondo del botón

settings-modules-storage-left-click = Clic izquierdo
    .description = Orden de shell al hacer clic izquierdo

settings-modules-storage-right-click = Clic derecho
    .description = Orden de shell al hacer clic derecho

settings-modules-storage-middle-click = Clic central
    .description = Orden de shell al hacer clic central

settings-modules-storage-scroll-up = Desplazar hacia arriba
    .description = Orden de shell al desplazar hacia arriba

settings-modules-storage-scroll-down = Desplazar hacia abajo
    .description = Orden de shell al desplazar hacia abajo

settings-modules-storage-thresholds = Umbrales
    .description = Umbrales de color que cambian según el nivel del valor

## Format placeholders

settings-modules-storage-format-placeholder-percent = Uso del disco (0-100)

settings-modules-storage-format-placeholder-used-tib = Espacio usado en TiB

settings-modules-storage-format-placeholder-used-gib = Espacio usado en GiB

settings-modules-storage-format-placeholder-used-mib = Espacio usado en MiB

settings-modules-storage-format-placeholder-used-auto = Espacio usado con su unidad (p. ej. 128.5 GiB)

settings-modules-storage-format-placeholder-total-tib = Espacio total en TiB

settings-modules-storage-format-placeholder-total-gib = Espacio total en GiB

settings-modules-storage-format-placeholder-total-mib = Espacio total en MiB

settings-modules-storage-format-placeholder-total-auto = Espacio total con su unidad

settings-modules-storage-format-placeholder-free-tib = Espacio libre en TiB

settings-modules-storage-format-placeholder-free-gib = Espacio libre en GiB

settings-modules-storage-format-placeholder-free-mib = Espacio libre en MiB

settings-modules-storage-format-placeholder-free-auto = Espacio libre con su unidad

settings-modules-storage-format-placeholder-filesystem = Tipo de sistema de archivos (p. ej. ext4, btrfs)
