### lys-shell Configuration - Niri Workspaces Module

## Niri Workspaces Module Configuration

settings-modules-niri-workspaces-min-workspace-count = Áreas de trabajo mínimas
    .description = Muestra siempre las áreas de trabajo con nombre numérico hasta este índice, aunque estén vacías

settings-modules-niri-workspaces-monitor-specific = Por monitor
    .description = Muestra solo las áreas de trabajo de este monitor

settings-modules-niri-workspaces-hide-trailing-empty = Ocultar la vacía final
    .description = Oculta el área de trabajo vacía que niri asigna automáticamente al final de cada salida

settings-modules-niri-workspaces-display-mode = Modo de visualización
    .description = Qué identifica cada área de trabajo (etiqueta, icono o nada)

settings-modules-niri-workspaces-label-strategy = Estrategia de etiqueta
    .description = Cómo componer la etiqueta del área de trabajo a partir del nombre y el índice

settings-modules-niri-workspaces-divider = Separador
    .description = Texto entre la identidad del área de trabajo y los iconos de las aplicaciones

settings-modules-niri-workspaces-urgent-show = Mostrar urgentes
    .description = Animación de pulso en las áreas de trabajo con ventanas urgentes

settings-modules-niri-workspaces-urgent-mode = Modo de urgencia
    .description = Hace pulsar toda el área de trabajo o solo el icono de la aplicación urgente

settings-modules-niri-workspaces-app-icons-show = Mostrar iconos de aplicaciones
    .description = Muestra los iconos de las ventanas de cada área de trabajo

settings-modules-niri-workspaces-app-icons-dedupe = Eliminar iconos duplicados
    .description = Muestra un icono por app_id en lugar de uno por ventana

settings-modules-niri-workspaces-app-icons-fallback = Icono de reserva
    .description = Icono para las ventanas que no coinciden con app-icon-map

settings-modules-niri-workspaces-app-icons-empty = Icono de vacía
    .description = Icono que se muestra cuando un área de trabajo no tiene ventanas de aplicaciones

settings-modules-niri-workspaces-icon-gap = Separación entre iconos
    .description = Espacio entre los iconos de las aplicaciones

settings-modules-niri-workspaces-workspace-padding = Relleno del área de trabajo
    .description = Relleno para el contenido más ancho que el círculo: iconos de aplicaciones, un separador o un nombre largo. Una etiqueta corta no lleva relleno: en su lugar, esto dimensiona el círculo

settings-modules-niri-workspaces-icon-size = Tamaño de los iconos
    .description = Multiplicador de escala de los iconos de las áreas de trabajo (0.25-3.0)

settings-modules-niri-workspaces-label-size = Tamaño de las etiquetas
    .description = Multiplicador de escala de las etiquetas de las áreas de trabajo (0.25-3.0)

settings-modules-niri-workspaces-workspace-ignore = Ignorar áreas de trabajo
    .description = Patrones glob que se comparan con el nombre, el índice o el id

settings-modules-niri-workspaces-active-indicator = Indicador de activa
    .description = Estilo visual del área de trabajo activa

settings-modules-niri-workspaces-active-color = Color de activa
    .description = Color del icono y la etiqueta del área de trabajo activa

settings-modules-niri-workspaces-occupied-color = Color de ocupada
    .description = Color de los iconos y etiquetas de las áreas de trabajo ocupadas

settings-modules-niri-workspaces-empty-color = Color de vacía
    .description = Color de los iconos y etiquetas de las áreas de trabajo vacías

settings-modules-niri-workspaces-container-bg-color = Fondo del contenedor
    .description = Color de fondo del contenedor de las áreas de trabajo

settings-modules-niri-workspaces-border-show = Mostrar el borde
    .description = Muestra un borde alrededor del contenedor de las áreas de trabajo

settings-modules-niri-workspaces-border-color = Color del borde
    .description = Color del borde del contenedor de las áreas de trabajo

settings-modules-niri-workspaces-workspace-map = Mapa de áreas de trabajo
    .description = Iconos y colores personalizados por área de trabajo, indexados por nombre o id

settings-modules-niri-workspaces-app-icon-map = Mapa de iconos de aplicaciones
    .description = Asignaciones de app_id o título de ventana a icono

settings-modules-niri-workspaces-left-click = Clic izquierdo
    .description = Acción al hacer clic izquierdo

settings-modules-niri-workspaces-middle-click = Clic central
    .description = Acción al hacer clic central

settings-modules-niri-workspaces-right-click = Clic derecho
    .description = Acción al hacer clic derecho

settings-modules-niri-workspaces-scroll-up = Desplazar hacia arriba
    .description = Acción al desplazar hacia arriba

settings-modules-niri-workspaces-scroll-down = Desplazar hacia abajo
    .description = Acción al desplazar hacia abajo


## LabelStrategy variants
enum-label-strategy-index = Índice
enum-label-strategy-name-or-index = Nombre o índice
enum-label-strategy-name-only = Solo el nombre
enum-label-strategy-index-and-name = Índice y nombre
