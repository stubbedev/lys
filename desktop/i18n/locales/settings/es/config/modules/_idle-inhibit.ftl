### lys-shell Configuration - Idle Inhibit Module

## Idle Inhibit Module Configuration

settings-modules-idle-inhibit-startup-duration = Duración al iniciar
    .description = Duración en minutos al iniciarse el servicio (0 para indefinida)

settings-modules-idle-inhibit-icon-inactive = Icono de inactivo
    .description = Icono cuando el inhibidor de inactividad está desactivado

settings-modules-idle-inhibit-icon-active = Icono de activo
    .description = Icono cuando el inhibidor de inactividad está activado

settings-modules-idle-inhibit-format = Formato
    .description = Cadena de formato con marcadores: {"{{ state }}"}, {"{{ remaining }}"}, {"{{ duration }}"}

settings-modules-idle-inhibit-border-show = Mostrar el borde
    .description = Muestra un borde alrededor del botón

settings-modules-idle-inhibit-border-color = Color del borde
    .description = Token de color del borde

settings-modules-idle-inhibit-icon-show = Mostrar el icono
    .description = Muestra el icono del módulo

settings-modules-idle-inhibit-tooltip = Mostrar descripción emergente
    .description = Muestra el estado del módulo en una descripción emergente al pasar el puntero

settings-modules-idle-inhibit-icon-color = Color del icono
    .description = Color de primer plano del icono

settings-modules-idle-inhibit-icon-bg-color = Fondo del icono
    .description = Color de fondo del contenedor del icono

settings-modules-idle-inhibit-label-show = Mostrar la etiqueta
    .description = Muestra la etiqueta

settings-modules-idle-inhibit-label-color = Color de la etiqueta
    .description = Color del texto de la etiqueta

settings-modules-idle-inhibit-label-max-length = Longitud máxima de la etiqueta
    .description = Máximo de caracteres antes de truncar

settings-modules-idle-inhibit-button-bg-color = Fondo del botón
    .description = Color de fondo del botón

settings-modules-idle-inhibit-left-click = Clic izquierdo
    .description = Orden de shell al hacer clic izquierdo

settings-modules-idle-inhibit-right-click = Clic derecho
    .description = Orden de shell al hacer clic derecho

settings-modules-idle-inhibit-middle-click = Clic central
    .description = Orden de shell al hacer clic central

settings-modules-idle-inhibit-scroll-up = Desplazar hacia arriba
    .description = Orden de shell al desplazar hacia arriba

settings-modules-idle-inhibit-scroll-down = Desplazar hacia abajo
    .description = Orden de shell al desplazar hacia abajo

## Format placeholders

settings-modules-idle-inhibit-format-placeholder-state = Estado del inhibidor (activado, desactivado)

settings-modules-idle-inhibit-format-placeholder-remaining = Tiempo restante como M:SS o H:MM:SS (∞ si es indefinido, - si está desactivado)

settings-modules-idle-inhibit-format-placeholder-duration = Duración total en minutos (∞ si es indefinida)
