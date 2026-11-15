### lys-shell Configuration - Hyprland Workspaces Module

## Hyprland Workspaces Module Configuration

settings-modules-hyprland-workspaces-min-workspace-count = Áreas de trabajo mínimas
    .description = Mínimo de botones que mostrar (0 = solo activas u ocupadas)

settings-modules-hyprland-workspaces-monitor-specific = Por monitor
    .description = Muestra solo las áreas de trabajo de este monitor

settings-modules-hyprland-workspaces-show-special = Mostrar especiales
    .description = Incluye las áreas de trabajo scratchpad

settings-modules-hyprland-workspaces-highlight-active-on-other-monitor = Resaltar la activa en otro monitor
    .description = Usa un color distinto para las áreas de trabajo activas en otros monitores (solo se aplica cuando { settings-modules-hyprland-workspaces-monitor-specific } está desactivado)

settings-modules-hyprland-workspaces-urgent-show = Mostrar urgentes
    .description = Animación de pulso en las áreas de trabajo con ventanas urgentes

settings-modules-hyprland-workspaces-display-mode = Modo de visualización
    .description = Qué identifica cada área de trabajo (etiqueta, icono o nada)

settings-modules-hyprland-workspaces-label-use-name = Usar el nombre
    .description = Muestra el nombre del área de trabajo en lugar del número

settings-modules-hyprland-workspaces-numbering = Numeración
    .description = Cómo se muestran los números de las áreas de trabajo (absolutos o relativos)

settings-modules-hyprland-workspaces-divider = Separador
    .description = Texto entre la identidad del área de trabajo y los iconos de las aplicaciones

settings-modules-hyprland-workspaces-app-icons-show = Mostrar iconos de aplicaciones
    .description = Muestra los iconos de las ventanas de cada área de trabajo

settings-modules-hyprland-workspaces-app-icons-dedupe = Eliminar iconos duplicados
    .description = Muestra un icono por clase de ventana

settings-modules-hyprland-workspaces-app-icons-fallback = Icono de reserva
    .description = Icono para las aplicaciones no reconocidas

settings-modules-hyprland-workspaces-app-icons-empty = Icono de vacía
    .description = Icono provisional para las áreas de trabajo vacías

settings-modules-hyprland-workspaces-icon-gap = Separación entre iconos
    .description = Espacio entre los iconos de las aplicaciones (rem)

settings-modules-hyprland-workspaces-workspace-padding = Relleno del área de trabajo
    .description = Relleno para el contenido más ancho que el círculo: iconos de aplicaciones, un separador o un nombre largo. Una etiqueta corta no lleva relleno: en su lugar, esto dimensiona el círculo

settings-modules-hyprland-workspaces-icon-size = Tamaño de los iconos
    .description = Multiplicador de escala de los iconos de las áreas de trabajo (0.25-3.0)

settings-modules-hyprland-workspaces-label-size = Tamaño de las etiquetas
    .description = Multiplicador de escala de las etiquetas de las áreas de trabajo (0.25-3.0)

settings-modules-hyprland-workspaces-workspace-ignore = Ignorar áreas de trabajo
    .description = Patrones glob de los ID de las áreas de trabajo que ocultar

settings-modules-hyprland-workspaces-active-indicator = Indicador de activa
    .description = Estilo visual del área de trabajo activa

settings-modules-hyprland-workspaces-active-color = Color de activa
    .description = Color de los iconos y etiquetas del área de trabajo con el foco

settings-modules-hyprland-workspaces-active-on-other-monitor-color = Color de activa en otro monitor
    .description = Color de los iconos y etiquetas de las áreas de trabajo con el foco activas en otros monitores (solo se usa cuando { settings-modules-hyprland-workspaces-highlight-active-on-other-monitor } está activado)

settings-modules-hyprland-workspaces-occupied-color = Color de ocupada
    .description = Color de los iconos y etiquetas de las áreas de trabajo ocupadas

settings-modules-hyprland-workspaces-empty-color = Color de vacía
    .description = Color de los iconos y etiquetas de las áreas de trabajo vacías

settings-modules-hyprland-workspaces-container-bg-color = Fondo del contenedor
    .description = Color de fondo del contenedor de las áreas de trabajo

settings-modules-hyprland-workspaces-border-show = Mostrar el borde
    .description = Muestra un borde alrededor del contenedor de las áreas de trabajo

settings-modules-hyprland-workspaces-border-color = Color del borde
    .description = Color del borde del contenedor de las áreas de trabajo

settings-modules-hyprland-workspaces-workspace-map = Mapa de áreas de trabajo
    .description = Iconos y colores personalizados por área de trabajo

settings-modules-hyprland-workspaces-app-icon-map = Mapa de iconos de aplicaciones
    .description = Asignaciones de clase o título de ventana a icono

settings-modules-hyprland-workspaces-urgent-mode = Modo de urgencia
    .description = Cómo se resaltan las áreas de trabajo urgentes (por área de trabajo o por aplicación)


## DisplayMode variants
enum-display-mode-label = Etiqueta
enum-display-mode-icon = Icono
enum-display-mode-none = Ninguno

## ActiveIndicator variants
enum-active-indicator-background = Fondo
enum-active-indicator-underline = Subrayado

settings-modules-hyprland-workspaces-left-click = Clic izquierdo
    .description = Acción al hacer clic izquierdo
settings-modules-hyprland-workspaces-middle-click = Clic central
    .description = Acción al hacer clic central
settings-modules-hyprland-workspaces-right-click = Clic derecho
    .description = Acción al hacer clic derecho
settings-modules-hyprland-workspaces-scroll-up = Desplazar hacia arriba
    .description = Acción al desplazar hacia arriba
settings-modules-hyprland-workspaces-scroll-down = Desplazar hacia abajo
    .description = Acción al desplazar hacia abajo
