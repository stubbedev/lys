### lys-shell Configuration - Recorder Module

## Recorder Module Configuration

settings-modules-recorder-icon-idle = Icono de inactivo
    .description = Icono cuando no se graba

settings-modules-recorder-icon-recording = Icono de grabación
    .description = Icono durante la grabación

settings-modules-recorder-icon-paused = Icono de pausa
    .description = Icono mientras la grabación está en pausa

settings-modules-recorder-icon-replay = Icono de repetición
    .description = Icono mientras solo la repetición instantánea está preparada

settings-modules-recorder-format = Formato
    .description = Lo que muestra la etiqueta, compuesto por los valores del elemento y texto propio

settings-modules-recorder-microphone = Micrófono
    .description = Captura el micrófono en la grabación

settings-modules-recorder-microphone-device = Dispositivo de micrófono
    .description = Nombre de la fuente del micrófono (vacío usa la fuente predeterminada)

settings-modules-recorder-system-audio = Audio del sistema
    .description = Captura el audio del escritorio en la grabación

settings-modules-recorder-framerate = Fotogramas por segundo
    .description = Fotogramas por segundo de la captura

settings-modules-recorder-webcam-enabled = Recuadro de la cámara web
    .description = Superpone un recuadro de la cámara web en la grabación

settings-modules-recorder-webcam-device = Dispositivo de cámara web
    .description = Ruta del dispositivo V4L2 (vacío selecciona automáticamente la primera cámara)

settings-modules-recorder-webcam-x = Posición X de la cámara web
    .description = Posición horizontal (0% = izquierda, 100% = derecha), relativa para que se mantenga al cambiar la resolución

settings-modules-recorder-webcam-y = Posición Y de la cámara web
    .description = Posición vertical (0% = arriba, 100% = abajo), relativa para que se mantenga al cambiar la resolución

settings-modules-recorder-webcam-size = Tamaño de la cámara web
    .description = Anchura del recuadro de la cámara web como porcentaje de la anchura de la grabación

settings-modules-recorder-output-directory = Carpeta de salida
    .description = Dónde se guardan las grabaciones (vacío usa la carpeta Vídeos de XDG)

settings-modules-recorder-output-format = Formato de salida
    .description = Formato de contenedor / preajuste de códec

settings-modules-recorder-show-cursor = Mostrar el cursor
    .description = Dibuja el cursor del ratón en la grabación

settings-modules-recorder-start-delay-ms = Retardo de inicio
    .description = Retardo entre elegir la fuente y empezar a grabar, para que el aviso de inicio desaparezca primero de la pantalla

settings-modules-recorder-border-show = Mostrar el borde
    .description = Muestra un borde alrededor del botón

settings-modules-recorder-border-color = Color del borde
    .description = Token de color del borde

settings-modules-recorder-icon-show = Mostrar el icono
    .description = Muestra el icono del módulo

settings-modules-recorder-tooltip = Mostrar descripción emergente
    .description = Muestra el estado del módulo en una descripción emergente al pasar el puntero

settings-modules-recorder-icon-color = Color del icono
    .description = Color de primer plano del icono

settings-modules-recorder-icon-bg-color = Fondo del icono
    .description = Color de fondo del contenedor del icono

settings-modules-recorder-label-show = Mostrar la etiqueta
    .description = Muestra la etiqueta

settings-modules-recorder-label-color = Color de la etiqueta
    .description = Color del texto de la etiqueta

settings-modules-recorder-label-max-length = Longitud máxima de la etiqueta
    .description = Caracteres que muestra la etiqueta antes de recortarse; 0 no la recorta nunca

settings-modules-recorder-button-bg-color = Fondo del botón
    .description = Color de fondo del botón

settings-modules-recorder-left-click = Clic izquierdo
    .description = Orden de shell al hacer clic izquierdo

settings-modules-recorder-right-click = Clic derecho
    .description = Orden de shell al hacer clic derecho

settings-modules-recorder-middle-click = Clic central
    .description = Orden de shell al hacer clic central

settings-modules-recorder-scroll-up = Desplazar hacia arriba
    .description = Orden de shell al desplazar hacia arriba

settings-modules-recorder-scroll-down = Desplazar hacia abajo
    .description = Orden de shell al desplazar hacia abajo


## RecorderFormat variants
enum-recorder-format-mp4 = MP4
enum-recorder-format-mkv = MKV

## Instant replay

settings-modules-recorder-replay-enable = Repetición instantánea
    .description = Conserva en memoria los últimos segundos de una salida, listos para guardar

settings-modules-recorder-replay-seconds = Duración de la repetición
    .description = Segundos de vídeo que escribe al guardar una repetición (5-600)

settings-modules-recorder-replay-output = Salida de la repetición
    .description = La pantalla capturada: el monitor con el foco al activar la repetición, o uno por su conector, como DP-1

## Format placeholders

settings-modules-recorder-format-placeholder-state = Estado del grabador (inactivo, repetición preparada, grabando, en pausa)

settings-modules-recorder-format-placeholder-elapsed = Tiempo de grabación transcurrido como M:SS o H:MM:SS (- mientras está inactivo)
