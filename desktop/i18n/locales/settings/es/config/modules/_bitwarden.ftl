### lys-shell Configuration - Bitwarden Module

## Bitwarden Module Configuration

settings-modules-bitwarden-server = Servidor
    .description = bitwarden.com, bitwarden.eu o la URL base de un servidor propio. Una cuenta con sesión iniciada conserva el servidor en el que inició sesión

settings-modules-bitwarden-email = Correo electrónico
    .description = Correo electrónico con el que empieza el formulario de inicio de sesión

settings-modules-bitwarden-lock-with-session = Bloquear con la sesión
    .description = Bloquea la caja fuerte cuando se bloquea la sesión

settings-modules-bitwarden-unlock-with-session = Desbloquear con la sesión
    .description = Desbloquea una caja fuerte bloqueada con la contraseña de la pantalla de bloqueo, cuando es la contraseña maestra

settings-modules-bitwarden-clipboard-clear = Vaciar el portapapeles tras (segundos)
    .description = Borra un secreto copiado tras estos segundos salvo que se haya copiado otra cosa. 0 lo deja

settings-modules-bitwarden-network-folder = Carpeta de redes
    .description = Carpeta de la caja fuerte cuyos inicios de sesión, con el nombre de la conexión, responden a las solicitudes de contraseña de Wi-Fi y VPN mientras la caja fuerte está abierta. Vacío no responde a ninguna

settings-modules-bitwarden-portal-folder = Carpeta de secretos del portal
    .description = Carpeta de la caja fuerte en la que se guardan las claves del depósito de claves de las aplicaciones aisladas, tras permitir cada una. Vacío las guarda solo en el depósito de claves

settings-modules-bitwarden-generator-passwords = Preajustes de contraseñas
    .description = Contraseñas de caracteres aleatorios que copia el generador del desplegable

settings-modules-bitwarden-generator-passphrases = Preajustes de frases de contraseña
    .description = Frases de contraseña con palabras de la lista larga de la EFF que copia el generador del desplegable

enum-password-charset-lowercase = Minúsculas (a-z)
enum-password-charset-uppercase = Mayúsculas (A-Z)
enum-password-charset-digits = Dígitos (0-9)
enum-password-charset-symbols = Símbolos (!@#$%^&*)

settings-modules-bitwarden-format = Formato
    .description = Cadena de formato de la etiqueta. Marcadores: {"{{ count }}"} (elementos de la caja fuerte abierta)

settings-modules-bitwarden-icon-locked = Icono de bloqueada
    .description = Icono mientras la caja fuerte está bloqueada o sin sesión iniciada

settings-modules-bitwarden-icon-unlocked = Icono de desbloqueada
    .description = Icono mientras la caja fuerte está abierta

settings-modules-bitwarden-border-show = Mostrar el borde
    .description = Muestra un borde alrededor del botón

settings-modules-bitwarden-border-color = Color del borde
    .description = Token de color del borde

settings-modules-bitwarden-icon-show = Mostrar el icono
    .description = Muestra el icono del módulo

settings-modules-bitwarden-tooltip = Mostrar descripción emergente
    .description = Muestra el estado del módulo en una descripción emergente al pasar el puntero

settings-modules-bitwarden-icon-color = Color del icono
    .description = Color de primer plano del icono

settings-modules-bitwarden-icon-bg-color = Fondo del icono
    .description = Color de fondo del contenedor del icono

settings-modules-bitwarden-label-show = Mostrar la etiqueta
    .description = Muestra la etiqueta

settings-modules-bitwarden-label-color = Color de la etiqueta
    .description = Color del texto de la etiqueta

settings-modules-bitwarden-label-max-length = Longitud máxima de la etiqueta
    .description = Máximo de caracteres antes de truncar

settings-modules-bitwarden-button-bg-color = Fondo del botón
    .description = Color de fondo del botón

settings-modules-bitwarden-left-click = Clic izquierdo
    .description = Acción al hacer clic izquierdo. De forma predeterminada abre el desplegable de la caja fuerte

settings-modules-bitwarden-right-click = Clic derecho
    .description = Orden de shell al hacer clic derecho

settings-modules-bitwarden-middle-click = Clic central
    .description = Orden de shell al hacer clic central

settings-modules-bitwarden-scroll-up = Desplazar hacia arriba
    .description = Orden de shell al desplazar hacia arriba

settings-modules-bitwarden-scroll-down = Desplazar hacia abajo
    .description = Orden de shell al desplazar hacia abajo

## Format placeholders

settings-modules-bitwarden-format-placeholder-count = Número de elementos de la caja fuerte abierta (vacío mientras está bloqueada)
