### lys-shell Configuration - Notifications Module

## Notifications Module Configuration

settings-modules-notifications-enabled = Activado
    .description = Activa el servicio y el módulo de notificaciones

settings-modules-notifications-icon-name = Icono predeterminado
    .description = Icono que se muestra cuando no hay notificaciones

settings-modules-notifications-icon-unread = Icono de no leídas
    .description = Icono que se muestra cuando hay notificaciones

settings-modules-notifications-icon-dnd = Icono de No molestar
    .description = Icono que se muestra cuando No molestar está activo

settings-modules-notifications-border-show = Mostrar el borde
    .description = Muestra un borde alrededor del botón

settings-modules-notifications-border-color = Color del borde
    .description = Token de color del borde

settings-modules-notifications-icon-show = Mostrar el icono
    .description = Muestra el icono del módulo

settings-modules-notifications-tooltip = Mostrar descripción emergente
    .description = Muestra el estado del módulo en una descripción emergente al pasar el puntero

settings-modules-notifications-icon-color = Color del icono
    .description = Color de primer plano del icono

settings-modules-notifications-icon-bg-color = Fondo del icono
    .description = Color de fondo del contenedor del icono

settings-modules-notifications-label-show = Mostrar la etiqueta
    .description = Muestra la etiqueta con el número de notificaciones

settings-modules-notifications-label-color = Color de la etiqueta
    .description = Color del texto de la etiqueta

settings-modules-notifications-label-max-length = Longitud máxima de la etiqueta
    .description = Máximo de caracteres antes de truncar

settings-modules-notifications-button-bg-color = Fondo del botón
    .description = Color de fondo del botón

settings-modules-notifications-middle-click = Clic central
    .description = Orden de shell al hacer clic central

settings-modules-notifications-scroll-up = Desplazar hacia arriba
    .description = Orden de shell al desplazar hacia arriba

settings-modules-notifications-scroll-down = Desplazar hacia abajo
    .description = Orden de shell al desplazar hacia abajo

settings-modules-notifications-left-click = Clic izquierdo
    .description = Acción al hacer clic izquierdo

settings-modules-notifications-right-click = Clic derecho
    .description = Acción al hacer clic derecho

settings-modules-notifications-popup-shadow = Sombra de las ventanas emergentes
    .description = Dibuja una sombra detrás de las notificaciones emergentes

settings-modules-notifications-thresholds = Umbrales
    .description = Umbrales de color que cambian según el nivel del valor


## PopupPosition variants
enum-popup-position-top-left = Arriba a la izquierda
enum-popup-position-top-center = Arriba en el centro
enum-popup-position-top-right = Arriba a la derecha
enum-popup-position-bottom-left = Abajo a la izquierda
enum-popup-position-bottom-center = Abajo en el centro
enum-popup-position-bottom-right = Abajo a la derecha
enum-popup-position-center-left = En el centro a la izquierda
enum-popup-position-center-right = En el centro a la derecha

## StackingOrder variants
enum-stacking-order-newest-first = Las más recientes primero
enum-stacking-order-oldest-first = Las más antiguas primero

## PopupStack variants
enum-popup-stack-per-app = Por aplicación
enum-popup-stack-all = Todas
enum-popup-stack-off = Desactivado

## PopupCloseBehavior variants
enum-popup-close-behavior-dismiss = Descartar
enum-popup-close-behavior-remove = Quitar

## UrgencyBarThreshold variants
enum-urgency-bar-threshold-low = Baja
enum-urgency-bar-threshold-normal = Normal
enum-urgency-bar-threshold-critical = Crítica
enum-urgency-bar-threshold-none = Ninguno

## IconSource variants
enum-icon-source-automatic = Automático
enum-icon-source-mapped = Asignado
enum-icon-source-application = Aplicación

## NotificationSounds variants
enum-notification-sounds-off = Desactivado
enum-notification-sounds-critical = Solo críticas
enum-notification-sounds-all = Todas

## Do Not Disturb and Focus

settings-modules-notifications-dnd-allow-critical = Permitir críticas
    .description = Las notificaciones críticas, como una batería baja o una alarma, aparecen incluso en No molestar

settings-modules-notifications-dnd-allow-apps = Aplicaciones permitidas
    .description = Nombres de aplicaciones o entradas de escritorio, con comodines * y ?, cuyas notificaciones aparecen incluso en No molestar

settings-modules-notifications-dnd-schedule = Programación
    .description = Momentos de la semana en que No molestar se activa solo; un intervalo que termina antes de empezar continúa pasada la medianoche

settings-modules-notifications-dnd-while-sharing = Al compartir la pantalla
    .description = Activa No molestar mientras la pantalla se comparte o se graba

settings-modules-notifications-dnd-while-fullscreen = A pantalla completa
    .description = Activa No molestar mientras una ventana está a pantalla completa
