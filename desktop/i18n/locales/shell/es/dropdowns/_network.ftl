### Network Dropdown

dropdown-network-title = Red
dropdown-network-active-connections = Conexiones activas
dropdown-network-active-connection = Conexión activa
dropdown-network-available = Redes disponibles
dropdown-network-connected = Conectado
dropdown-network-connecting = Conectando
dropdown-network-connect = Conectar
dropdown-network-disconnect = Desconectar
dropdown-network-forget = Olvidar
dropdown-network-dismiss = Descartar
dropdown-network-error = Error
dropdown-network-cancel = Cancelar
dropdown-network-password-placeholder = Introduzca la contraseña
dropdown-network-ethernet = Ethernet
dropdown-network-wifi = Wi-Fi
dropdown-network-no-networks-title = No se han encontrado redes
dropdown-network-no-networks-description = Asegúrese de que la Wi-Fi está activada y vuelva a buscar
dropdown-network-no-adapter-title = No hay adaptador Wi-Fi
dropdown-network-no-adapter-description = No se ha detectado ningún adaptador inalámbrico en este sistema

## Security Types

dropdown-network-security-open = Abierta
dropdown-network-security-wep = WEP
dropdown-network-security-wpa = WPA
dropdown-network-security-wpa2 = WPA2
dropdown-network-security-wpa3 = WPA3
dropdown-network-security-enterprise = Empresarial
dropdown-network-security-saved = { $security } · Guardada
dropdown-network-security-stale = { $security } · No vista recientemente

## Connection Steps

dropdown-network-step-preparing = Preparando...
dropdown-network-step-configuring = Configurando...
dropdown-network-step-authenticating = Autenticando...
dropdown-network-step-obtaining-ip = Obteniendo la dirección IP...
dropdown-network-step-verifying = Verificando la conexión...
dropdown-network-step-searching = Buscando la red...

## Connection Errors

dropdown-network-error-wrong-password = La autenticación ha fallado
dropdown-network-error-timeout = Se ha agotado el tiempo de conexión
dropdown-network-error-ip-config = No se ha podido obtener la dirección IP
dropdown-network-error-not-found = No se ha encontrado la red
dropdown-network-error-generic = La conexión ha fallado

## VPN

dropdown-network-vpn = VPN
dropdown-network-vpn-connected = Conectada
dropdown-network-vpn-connecting = Conectando...
dropdown-network-vpn-disconnected = Desconectada
dropdown-network-vpn-failed = La conexión ha fallado

## Credentials

dropdown-network-secret-title = Se necesita autenticación
dropdown-network-secret-submit = Enviar
dropdown-network-secret-password = Contraseña
dropdown-network-secret-username = Nombre de usuario
dropdown-network-secret-pin = PIN
dropdown-network-secret-group = Grupo
dropdown-network-secret-domain = Dominio
dropdown-network-secret-wep-key = Clave WEP
dropdown-network-secret-private-key = Clave privada
dropdown-network-secret-private-key-password = Contraseña de la clave privada

## VPN configuration

dropdown-network-vpn-add = Añadir VPN
dropdown-network-vpn-edit = Editar
dropdown-network-vpn-new = VPN nueva
dropdown-network-vpn-name = Nombre
dropdown-network-vpn-type = Tipo
dropdown-network-vpn-save = Guardar
dropdown-network-vpn-delete = Eliminar
dropdown-network-vpn-delete-confirm = ¿Eliminar { $name }?
dropdown-network-vpn-delete-confirm-detail = Esto no se puede deshacer. Una clave privada guardada aquí no está guardada en ningún otro sitio.
dropdown-network-vpn-no-native-sign-in = inicia sesión a través del complemento
dropdown-network-vpn-import = Importar un archivo de wg-quick
dropdown-network-vpn-import-filter = Configuración de WireGuard
dropdown-network-vpn-import-failed = Ese archivo no es una configuración de wg-quick
dropdown-network-vpn-generate-key = Generar un par de claves nuevo
dropdown-network-vpn-public-key = Clave pública: { $key }
dropdown-network-vpn-public-key-empty = La clave pública aparece aquí cuando se establece una clave privada. Entréguesela a quien gestione el otro extremo.
dropdown-network-vpn-raw-hint = Una línea clave = valor por cada clave
dropdown-network-vpn-raw-hint-typed = Una línea clave = valor por cada clave, para las claves que este formulario no tiene como campo. Ya cubiertas: { $covered }
dropdown-network-vpn-raw-hint-unknown = Una línea clave = valor por cada clave. Se pasan a { $service } tal cual se escriben: consulte la documentación de ese complemento para ver las claves que acepta.
dropdown-network-vpn-advanced-show = Claves avanzadas
dropdown-network-vpn-advanced-hide = Ocultar claves avanzadas
dropdown-network-vpn-name-required = El nombre es obligatorio
dropdown-network-vpn-field-required = { $field } es obligatorio
dropdown-network-vpn-invalid-text = { $field } no es válido
dropdown-network-vpn-invalid-host = { $field } debe ser un nombre de equipo o una dirección, sin https:// ni ruta
dropdown-network-vpn-invalid-host-port = { $field } debe ser un equipo y un puerto, como vpn.example.com:51820
dropdown-network-vpn-invalid-ip-list = { $field } deben ser direcciones IP separadas por comas
dropdown-network-vpn-invalid-cidr-list = { $field } deben ser direcciones como 10.0.0.2/24, separadas por comas
dropdown-network-vpn-invalid-key = { $field } debe ser una clave de WireGuard: 44 caracteres en base64
dropdown-network-vpn-invalid-number = { $field } debe ser un número entero
dropdown-network-vpn-section-interface = Este equipo
dropdown-network-vpn-section-addressing = Direccionamiento
dropdown-network-vpn-section-peer = Par
dropdown-network-vpn-field-interface = Interfaz
dropdown-network-vpn-field-private-key = Clave privada
dropdown-network-vpn-field-address = Direcciones
dropdown-network-vpn-field-dns = DNS
dropdown-network-vpn-field-peer-public-key = Clave pública del par
dropdown-network-vpn-field-peer-endpoint = Extremo del par
dropdown-network-vpn-field-peer-allowed-ips = IP permitidas
dropdown-network-vpn-field-peer-preshared-key = Clave precompartida
dropdown-network-vpn-field-peer-keepalive = Keepalive (segundos)
dropdown-network-vpn-field-gateway = Pasarela
dropdown-network-vpn-field-protocol = Protocolo
dropdown-network-vpn-field-lys-shell-username = Nombre de usuario
dropdown-network-vpn-field-remote = Servidor
dropdown-network-vpn-field-username = Nombre de usuario
dropdown-network-vpn-field-password = Contraseña
dropdown-network-vpn-field-ca = Certificado de la CA

dropdown-network-vpn-section-gateway = Pasarela
dropdown-network-vpn-section-credentials = Inicio de sesión
dropdown-network-vpn-section-certificates = Certificados
dropdown-network-vpn-section-cipher = Cifrado
dropdown-network-vpn-section-ipsec = IPsec

dropdown-network-vpn-field-cert = Certificado de usuario
dropdown-network-vpn-field-key = Clave privada
dropdown-network-vpn-field-cert-pass = Contraseña de la clave privada
dropdown-network-vpn-field-connection-type = Autenticación
dropdown-network-vpn-field-ipsec-gateway = Pasarela
dropdown-network-vpn-field-ipsec-id = Nombre del grupo
dropdown-network-vpn-field-ipsec-secret = Contraseña del grupo
dropdown-network-vpn-field-xauth-username = Nombre de usuario
dropdown-network-vpn-field-xauth-password = Contraseña
dropdown-network-vpn-field-domain = Dominio
dropdown-network-vpn-field-nat-traversal-mode = Atravesar NAT
dropdown-network-vpn-field-ike-dh-group = Grupo DH de IKE
dropdown-network-vpn-field-perfect-forward-secrecy = Confidencialidad directa
dropdown-network-vpn-field-certificate = Certificado de la pasarela
dropdown-network-vpn-field-method = Autenticación
dropdown-network-vpn-field-user = Nombre de usuario
dropdown-network-vpn-field-usercert = Certificado de usuario
dropdown-network-vpn-field-userkey = Clave privada
dropdown-network-vpn-field-right = Pasarela
dropdown-network-vpn-field-leftid = Identidad local
dropdown-network-vpn-field-ikev2 = Versión de IKE
dropdown-network-vpn-field-leftxauthusername = Nombre de usuario
dropdown-network-vpn-field-xauthpassword = Contraseña
dropdown-network-vpn-field-pskvalue = Clave precompartida
dropdown-network-vpn-field-ipsec-enabled = IPsec
dropdown-network-vpn-field-ipsec-psk = Clave precompartida
dropdown-network-vpn-field-ipsec-gateway-id = ID de la pasarela
dropdown-network-vpn-field-require-mppe = Requerir cifrado
dropdown-network-vpn-field-ca-cert = Certificado de la CA
dropdown-network-vpn-field-trusted-cert = Certificado de confianza
dropdown-network-vpn-field-otp = Código de un solo uso
dropdown-network-vpn-field-realm = Dominio (realm)
dropdown-network-vpn-field-topdomain = Dominio superior
dropdown-network-vpn-field-nameserver = Servidor de nombres
dropdown-network-vpn-field-fragsize = Tamaño de fragmento
dropdown-network-vpn-field-lys-shell-sso = Inicio de sesión en el navegador (SAML)
dropdown-network-vpn-back = Atrás
dropdown-network-save-title = ¿Guardar en Bitwarden?
dropdown-network-save-body = { $name } se ha conectado con lo que ha escrito. Guárdelo en la carpeta de redes de la caja fuerte para conectarse sin preguntar la próxima vez.
dropdown-network-save = Guardar
dropdown-network-save-not-now = Ahora no

## Olvidar una red
dropdown-network-forget-confirm = ¿Olvidar { $name }?
dropdown-network-forget-confirm-detail = Se eliminarán su contraseña y su configuración guardadas. Para volver a conectarse se pedirá la contraseña.
dropdown-network-error-forget = No se ha podido olvidar { $name }: { $error }

## Redes ocultas
dropdown-network-hidden-join = Conectarse a una red oculta
dropdown-network-hidden-title = Red oculta
dropdown-network-hidden-name = Nombre de la red
dropdown-network-hidden-security = Seguridad
dropdown-network-hidden-password = Contraseña
dropdown-network-hidden-error-name-empty = Introduzca el nombre de la red
dropdown-network-hidden-error-name-too-long = Ese nombre es demasiado largo para una red
dropdown-network-hidden-error-password = Introduzca la contraseña de la red
dropdown-network-scan = Buscar redes
dropdown-network-error-wifi-on = No se ha podido activar el Wi-Fi: { $error }
dropdown-network-error-wifi-off = No se ha podido desactivar el Wi-Fi: { $error }
dropdown-network-error-disconnect = No se ha podido desconectar de { $name }: { $error }
dropdown-network-vpn-import-error = No se ha podido abrir un archivo: { $error }
dropdown-network-vpn-error-toggle = No se ha podido cambiar { $name }: { $error }
dropdown-network-vpn-error-read = No se ha podido leer { $name }: { $error }

## Inicio de sesión empresarial

dropdown-network-enterprise-method = Autenticación
dropdown-network-enterprise-phase2 = Autenticación interna
dropdown-network-enterprise-identity = Identidad
dropdown-network-enterprise-anonymous-identity = Identidad anónima
dropdown-network-enterprise-password = Contraseña
dropdown-network-enterprise-client-cert = Certificado de usuario
dropdown-network-enterprise-private-key = Clave privada
dropdown-network-enterprise-private-key-password = Contraseña de la clave privada
dropdown-network-enterprise-ca-cert = Certificado de la CA
dropdown-network-enterprise-domain = Dominio del servidor
dropdown-network-enterprise-optional = Opcional
dropdown-network-enterprise-browse = Elegir un archivo
dropdown-network-enterprise-browse-filter = Certificados y claves
dropdown-network-enterprise-error-identity = Introduzca su identidad
dropdown-network-enterprise-error-password = Introduzca su contraseña
dropdown-network-enterprise-error-server = Indique un certificado de la CA o un dominio del servidor para comprobar el servidor de la red
dropdown-network-enterprise-error-domain = Un dominio del servidor no tiene espacios
dropdown-network-enterprise-error-path = Las rutas de certificados y claves deben ser absolutas y empezar por /
dropdown-network-enterprise-error-client-cert = TLS necesita su certificado de usuario
dropdown-network-enterprise-error-private-key = TLS necesita su clave privada
dropdown-network-enterprise-error-auth = Inicio de sesión rechazado. Compruebe su identidad, contraseña y certificados
dropdown-network-enterprise-error-browse = No se ha podido abrir el selector de archivos: { $error }
