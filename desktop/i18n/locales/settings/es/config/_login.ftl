### lys-shell Configuration - Login Screen

## Login screen: the greeter and the lock screen

settings-login-background-mode = Fondo
    .description = Cómo se dibuja el fondo de la pantalla de inicio de sesión: color sólido, una imagen o el fondo de escritorio

settings-login-background-image = Imagen de fondo
    .description = Imagen que se muestra detrás de la pantalla de inicio de sesión cuando el fondo es Imagen

settings-login-background-color = Color de fondo
    .description = Color de relleno detrás de la pantalla de inicio de sesión cuando el fondo es Color

settings-login-blur = Desenfoque
    .description = Cuánto se desenfoca un fondo de imagen o de escritorio; 0 lo deja nítido

settings-login-show-clock = Mostrar el reloj
    .description = Muestra un reloj encima de la solicitud de credenciales

settings-login-clock-format = Formato del reloj
    .description = Cómo se muestra la hora en la pantalla de inicio de sesión

settings-login-date-format = Formato de la fecha
    .description = Cómo se muestra la fecha en la pantalla de inicio de sesión

settings-login-max-attempts = Intentos máximos
    .description = Contraseñas incorrectas en la pantalla de bloqueo antes de bloquear la escritura, con la pantalla aún bloqueada; 0 no pone límite

settings-login-show-failed-attempts = Mostrar intentos fallidos
    .description = Muestra el número de intentos fallidos en la pantalla de bloqueo

settings-login-lock-enabled = Activado
    .description = Permite que lys-shell bloquee la sesión (responde a loginctl lock-session y a lys-shell lock)

settings-login-lock-lock-on-start = Bloquear al iniciar
    .description = Bloquea la sesión en cuanto se inicia el shell, para configuraciones con inicio de sesión automático en las que la pantalla de bloqueo es la puerta de acceso

settings-login-lock-grace-period-ms = Periodo de gracia
    .description = Cuánto tiempo después del bloqueo la pantalla aún se desbloquea sin contraseña

settings-login-lock-blank-timeout-ms = Tiempo hasta oscurecer
    .description = Oscurece la pantalla de bloqueo tras este tiempo sin uso; cuándo se apagan las pantallas se ajusta en la página Inactividad y energía

settings-login-lock-pam-service = Servicio PAM
    .description = Servicio PAM que comprueba la contraseña, como system-auth o login

settings-login-lock-show-media = Mostrar lo que se reproduce
    .description = Muestra la pista en reproducción con reproducir/pausar y siguiente en la pantalla de bloqueo (solo carátulas locales)

settings-login-lock-show-battery = Mostrar la batería
    .description = Muestra la carga de la batería en la pantalla de bloqueo; no aparece si no hay batería

settings-login-lock-fingerprint = Huella dactilar
    .description = Desbloquea con una huella dactilar registrada (fprintd) además de con la contraseña

settings-login-lock-show-switch-user = Mostrar Cambiar de usuario
    .description = Ofrece "Cambiar de usuario" en la pantalla de bloqueo cuando el equipo tiene más de una cuenta

settings-login-greeter-show-user-list = Mostrar la lista de usuarios
    .description = Muestra avatares pulsables de los usuarios del equipo encima del campo de nombre de usuario

settings-login-greeter-show-power-buttons = Mostrar los botones de energía
    .description = Muestra los botones de apagar/reiniciar en la parte inferior de la pantalla de inicio de sesión

settings-login-greeter-fingerprint = Inicio de sesión con huella
    .description = Iniciar sesión con una huella registrada (fprintd) además de la contraseña

settings-login-greeter-match-cursor = Igualar mi cursor
    .description = Mostrar el cursor de la última sesión iniciada en lugar del tema y el tamaño definidos aquí

settings-login-greeter-cursor-theme = Tema del cursor
    .description = Tema del cursor en la pantalla de inicio de sesión; sin definir, el del sistema

settings-login-greeter-cursor-size = Tamaño del cursor
    .description = Tamaño lógico del cursor; se escala por pantalla para que las salidas HiDPI tengan un cursor nítido

settings-login-greeter-screen-reader = Lector de pantalla
    .description = Lector de pantalla que inician el menú de accesibilidad de la pantalla de inicio de sesión y Super+Alt+S; oculto si no está instalado

## LockBackground variants
enum-lock-background-color = Color
enum-lock-background-image = Imagen
enum-lock-background-wallpaper = Fondo de escritorio
