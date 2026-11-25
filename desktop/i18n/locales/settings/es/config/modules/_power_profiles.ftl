### lys-shell Configuration - Power Profiles Module

## Power Profiles Module Configuration

settings-modules-power-profiles-format = Formato
    .description = Lo que muestra la etiqueta, compuesto por los valores del elemento y texto propio

settings-modules-power-profiles-icon-power-saver = Icono de ahorro de energía
    .description = Icono que se muestra mientras el perfil de ahorro de energía está activo

settings-modules-power-profiles-icon-balanced = Icono de equilibrado
    .description = Icono que se muestra mientras el perfil equilibrado está activo

settings-modules-power-profiles-icon-performance = Icono de rendimiento
    .description = Icono que se muestra mientras el perfil de rendimiento está activo

settings-modules-power-profiles-color-power-saver = Color de ahorro de energía
    .description = Color del icono y la etiqueta mientras el perfil de ahorro de energía está activo

settings-modules-power-profiles-color-balanced = Color de equilibrado
    .description = Color del icono y la etiqueta mientras el perfil equilibrado está activo

settings-modules-power-profiles-color-performance = Color de rendimiento
    .description = Color del icono y la etiqueta mientras el perfil de rendimiento está activo

settings-modules-power-profiles-border-show = Mostrar el borde
    .description = Muestra un borde alrededor del botón

settings-modules-power-profiles-border-color = Color del borde
    .description = Token de color del borde

settings-modules-power-profiles-icon-show = Mostrar el icono
    .description = Muestra el icono del módulo

settings-modules-power-profiles-tooltip = Mostrar descripción emergente
    .description = Muestra el estado del módulo en una descripción emergente al pasar el puntero

settings-modules-power-profiles-icon-color = Color del icono
    .description = Color de primer plano del icono (sustituido según el perfil activo)

settings-modules-power-profiles-icon-bg-color = Fondo del icono
    .description = Color de fondo del contenedor del icono

settings-modules-power-profiles-label-show = Mostrar la etiqueta
    .description = Muestra la etiqueta

settings-modules-power-profiles-label-color = Color de la etiqueta
    .description = Color del texto de la etiqueta (sustituido según el perfil activo)

settings-modules-power-profiles-label-max-length = Longitud máxima de la etiqueta
    .description = Caracteres que muestra la etiqueta antes de recortarse; 0 no la recorta nunca

settings-modules-power-profiles-button-bg-color = Fondo del botón
    .description = Color de fondo del botón

settings-modules-power-profiles-left-click = Clic izquierdo
    .description = Acción al hacer clic izquierdo. Use :cycle para el ciclo de perfiles integrado, vacío para ninguna acción, o una orden de shell

settings-modules-power-profiles-right-click = Clic derecho
    .description = Orden de shell al hacer clic derecho

settings-modules-power-profiles-middle-click = Clic central
    .description = Orden de shell al hacer clic central

settings-modules-power-profiles-scroll-up = Desplazar hacia arriba
    .description = Orden de shell al desplazar hacia arriba

settings-modules-power-profiles-scroll-down = Desplazar hacia abajo
    .description = Orden de shell al desplazar hacia abajo

settings-modules-power-profiles-auto-saver-at = Ahorro de energía automático
    .description = Nivel de batería que activa el ahorro de energía hasta que vuelva la corriente alterna

## Format placeholders

settings-modules-power-profiles-format-placeholder-profile = Nombre del perfil activo (power-saver, balanced, performance)

## AutoSaverLevel variants
enum-auto-saver-level-low = Baja
enum-auto-saver-level-critical = Crítica
enum-auto-saver-level-never = Nunca

