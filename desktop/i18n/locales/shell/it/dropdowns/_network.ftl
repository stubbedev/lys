### Network Dropdown

dropdown-network-title = Rete
dropdown-network-active-connections = Connessioni attive
dropdown-network-active-connection = Connessione attiva
dropdown-network-available = Reti disponibili
dropdown-network-connected = Connesso
dropdown-network-connecting = Connessione in corso
dropdown-network-connect = Connetti
dropdown-network-disconnect = Disconnetti
dropdown-network-forget = Dimentica
dropdown-network-dismiss = Ignora
dropdown-network-error = Errore
dropdown-network-cancel = Annulla
dropdown-network-password-placeholder = Inserisci la password
dropdown-network-ethernet = Ethernet
dropdown-network-wifi = Wi-Fi
dropdown-network-no-networks-title = Nessuna rete trovata
dropdown-network-no-networks-description = Assicurati che il Wi-Fi sia attivo e riprova la ricerca
dropdown-network-no-adapter-title = Nessun adattatore Wi-Fi
dropdown-network-no-adapter-description = Non è stato rilevato alcun adattatore wireless su questo sistema

## Security Types

dropdown-network-security-open = Aperta
dropdown-network-security-wep = WEP
dropdown-network-security-wpa = WPA
dropdown-network-security-wpa2 = WPA2
dropdown-network-security-wpa3 = WPA3
dropdown-network-security-enterprise = Enterprise
dropdown-network-security-saved = { $security } · Salvata
dropdown-network-security-stale = { $security } · Non vista di recente

## Connection Steps

dropdown-network-step-preparing = Preparazione...
dropdown-network-step-configuring = Configurazione...
dropdown-network-step-authenticating = Autenticazione...
dropdown-network-step-obtaining-ip = Acquisizione dell'indirizzo IP...
dropdown-network-step-verifying = Verifica della connessione...
dropdown-network-step-searching = Ricerca della rete...

## Connection Errors

dropdown-network-error-wrong-password = Autenticazione non riuscita
dropdown-network-error-timeout = Connessione scaduta
dropdown-network-error-ip-config = Impossibile ottenere l'indirizzo IP
dropdown-network-error-not-found = Rete non trovata
dropdown-network-error-generic = Connessione non riuscita

## VPN

dropdown-network-vpn = VPN
dropdown-network-vpn-connected = Connessa
dropdown-network-vpn-connecting = Connessione in corso...
dropdown-network-vpn-disconnected = Disconnessa
dropdown-network-vpn-failed = Connessione non riuscita

## Credentials

dropdown-network-secret-title = Autenticazione richiesta
dropdown-network-secret-submit = Invia
dropdown-network-secret-password = Password
dropdown-network-secret-username = Nome utente
dropdown-network-secret-pin = PIN
dropdown-network-secret-group = Gruppo
dropdown-network-secret-domain = Dominio
dropdown-network-secret-wep-key = Chiave WEP
dropdown-network-secret-private-key = Chiave privata
dropdown-network-secret-private-key-password = Password della chiave privata

## VPN configuration

dropdown-network-vpn-add = Aggiungi VPN
dropdown-network-vpn-edit = Modifica
dropdown-network-vpn-new = Nuova VPN
dropdown-network-vpn-name = Nome
dropdown-network-vpn-type = Tipo
dropdown-network-vpn-save = Salva
dropdown-network-vpn-delete = Elimina
dropdown-network-vpn-delete-confirm = Eliminare { $name }?
dropdown-network-vpn-delete-confirm-detail = L'operazione non può essere annullata. Una chiave privata memorizzata qui non è salvata altrove.
dropdown-network-vpn-no-native-sign-in = accede tramite il plugin
dropdown-network-vpn-import = Importa un file wg-quick
dropdown-network-vpn-import-filter = Configurazione WireGuard
dropdown-network-vpn-import-failed = Quel file non è una configurazione wg-quick
dropdown-network-vpn-generate-key = Genera una nuova coppia di chiavi
dropdown-network-vpn-public-key = Chiave pubblica: { $key }
dropdown-network-vpn-public-key-empty = La chiave pubblica compare qui dopo aver impostato una chiave privata. Consegnala a chi gestisce l'altro capo.
dropdown-network-vpn-raw-hint = Una coppia chiave = valore per riga
dropdown-network-vpn-raw-hint-typed = Una coppia chiave = valore per riga, per le chiavi che questo modulo non prevede. Già coperte: { $covered }
dropdown-network-vpn-raw-hint-unknown = Una coppia chiave = valore per riga. Vengono passate a { $service } esattamente come scritte: consulta la documentazione del plugin per le chiavi che accetta.
dropdown-network-vpn-advanced-show = Chiavi avanzate
dropdown-network-vpn-advanced-hide = Nascondi le chiavi avanzate
dropdown-network-vpn-name-required = Il nome è obbligatorio
dropdown-network-vpn-field-required = { $field } è obbligatorio
dropdown-network-vpn-invalid-text = { $field } non è valido
dropdown-network-vpn-invalid-host = { $field } deve essere un nome host o un indirizzo, senza https:// e senza percorso
dropdown-network-vpn-invalid-host-port = { $field } deve essere un host e una porta, come vpn.example.com:51820
dropdown-network-vpn-invalid-ip-list = { $field } deve contenere indirizzi IP separati da virgole
dropdown-network-vpn-invalid-cidr-list = { $field } deve contenere indirizzi come 10.0.0.2/24, separati da virgole
dropdown-network-vpn-invalid-key = { $field } deve essere una chiave WireGuard: 44 caratteri in base64
dropdown-network-vpn-invalid-number = { $field } deve essere un numero intero
dropdown-network-vpn-section-interface = Questa macchina
dropdown-network-vpn-section-addressing = Indirizzamento
dropdown-network-vpn-section-peer = Peer
dropdown-network-vpn-field-interface = Interfaccia
dropdown-network-vpn-field-private-key = Chiave privata
dropdown-network-vpn-field-address = Indirizzi
dropdown-network-vpn-field-dns = DNS
dropdown-network-vpn-field-peer-public-key = Chiave pubblica del peer
dropdown-network-vpn-field-peer-endpoint = Endpoint del peer
dropdown-network-vpn-field-peer-allowed-ips = IP consentiti
dropdown-network-vpn-field-peer-preshared-key = Chiave pre-condivisa
dropdown-network-vpn-field-peer-keepalive = Keepalive (secondi)
dropdown-network-vpn-field-gateway = Gateway
dropdown-network-vpn-field-protocol = Protocollo
dropdown-network-vpn-field-lys-shell-username = Nome utente
dropdown-network-vpn-field-remote = Server
dropdown-network-vpn-field-username = Nome utente
dropdown-network-vpn-field-password = Password
dropdown-network-vpn-field-ca = Certificato CA

dropdown-network-vpn-section-gateway = Gateway
dropdown-network-vpn-section-credentials = Accesso
dropdown-network-vpn-section-certificates = Certificati
dropdown-network-vpn-section-cipher = Cifratura
dropdown-network-vpn-section-ipsec = IPsec

dropdown-network-vpn-field-cert = Certificato utente
dropdown-network-vpn-field-key = Chiave privata
dropdown-network-vpn-field-cert-pass = Password della chiave privata
dropdown-network-vpn-field-connection-type = Autenticazione
dropdown-network-vpn-field-ipsec-gateway = Gateway
dropdown-network-vpn-field-ipsec-id = Nome del gruppo
dropdown-network-vpn-field-ipsec-secret = Password del gruppo
dropdown-network-vpn-field-xauth-username = Nome utente
dropdown-network-vpn-field-xauth-password = Password
dropdown-network-vpn-field-domain = Dominio
dropdown-network-vpn-field-nat-traversal-mode = Attraversamento NAT
dropdown-network-vpn-field-ike-dh-group = Gruppo DH IKE
dropdown-network-vpn-field-perfect-forward-secrecy = Forward secrecy
dropdown-network-vpn-field-certificate = Certificato del gateway
dropdown-network-vpn-field-method = Autenticazione
dropdown-network-vpn-field-user = Nome utente
dropdown-network-vpn-field-usercert = Certificato utente
dropdown-network-vpn-field-userkey = Chiave privata
dropdown-network-vpn-field-right = Gateway
dropdown-network-vpn-field-leftid = Identità locale
dropdown-network-vpn-field-ikev2 = Versione IKE
dropdown-network-vpn-field-leftxauthusername = Nome utente
dropdown-network-vpn-field-xauthpassword = Password
dropdown-network-vpn-field-pskvalue = Chiave pre-condivisa
dropdown-network-vpn-field-ipsec-enabled = IPsec
dropdown-network-vpn-field-ipsec-psk = Chiave pre-condivisa
dropdown-network-vpn-field-ipsec-gateway-id = ID del gateway
dropdown-network-vpn-field-require-mppe = Richiedi la cifratura
dropdown-network-vpn-field-ca-cert = Certificato CA
dropdown-network-vpn-field-trusted-cert = Certificato attendibile
dropdown-network-vpn-field-otp = Codice monouso
dropdown-network-vpn-field-realm = Realm
dropdown-network-vpn-field-topdomain = Dominio principale
dropdown-network-vpn-field-nameserver = Server dei nomi
dropdown-network-vpn-field-fragsize = Dimensione dei frammenti
dropdown-network-vpn-field-lys-shell-sso = Accesso tramite browser (SAML)
dropdown-network-vpn-back = Indietro
dropdown-network-save-title = Salvare in Bitwarden?
dropdown-network-save-body = { $name } si è connessa con i dati inseriti. Conservali nella cartella di rete della cassaforte per connetterti senza richieste la prossima volta.
dropdown-network-save = Salva
dropdown-network-save-not-now = Non ora

## Dimenticare una rete
dropdown-network-forget-confirm = Dimenticare { $name }?
dropdown-network-forget-confirm-detail = La password e le impostazioni salvate verranno eliminate. Per riconnettersi verrà chiesta la password.
dropdown-network-error-forget = Impossibile dimenticare { $name }: { $error }

## Reti nascoste
dropdown-network-hidden-join = Connetti a una rete nascosta
dropdown-network-hidden-title = Rete nascosta
dropdown-network-hidden-name = Nome della rete
dropdown-network-hidden-security = Sicurezza
dropdown-network-hidden-password = Password
dropdown-network-hidden-error-name-empty = Inserisci il nome della rete
dropdown-network-hidden-error-name-too-long = Questo nome è troppo lungo per una rete
dropdown-network-hidden-error-password = Inserisci la password della rete
dropdown-network-scan = Cerca reti
dropdown-network-error-wifi-on = Impossibile attivare il Wi-Fi: { $error }
dropdown-network-error-wifi-off = Impossibile disattivare il Wi-Fi: { $error }
dropdown-network-error-disconnect = Impossibile disconnettersi da { $name }: { $error }
dropdown-network-vpn-import-error = Impossibile aprire un file
dropdown-network-vpn-error-toggle = Impossibile commutare { $name }: { $error }
dropdown-network-vpn-error-read = Impossibile leggere { $name }: { $error }

## Accesso aziendale

dropdown-network-enterprise-method = Autenticazione
dropdown-network-enterprise-phase2 = Autenticazione interna
dropdown-network-enterprise-identity = Identità
dropdown-network-enterprise-anonymous-identity = Identità anonima
dropdown-network-enterprise-password = Password
dropdown-network-enterprise-client-cert = Certificato utente
dropdown-network-enterprise-private-key = Chiave privata
dropdown-network-enterprise-private-key-password = Password della chiave privata
dropdown-network-enterprise-ca-cert = Certificato CA
dropdown-network-enterprise-domain = Dominio del server
dropdown-network-enterprise-optional = Facoltativo
dropdown-network-enterprise-browse = Scegli un file
dropdown-network-enterprise-browse-filter = Certificati e chiavi
dropdown-network-enterprise-error-identity = Inserisci la tua identità
dropdown-network-enterprise-error-password = Inserisci la tua password
dropdown-network-enterprise-error-server = Indica un certificato CA o un dominio del server per verificare il server della rete
dropdown-network-enterprise-error-domain = Un dominio del server non contiene spazi
dropdown-network-enterprise-error-path = I percorsi di certificati e chiavi devono essere assoluti e iniziare con /
dropdown-network-enterprise-error-client-cert = TLS richiede il tuo certificato utente
dropdown-network-enterprise-error-private-key = TLS richiede la tua chiave privata
dropdown-network-enterprise-error-auth = Accesso rifiutato. Controlla identità, password e certificati
dropdown-network-enterprise-error-browse = Impossibile aprire il selettore di file: { $error }
