

settings-osd-enabled = Activado
    .description = Muestra superposiciones en pantalla para el volumen, el brillo, etc.

settings-osd-presets = Avisos predefinidos
    .description = Avisos reutilizables que se pueden lanzar con `lys-shell toast --preset <id>`

settings-osd-position = Posición
    .description = Dónde aparece el OSD en la pantalla (p. ej. top-center, bottom-right)

settings-osd-duration = Duración
    .description = Cuánto tiempo permanece visible el OSD antes de cerrarse automáticamente

settings-osd-monitor = Monitor
    .description = Qué monitor muestra el OSD: "primary" o un conector como "DP-1"

settings-osd-margin = Margen
    .description = Distancia del borde de la pantalla al OSD

settings-osd-border = Borde
    .description = Dibuja un borde alrededor de la superposición del OSD

settings-osd-layer = Capa
    .description = Capa de layer-shell en la que se coloca el OSD. El modo de tearing rebaja overlay a top.

settings-osd-text-align = Alineación del texto
    .description = Alineación horizontal del contenido de los avisos y conmutadores del OSD

settings-osd-media = Reproduciendo ahora
    .description = Cambios de pista que muestran el OSD de reproducción: solicitados (teclas multimedia y `lys-shell media next`), siempre o nunca


## OsdPosition variants
enum-osd-position-top-left = Arriba a la izquierda
enum-osd-position-top = Arriba
enum-osd-position-top-right = Arriba a la derecha
enum-osd-position-right = Derecha
enum-osd-position-bottom-right = Abajo a la derecha
enum-osd-position-bottom = Abajo
enum-osd-position-bottom-left = Abajo a la izquierda
enum-osd-position-left = Izquierda


## OsdTextAlign variants
enum-osd-text-align-start = Inicio
enum-osd-text-align-center = Centro
enum-osd-text-align-end = Final


## OsdMedia variants
enum-osd-media-requested = Solicitado
enum-osd-media-always = Siempre
enum-osd-media-never = Nunca
