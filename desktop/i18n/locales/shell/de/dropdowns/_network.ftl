### Network Dropdown

dropdown-network-title = Netzwerk
dropdown-network-active-connections = Aktive Verbindungen
dropdown-network-active-connection = Aktive Verbindung
dropdown-network-available = Verfügbare Netzwerke
dropdown-network-connected = Verbunden
dropdown-network-connecting = Verbindung wird hergestellt
dropdown-network-connect = Verbinden
dropdown-network-disconnect = Trennen
dropdown-network-forget = Vergessen
dropdown-network-dismiss = Verwerfen
dropdown-network-error = Fehler
dropdown-network-cancel = Abbrechen
dropdown-network-password-placeholder = Passwort eingeben
dropdown-network-ethernet = Ethernet
dropdown-network-wifi = WLAN
dropdown-network-no-networks-title = Keine Netzwerke gefunden
dropdown-network-no-networks-description = Stellen Sie sicher, dass WLAN aktiviert ist, und suchen Sie erneut
dropdown-network-no-adapter-title = Kein WLAN-Adapter
dropdown-network-no-adapter-description = Auf diesem System wurde kein Funkadapter erkannt

## Security Types

dropdown-network-security-open = Offen
dropdown-network-security-wep = WEP
dropdown-network-security-wpa = WPA
dropdown-network-security-wpa2 = WPA2
dropdown-network-security-wpa3 = WPA3
dropdown-network-security-enterprise = Enterprise
dropdown-network-security-saved = { $security } · Gespeichert
dropdown-network-security-stale = { $security } · Länger nicht gesehen

## Connection Steps

dropdown-network-step-preparing = Wird vorbereitet …
dropdown-network-step-configuring = Wird konfiguriert …
dropdown-network-step-authenticating = Wird authentifiziert …
dropdown-network-step-obtaining-ip = IP-Adresse wird bezogen …
dropdown-network-step-verifying = Verbindung wird geprüft …
dropdown-network-step-searching = Netzwerk wird gesucht …

## Connection Errors

dropdown-network-error-wrong-password = Authentifizierung fehlgeschlagen
dropdown-network-error-timeout = Zeitüberschreitung der Verbindung
dropdown-network-error-ip-config = IP-Adresse konnte nicht bezogen werden
dropdown-network-error-not-found = Netzwerk nicht gefunden
dropdown-network-error-generic = Verbindung fehlgeschlagen

## VPN

dropdown-network-vpn = VPN
dropdown-network-vpn-connected = Verbunden
dropdown-network-vpn-connecting = Verbindung wird hergestellt …
dropdown-network-vpn-disconnected = Getrennt
dropdown-network-vpn-failed = Verbindung fehlgeschlagen

## Credentials

dropdown-network-secret-title = Authentifizierung erforderlich
dropdown-network-secret-submit = Senden
dropdown-network-secret-password = Passwort
dropdown-network-secret-username = Benutzername
dropdown-network-secret-pin = PIN
dropdown-network-secret-group = Gruppe
dropdown-network-secret-domain = Domäne
dropdown-network-secret-wep-key = WEP-Schlüssel
dropdown-network-secret-private-key = Privater Schlüssel
dropdown-network-secret-private-key-password = Passwort des privaten Schlüssels

## VPN configuration

dropdown-network-vpn-add = VPN hinzufügen
dropdown-network-vpn-edit = Bearbeiten
dropdown-network-vpn-new = Neues VPN
dropdown-network-vpn-name = Name
dropdown-network-vpn-type = Typ
dropdown-network-vpn-save = Speichern
dropdown-network-vpn-delete = Löschen
dropdown-network-vpn-delete-confirm = { $name } löschen?
dropdown-network-vpn-delete-confirm-detail = Dies kann nicht rückgängig gemacht werden. Ein hier gespeicherter privater Schlüssel ist nirgendwo sonst gesichert.
dropdown-network-vpn-no-native-sign-in = meldet sich über das Plugin an
dropdown-network-vpn-import = wg-quick-Datei importieren
dropdown-network-vpn-import-filter = WireGuard-Konfiguration
dropdown-network-vpn-import-failed = Diese Datei ist keine wg-quick-Konfiguration
dropdown-network-vpn-generate-key = Neues Schlüsselpaar generieren
dropdown-network-vpn-public-key = Öffentlicher Schlüssel: { $key }
dropdown-network-vpn-public-key-empty = Der öffentliche Schlüssel erscheint hier, sobald ein privater Schlüssel gesetzt ist. Geben Sie ihn an die Person weiter, die die Gegenseite betreibt.
dropdown-network-vpn-raw-hint = Ein Schlüssel = Wert pro Zeile
dropdown-network-vpn-raw-hint-typed = Ein Schlüssel = Wert pro Zeile, für Schlüssel, für die dieses Formular kein Feld hat. Bereits abgedeckt: { $covered }
dropdown-network-vpn-raw-hint-unknown = Ein Schlüssel = Wert pro Zeile. Diese gehen genau so an { $service } – die akzeptierten Schlüssel stehen in der Dokumentation dieses Plugins.
dropdown-network-vpn-advanced-show = Erweiterte Schlüssel
dropdown-network-vpn-advanced-hide = Erweiterte Schlüssel ausblenden
dropdown-network-vpn-name-required = Ein Name ist erforderlich
dropdown-network-vpn-field-required = { $field } ist erforderlich
dropdown-network-vpn-invalid-text = { $field } ist ungültig
dropdown-network-vpn-invalid-host = { $field } muss ein Hostname oder eine Adresse sein, ohne https:// und ohne Pfad
dropdown-network-vpn-invalid-host-port = { $field } muss ein Host mit Port sein, etwa vpn.example.com:51820
dropdown-network-vpn-invalid-ip-list = { $field } muss aus IP-Adressen bestehen, durch Kommas getrennt
dropdown-network-vpn-invalid-cidr-list = { $field } muss aus Adressen wie 10.0.0.2/24 bestehen, durch Kommas getrennt
dropdown-network-vpn-invalid-key = { $field } muss ein WireGuard-Schlüssel sein: 44 Zeichen Base64
dropdown-network-vpn-invalid-number = { $field } muss eine ganze Zahl sein
dropdown-network-vpn-section-interface = Dieser Rechner
dropdown-network-vpn-section-addressing = Adressierung
dropdown-network-vpn-section-peer = Gegenstelle
dropdown-network-vpn-field-interface = Schnittstelle
dropdown-network-vpn-field-private-key = Privater Schlüssel
dropdown-network-vpn-field-address = Adressen
dropdown-network-vpn-field-dns = DNS
dropdown-network-vpn-field-peer-public-key = Öffentlicher Schlüssel der Gegenstelle
dropdown-network-vpn-field-peer-endpoint = Endpunkt der Gegenstelle
dropdown-network-vpn-field-peer-allowed-ips = Erlaubte IPs
dropdown-network-vpn-field-peer-preshared-key = Pre-Shared Key
dropdown-network-vpn-field-peer-keepalive = Keepalive (Sekunden)
dropdown-network-vpn-field-gateway = Gateway
dropdown-network-vpn-field-protocol = Protokoll
dropdown-network-vpn-field-lys-shell-username = Benutzername
dropdown-network-vpn-field-remote = Server
dropdown-network-vpn-field-username = Benutzername
dropdown-network-vpn-field-password = Passwort
dropdown-network-vpn-field-ca = CA-Zertifikat

dropdown-network-vpn-section-gateway = Gateway
dropdown-network-vpn-section-credentials = Anmeldung
dropdown-network-vpn-section-certificates = Zertifikate
dropdown-network-vpn-section-cipher = Verschlüsselung
dropdown-network-vpn-section-ipsec = IPsec

dropdown-network-vpn-field-cert = Benutzerzertifikat
dropdown-network-vpn-field-key = Privater Schlüssel
dropdown-network-vpn-field-cert-pass = Passwort des privaten Schlüssels
dropdown-network-vpn-field-connection-type = Authentifizierung
dropdown-network-vpn-field-ipsec-gateway = Gateway
dropdown-network-vpn-field-ipsec-id = Gruppenname
dropdown-network-vpn-field-ipsec-secret = Gruppenpasswort
dropdown-network-vpn-field-xauth-username = Benutzername
dropdown-network-vpn-field-xauth-password = Passwort
dropdown-network-vpn-field-domain = Domäne
dropdown-network-vpn-field-nat-traversal-mode = NAT-Traversal
dropdown-network-vpn-field-ike-dh-group = IKE-DH-Gruppe
dropdown-network-vpn-field-perfect-forward-secrecy = Forward Secrecy
dropdown-network-vpn-field-certificate = Gateway-Zertifikat
dropdown-network-vpn-field-method = Authentifizierung
dropdown-network-vpn-field-user = Benutzername
dropdown-network-vpn-field-usercert = Benutzerzertifikat
dropdown-network-vpn-field-userkey = Privater Schlüssel
dropdown-network-vpn-field-right = Gateway
dropdown-network-vpn-field-leftid = Lokale Identität
dropdown-network-vpn-field-ikev2 = IKE-Version
dropdown-network-vpn-field-leftxauthusername = Benutzername
dropdown-network-vpn-field-xauthpassword = Passwort
dropdown-network-vpn-field-pskvalue = Pre-Shared Key
dropdown-network-vpn-field-ipsec-enabled = IPsec
dropdown-network-vpn-field-ipsec-psk = Pre-Shared Key
dropdown-network-vpn-field-ipsec-gateway-id = Gateway-ID
dropdown-network-vpn-field-require-mppe = Verschlüsselung erzwingen
dropdown-network-vpn-field-ca-cert = CA-Zertifikat
dropdown-network-vpn-field-trusted-cert = Vertrauenswürdiges Zertifikat
dropdown-network-vpn-field-otp = Einmalcode
dropdown-network-vpn-field-realm = Realm
dropdown-network-vpn-field-topdomain = Top-Level-Domain
dropdown-network-vpn-field-nameserver = Nameserver
dropdown-network-vpn-field-fragsize = Fragmentgröße
dropdown-network-vpn-field-lys-shell-sso = Browser-Anmeldung (SAML)
dropdown-network-vpn-back = Zurück
dropdown-network-save-title = In Bitwarden speichern?
dropdown-network-save-body = { $name } wurde mit Ihren Eingaben verbunden. Speichern Sie sie im Netzwerkordner des Tresors, um sich beim nächsten Mal ohne Nachfrage zu verbinden.
dropdown-network-save = Speichern
dropdown-network-save-not-now = Nicht jetzt
