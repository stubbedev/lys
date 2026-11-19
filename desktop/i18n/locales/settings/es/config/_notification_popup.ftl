### lys-shell Configuration - Notification Module Settings

## Shared Settings

settings-modules-notifications-blocklist = Lista de bloqueo
    .description = Patrones glob de nombres de aplicaciones cuyas notificaciones se bloquean

settings-modules-notifications-icon-source = Origen de los iconos
    .description = Cómo se resuelven los iconos de las notificaciones

settings-modules-notifications-history-memory = Historial en memoria
    .description = Notificaciones más recientes que se mantienen en memoria; las más antiguas se cargan del disco al desplazarse

settings-modules-notifications-history-stored = Historial en disco
    .description = Notificaciones que se guardan en disco; las más antiguas se eliminan al superar esta cifra

## Popup Settings

settings-modules-notifications-popup-position = Posición de las ventanas emergentes
    .description = Esquina de la pantalla donde aparecen las notificaciones emergentes

settings-modules-notifications-popup-max-visible = Máximo visible
    .description = Número máximo de ventanas emergentes mostradas a la vez antes de apilarlas

settings-modules-notifications-popup-stacking-order = Orden de apilamiento
    .description = Si las nuevas ventanas emergentes aparecen encima o debajo de las existentes

settings-modules-notifications-popup-stack = Apilamiento
    .description = Qué ventanas emergentes se agrupan en una pila que se despliega al pasar el puntero

settings-modules-notifications-popup-opacity = Opacidad en reposo
    .description = Opacidad de la superficie esmerilada de una ventana emergente mientras el puntero está en otro lugar

settings-modules-notifications-popup-margin-x = Margen horizontal
    .description = Distancia al borde izquierdo/derecho de la pantalla

settings-modules-notifications-popup-margin-y = Margen vertical
    .description = Distancia al borde superior/inferior de la pantalla

settings-modules-notifications-popup-gap = Separación entre ventanas emergentes
    .description = Espacio entre las notificaciones emergentes apiladas

settings-modules-notifications-popup-monitor = Monitor de las ventanas emergentes
    .description = Qué monitor muestra las ventanas emergentes: "primary" o un conector como "DP-1"

settings-modules-notifications-popup-layer = Capa de las ventanas emergentes
    .description = Capa de layer-shell en la que se colocan las notificaciones emergentes. El modo de tearing rebaja overlay a top.

settings-modules-notifications-popup-duration = Cierre automático
    .description = Cuánto tiempo permanecen visibles las ventanas emergentes antes de cerrarse

settings-modules-notifications-popup-hover-pause = Pausar al pasar el puntero
    .description = Detiene el temporizador de cierre automático mientras el puntero está sobre una ventana emergente

settings-modules-notifications-popup-close-behavior = Comportamiento al cerrar
    .description = Qué ocurre al cerrar una ventana emergente (descartar o marcar como leída)

settings-modules-notifications-popup-urgency-bar = Barra de urgencia
    .description = Muestra una barra de color en las ventanas emergentes que alcanzan un nivel mínimo de urgencia

settings-modules-notifications-sounds = Sonidos
    .description = Qué notificaciones reproducen un sonido; silencio en No molestar, al compartir la pantalla y con el sonido silenciado

settings-modules-notifications-sound-default = Sonido predeterminado
    .description = Nombre del tema de sonido que reproduce una notificación sin sonido propio

settings-modules-notifications-sound-mute-apps = Aplicaciones silenciadas
    .description = Patrones glob de nombres de aplicaciones cuyas notificaciones nunca reproducen sonido
settings-modules-notifications-sound-volume = Volumen del sonido
    .description = Con qué volumen suenan las notificaciones, como proporción del sonido tal como se grabó
