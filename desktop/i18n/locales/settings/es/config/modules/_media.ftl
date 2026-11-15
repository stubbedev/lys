### lys-shell Configuration - Media Module

## Media Module Configuration

settings-modules-media-icon-type = Tipo de icono
    .description = Modo de visualización del icono (default, application, spinning-disc, application-mapped). spinning-disc usa algo más de CPU por la animación

settings-modules-media-player-icons = Iconos de reproductores
    .description = Asignaciones personalizadas de reproductor a icono (patrón glob a nombre de icono)

settings-modules-media-players-ignored = Reproductores ignorados
    .description = Patrones de nombre de bus de reproductores que excluir de la detección

settings-modules-media-player-priority = Prioridad de reproductores
    .description = Orden preferido de reproductores como patrones glob que coinciden con nombres de bus

settings-modules-media-lyrics = Letras
    .description = Muestra letras sincronizadas de LRCLIB en el desplegable multimedia. Envía el artista, el título, el álbum y la duración de cada pista a lrclib.net

settings-modules-media-format = Formato
    .description = Formato de la etiqueta con marcadores: {"{{ title }}"}, {"{{ artist }}"}, {"{{ album }}"}, {"{{ status }}"}, {"{{ status_icon }}"}

settings-modules-media-icon-name = Nombre del icono
    .description = Nombre del icono simbólico para el modo predeterminado

settings-modules-media-spinning-disc-icon = Icono de disco giratorio
    .description = Icono que se muestra en el modo spinning-disc

settings-modules-media-border-show = Mostrar el borde
    .description = Muestra un borde alrededor del botón

settings-modules-media-border-color = Color del borde
    .description = Token de color del borde

settings-modules-media-icon-show = Mostrar el icono
    .description = Muestra el icono del módulo

settings-modules-media-tooltip = Mostrar descripción emergente
    .description = Muestra el estado del módulo en una descripción emergente al pasar el puntero

settings-modules-media-icon-color = Color del icono
    .description = Color de primer plano del icono

settings-modules-media-icon-bg-color = Fondo del icono
    .description = Color de fondo del contenedor del icono

settings-modules-media-label-show = Mostrar la etiqueta
    .description = Muestra la etiqueta de texto

settings-modules-media-label-color = Color de la etiqueta
    .description = Color del texto de la etiqueta

settings-modules-media-label-max-length = Longitud máxima de la etiqueta
    .description = Máximo de caracteres antes de truncar

settings-modules-media-button-bg-color = Fondo del botón
    .description = Color de fondo del botón

settings-modules-media-left-click = Clic izquierdo
    .description = Acción al hacer clic izquierdo

settings-modules-media-right-click = Clic derecho
    .description = Orden de shell al hacer clic derecho

settings-modules-media-middle-click = Clic central
    .description = Orden de shell al hacer clic central

settings-modules-media-scroll-up = Desplazar hacia arriba
    .description = Orden de shell al desplazar hacia arriba

settings-modules-media-scroll-down = Desplazar hacia abajo
    .description = Orden de shell al desplazar hacia abajo


## MediaIconType variants
enum-media-icon-type-default = Predeterminado
enum-media-icon-type-application = Aplicación
enum-media-icon-type-spinning-disc = Disco giratorio
enum-media-icon-type-application-mapped = Aplicación asignada

## Format placeholders

settings-modules-media-format-placeholder-title = Título de la pista

settings-modules-media-format-placeholder-artist = Nombre del artista o artistas

settings-modules-media-format-placeholder-album = Nombre del álbum

settings-modules-media-format-placeholder-status = Estado de reproducción (reproduciendo, en pausa, detenido)

settings-modules-media-format-placeholder-status-icon = Glifo del estado de reproducción
