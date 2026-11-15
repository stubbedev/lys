### Bitwarden Dropdown

dropdown-bitwarden-title = Bitwarden
dropdown-bitwarden-tooltip-logged-out = Bitwarden: inicie sesión para abrir la caja fuerte
dropdown-bitwarden-tooltip-locked = Bitwarden: la caja fuerte está bloqueada
dropdown-bitwarden-tooltip-unlocked = Bitwarden: la caja fuerte está abierta
dropdown-bitwarden-unavailable = Caja fuerte no disponible
dropdown-bitwarden-server = Servidor: { $server }
dropdown-bitwarden-account = Bloqueada: { $email }
dropdown-bitwarden-email = Correo electrónico
dropdown-bitwarden-email-placeholder = usted@ejemplo.com
dropdown-bitwarden-master-password = Contraseña maestra
dropdown-bitwarden-client-id = client_id de la clave de API
dropdown-bitwarden-client-secret = client_secret de la clave de API
dropdown-bitwarden-required = Obligatorio
dropdown-bitwarden-login = Iniciar sesión
dropdown-bitwarden-login-tooltip = Iniciar sesión en su cuenta de Bitwarden
dropdown-bitwarden-use-api-key = Iniciar sesión con una clave de API
dropdown-bitwarden-use-api-key-tooltip = Iniciar sesión con su clave de API personal, que no necesita segundo factor
dropdown-bitwarden-use-password = Iniciar sesión con la contraseña maestra
dropdown-bitwarden-use-password-tooltip = Iniciar sesión con su correo electrónico y su contraseña maestra
dropdown-bitwarden-two-factor-title = Inicio de sesión en dos pasos
dropdown-bitwarden-two-factor-app = Introduzca el código de su aplicación de autenticación.
dropdown-bitwarden-two-factor-email = Introduzca el código enviado por correo electrónico a { $email }.
dropdown-bitwarden-new-device-title = Dispositivo nuevo
dropdown-bitwarden-new-device-desc = Bitwarden ha enviado un código de verificación a { $email } para confirmar este dispositivo.
dropdown-bitwarden-code-placeholder = Código
dropdown-bitwarden-send-code = Enviar código
dropdown-bitwarden-send-code-tooltip = Enviar el código de verificación a la dirección de correo de su cuenta
dropdown-bitwarden-back = Atrás
dropdown-bitwarden-back-login-tooltip = Volver al inicio de sesión
dropdown-bitwarden-back-vault-tooltip = Volver a la caja fuerte
dropdown-bitwarden-verify = Verificar
dropdown-bitwarden-verify-tooltip = Comprobar el código y terminar de iniciar sesión
dropdown-bitwarden-unlock = Desbloquear
dropdown-bitwarden-unlock-tooltip = Abrir la caja fuerte con su contraseña maestra
dropdown-bitwarden-sync = Sincronizar
dropdown-bitwarden-lock = Bloquear
dropdown-bitwarden-logout = Cerrar sesión
dropdown-bitwarden-search = Buscar en la caja fuerte
dropdown-bitwarden-no-items = No hay elementos
dropdown-bitwarden-more = Mostrando { $shown } de { $total } elementos. Busque para acotarlos.
dropdown-bitwarden-undecryptable = No se ha podido descifrar
dropdown-bitwarden-copy = Copiar
dropdown-bitwarden-copy-reprompt-tooltip = Comprobar la contraseña maestra y copiar el campo protegido
dropdown-bitwarden-cancel = Cancelar
dropdown-bitwarden-cancel-tooltip = No copiar nada y volver a la caja fuerte
dropdown-bitwarden-copy-field = Copiar { $field }
dropdown-bitwarden-copied = Se ha copiado: { $field }.
dropdown-bitwarden-copied-clears = Se ha copiado: { $field }. El portapapeles se vaciará en { $seconds } s.
dropdown-bitwarden-reprompt-title = Volver a pedir la contraseña maestra
dropdown-bitwarden-reprompt-desc = { $item } está protegido: introduzca su contraseña maestra para copiar el campo { $field }.
dropdown-bitwarden-field-username = nombre de usuario
dropdown-bitwarden-field-password = contraseña
dropdown-bitwarden-field-totp = código de verificación
dropdown-bitwarden-field-card-number = número de tarjeta
dropdown-bitwarden-field-card-code = código de seguridad
dropdown-bitwarden-field-email = correo electrónico
dropdown-bitwarden-field-notes = nota
dropdown-bitwarden-field-passphrase = frase de contraseña
dropdown-bitwarden-generator = Generar una contraseña
dropdown-bitwarden-generate = Generar y copiar: { $field }
dropdown-bitwarden-generator-password-caption = { $length } caracteres: { $charsets }
dropdown-bitwarden-generator-passphrase-caption = { $words } palabras
dropdown-bitwarden-generator-empty = No hay preajustes del generador
dropdown-bitwarden-generator-empty-hint = Añádalos en modules.bitwarden.generator-passwords y generator-passphrases.
dropdown-bitwarden-error-wrong-password = Contraseña maestra incorrecta.
dropdown-bitwarden-error-credentials = El correo electrónico, la contraseña maestra o el código son incorrectos.
dropdown-bitwarden-error-login-required = La sesión ha terminado. Vuelva a iniciar sesión.
dropdown-bitwarden-error-unavailable = El servidor no está disponible. Inténtelo de nuevo más tarde.
dropdown-bitwarden-error-server = El servidor ha rechazado la solicitud: { $message }
dropdown-bitwarden-error-network = No se ha podido contactar con el servidor.
launcher-bitwarden-locked = Bitwarden está bloqueado. Escriba su contraseña maestra y pulse Intro.
launcher-bitwarden-logged-out = Inicie sesión en Bitwarden primero desde su menú desplegable en la barra.
launcher-bitwarden-hint = Intro copia la contraseña o el primer campo de un elemento; Mayús+Intro muestra sus campos.
dropdown-bitwarden-totp-live = { $code } · quedan { $seconds } s
dropdown-bitwarden-notify-sync-failed = Bitwarden no ha podido sincronizar
dropdown-bitwarden-notify-logged-out = Bitwarden ha cerrado su sesión
dropdown-bitwarden-notify-logged-out-body = La sesión ha caducado o se ha revocado. Vuelva a iniciar sesión desde la barra.
dropdown-bitwarden-sends = Bitwarden Send
dropdown-bitwarden-send-name = Nombre
dropdown-bitwarden-send-name-placeholder = Lo que está enviando
dropdown-bitwarden-send-text = Texto
dropdown-bitwarden-send-text-placeholder = El texto que abre el enlace
dropdown-bitwarden-send-deletion = Eliminar tras
dropdown-bitwarden-send-max-access = Máximo de aperturas (0 para ilimitadas)
dropdown-bitwarden-send-password = Contraseña
dropdown-bitwarden-send-password-placeholder = Opcional, se pide antes de mostrar el texto
dropdown-bitwarden-send-create = Crear y copiar el enlace
dropdown-bitwarden-send-create-tooltip = Crear el Send y copiar su enlace para compartir
dropdown-bitwarden-send-none = No hay Sends
dropdown-bitwarden-send-delete = Eliminar { $name }
dropdown-bitwarden-send-hours = { $hours ->
    [one] 1 hora
    [many] { $hours } horas
   *[other] { $hours } horas
}
dropdown-bitwarden-send-days = { $days ->
    [one] 1 día
    [many] { $days } días
   *[other] { $days } días
}
dropdown-bitwarden-send-accesses = { $count ->
    [one] abierto una vez
    [many] abierto { $count } veces
   *[other] abierto { $count } veces
}
dropdown-bitwarden-send-accesses-of = abierto { $count } de { $max } veces
dropdown-bitwarden-send-deletes = se elimina el { $date }
dropdown-bitwarden-send-protected = contraseña
dropdown-bitwarden-send-disabled = desactivado
dropdown-bitwarden-field-send-link = enlace del Send
