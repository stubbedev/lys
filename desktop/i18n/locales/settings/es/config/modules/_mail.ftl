### lys-shell Configuration - Mail Module

## Mail Module Configuration

settings-modules-mail-format = Formato
    .description = Cadena de formato de la etiqueta. Marcadores: {"{{ count }}"}

settings-modules-mail-query = Consulta
    .description = Consulta de búsqueda de notmuch cuyo número de coincidencias se muestra (p. ej. tag:unread)

settings-modules-mail-hide-when-zero = Ocultar cuando sea cero
    .description = Oculta el módulo por completo mientras el recuento sea cero

settings-modules-mail-accounts = Cuentas
    .description = Desglose de no leídos por cuenta que se muestra en el desplegable; el recuento de la barra es su suma

enum-mail-provider-generic = Genérico
enum-mail-provider-gmail = Gmail
enum-mail-provider-outlook = Outlook
enum-mail-provider-icloud = iCloud
enum-mail-provider-proton = Proton
enum-mail-provider-fastmail = Fastmail
enum-mail-provider-yahoo = Yahoo

settings-modules-mail-notify = Notificar el correo nuevo
    .description = Lanza una notificación de escritorio por cada mensaje nuevo (remitente, asunto, icono del proveedor)

settings-modules-mail-notify-summary = Resumen de la notificación
    .description = Texto del resumen. Marcadores: {"{{ sender }}"}, {"{{ subject }}"}, {"{{ count }}"} (total), {"{{ new }}"} (nuevos)

settings-modules-mail-notify-body = Cuerpo de la notificación
    .description = Texto del cuerpo. Marcadores: {"{{ sender }}"}, {"{{ subject }}"}, {"{{ count }}"} (total), {"{{ new }}"} (nuevos)

settings-modules-mail-notify-click = Comando al pulsar la notificación
    .description = Comando de shell que se ejecuta al pulsar una notificación de correo nuevo. Marcador: {"{{ thread }}"} (hilo de notmuch)

settings-modules-mail-icon-name = Icono
    .description = Icono del módulo

settings-modules-mail-border-show = Mostrar el borde
    .description = Muestra un borde alrededor del botón

settings-modules-mail-border-color = Color del borde
    .description = Token de color del borde

settings-modules-mail-icon-show = Mostrar el icono
    .description = Muestra el icono del módulo

settings-modules-mail-tooltip = Mostrar descripción emergente
    .description = Muestra el estado del módulo en una descripción emergente al pasar el puntero

settings-modules-mail-icon-color = Color del icono
    .description = Color de primer plano del icono

settings-modules-mail-icon-bg-color = Fondo del icono
    .description = Color de fondo del contenedor del icono

settings-modules-mail-label-show = Mostrar la etiqueta
    .description = Muestra la etiqueta

settings-modules-mail-label-color = Color de la etiqueta
    .description = Color del texto de la etiqueta

settings-modules-mail-label-max-length = Longitud máxima de la etiqueta
    .description = Máximo de caracteres antes de truncar

settings-modules-mail-button-bg-color = Fondo del botón
    .description = Color de fondo del botón

settings-modules-mail-left-click = Clic izquierdo
    .description = Acción al hacer clic izquierdo. Vacío para ninguna acción, o una orden de shell (p. ej. su cliente de correo)

settings-modules-mail-right-click = Clic derecho
    .description = Orden de shell al hacer clic derecho

settings-modules-mail-middle-click = Clic central
    .description = Orden de shell al hacer clic central

settings-modules-mail-scroll-up = Desplazar hacia arriba
    .description = Orden de shell al desplazar hacia arriba

settings-modules-mail-scroll-down = Desplazar hacia abajo
    .description = Orden de shell al desplazar hacia abajo

## Format placeholders

settings-modules-mail-format-placeholder-count = Número de mensajes que coinciden con la consulta

settings-modules-mail-notify-summary-placeholder-sender = Remitente del mensaje (nombre o dirección)

settings-modules-mail-notify-summary-placeholder-subject = Asunto del mensaje

settings-modules-mail-notify-summary-placeholder-count = Total de mensajes que coinciden con la consulta

settings-modules-mail-notify-summary-placeholder-new = Cuántos han llegado desde el último recuento

settings-modules-mail-notify-body-placeholder-sender = Remitente del mensaje (nombre o dirección)

settings-modules-mail-notify-body-placeholder-subject = Asunto del mensaje

settings-modules-mail-notify-body-placeholder-count = Total de mensajes que coinciden con la consulta

settings-modules-mail-notify-body-placeholder-new = Cuántos han llegado desde el último recuento

settings-modules-mail-notify-click-placeholder-thread = El identificador del hilo de notmuch del mensaje
