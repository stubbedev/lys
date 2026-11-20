### lys-shell Configuration - Night Light Module

## Night Light Module Configuration

settings-modules-nightlight-temperature = Temperatura
    .description = Temperatura de color en kelvin cuando el filtro está activado (1000-20000)

settings-modules-nightlight-gamma = Gamma
    .description = Porcentaje de gamma de la pantalla cuando el filtro está activado (0-200)

settings-modules-nightlight-auto-schedule = Programación automática
    .description = Activa el filtro gradualmente al anochecer y lo desactiva al amanecer. Sigue el amanecer y el anochecer civiles en la ubicación siguiente

settings-modules-nightlight-transition-duration = Transición
    .description = Duración de la transición entre el día y la noche en minutos, 0 para un cambio instantáneo (0-120)

settings-modules-nightlight-location = Ubicación
    .description = Dónde sigue el horario al sol: detectada automáticamente o una ciudad elegida

settings-modules-nightlight-icon-off = Icono de desactivado
    .description = Icono cuando el filtro está desactivado

settings-modules-nightlight-icon-on = Icono de activado
    .description = Icono cuando el filtro está activado

settings-modules-nightlight-format = Formato
    .description = Cadena de formato de la etiqueta. Marcadores: {"{{ status }}"}, {"{{ temp }}"}, {"{{ gamma }}"}, {"{{ config_temp }}"}, {"{{ config_gamma }}"}

settings-modules-nightlight-border-show = Mostrar el borde
    .description = Muestra un borde alrededor del botón

settings-modules-nightlight-border-color = Color del borde
    .description = Token de color del borde

settings-modules-nightlight-icon-show = Mostrar el icono
    .description = Muestra el icono del módulo

settings-modules-nightlight-tooltip = Mostrar descripción emergente
    .description = Muestra el estado del módulo en una descripción emergente al pasar el puntero

settings-modules-nightlight-icon-color = Color del icono
    .description = Color de primer plano del icono

settings-modules-nightlight-icon-bg-color = Fondo del icono
    .description = Color de fondo del contenedor del icono

settings-modules-nightlight-label-show = Mostrar la etiqueta
    .description = Muestra la etiqueta

settings-modules-nightlight-label-color = Color de la etiqueta
    .description = Color del texto de la etiqueta

settings-modules-nightlight-label-max-length = Longitud máxima de la etiqueta
    .description = Máximo de caracteres antes de truncar

settings-modules-nightlight-button-bg-color = Fondo del botón
    .description = Color de fondo del botón

settings-modules-nightlight-left-click = Clic izquierdo
    .description = Acción al hacer clic izquierdo. Use `:toggle` para el encendido/apagado integrado, vacío para ninguna acción, o una orden de shell

settings-modules-nightlight-right-click = Clic derecho
    .description = Orden de shell al hacer clic derecho

settings-modules-nightlight-middle-click = Clic central
    .description = Orden de shell al hacer clic central

settings-modules-nightlight-scroll-up = Desplazar hacia arriba
    .description = Orden de shell al desplazar hacia arriba

settings-modules-nightlight-scroll-down = Desplazar hacia abajo
    .description = Orden de shell al desplazar hacia abajo

## Format placeholders

settings-modules-nightlight-format-placeholder-status = Estado del filtro (activado, desactivado)

settings-modules-nightlight-format-placeholder-temp = Temperatura actual en kelvin (-- mientras está desactivado)

settings-modules-nightlight-format-placeholder-gamma = Porcentaje de gamma actual (-- mientras está desactivado)

settings-modules-nightlight-format-placeholder-config-temp = Temperatura configurada en kelvin

settings-modules-nightlight-format-placeholder-config-gamma = Porcentaje de gamma configurado
