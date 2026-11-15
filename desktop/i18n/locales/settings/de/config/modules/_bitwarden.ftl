### lys-shell Configuration - Bitwarden Module

## Bitwarden Module Configuration

settings-modules-bitwarden-server = Server
    .description = bitwarden.com, bitwarden.eu oder die Basis-URL eines selbst gehosteten Servers. Ein angemeldetes Konto behält den Server, bei dem es sich angemeldet hat

settings-modules-bitwarden-email = E-Mail
    .description = E-Mail-Adresse, mit der das Anmeldeformular beginnt

settings-modules-bitwarden-lock-with-session = Mit der Sitzung sperren
    .description = Den Tresor sperren, wenn die Sitzung gesperrt wird

settings-modules-bitwarden-unlock-with-session = Mit der Sitzung entsperren
    .description = Einen gesperrten Tresor mit dem Passwort des Sperrbildschirms entsperren, wenn es das Master-Passwort ist

settings-modules-bitwarden-clipboard-clear = Zwischenablage leeren nach (Sekunden)
    .description = Ein kopiertes Geheimnis nach so vielen Sekunden löschen, sofern nichts anderes kopiert wurde. 0 lässt es stehen

settings-modules-bitwarden-network-folder = Netzwerkordner
    .description = Tresorordner, dessen nach der Verbindung benannte Zugangsdaten WLAN- und VPN-Passwortanfragen beantworten, solange der Tresor geöffnet ist. Leer beantwortet keine

settings-modules-bitwarden-portal-folder = Ordner für Portal-Geheimnisse
    .description = Tresorordner, in dem die Schlüsselbund-Schlüssel isolierter Apps abgelegt werden, nachdem Sie jede einzeln erlaubt haben. Leer behält sie nur im Schlüsselbund

settings-modules-bitwarden-generator-passwords = Passwortvorlagen
    .description = Passwörter aus Zufallszeichen, die der Generator im Dropdown kopiert

settings-modules-bitwarden-generator-passphrases = Passphrase-Vorlagen
    .description = Passphrasen aus Wörtern der großen EFF-Wortliste, die der Generator im Dropdown kopiert

enum-password-charset-lowercase = Kleinbuchstaben (a-z)
enum-password-charset-uppercase = Großbuchstaben (A-Z)
enum-password-charset-digits = Ziffern (0-9)
enum-password-charset-symbols = Sonderzeichen (!@#$%^&*)

settings-modules-bitwarden-format = Format
    .description = Formatzeichenkette der Beschriftung. Platzhalter: {"{{ count }}"} (Einträge im geöffneten Tresor)

settings-modules-bitwarden-icon-locked = Symbol für gesperrt
    .description = Symbol, solange der Tresor gesperrt oder abgemeldet ist

settings-modules-bitwarden-icon-unlocked = Symbol für entsperrt
    .description = Symbol, solange der Tresor geöffnet ist

settings-modules-bitwarden-border-show = Rahmen anzeigen
    .description = Rahmen um die Schaltfläche anzeigen

settings-modules-bitwarden-border-color = Rahmenfarbe
    .description = Farbtoken des Rahmens

settings-modules-bitwarden-icon-show = Symbol anzeigen
    .description = Modulsymbol anzeigen

settings-modules-bitwarden-tooltip = Kurzinfo anzeigen
    .description = Den Zustand des Moduls beim Überfahren in einer Kurzinfo anzeigen

settings-modules-bitwarden-icon-color = Symbolfarbe
    .description = Vordergrundfarbe des Symbols

settings-modules-bitwarden-icon-bg-color = Symbolhintergrund
    .description = Hintergrundfarbe des Symbolcontainers

settings-modules-bitwarden-label-show = Beschriftung anzeigen
    .description = Beschriftung anzeigen

settings-modules-bitwarden-label-color = Beschriftungsfarbe
    .description = Textfarbe der Beschriftung

settings-modules-bitwarden-label-max-length = Maximale Beschriftungslänge
    .description = Maximale Zeichenanzahl vor dem Kürzen

settings-modules-bitwarden-button-bg-color = Schaltflächenhintergrund
    .description = Hintergrundfarbe der Schaltfläche

settings-modules-bitwarden-left-click = Linksklick
    .description = Aktion bei Linksklick. Öffnet standardmäßig das Tresor-Dropdown

settings-modules-bitwarden-right-click = Rechtsklick
    .description = Shell-Befehl bei Rechtsklick

settings-modules-bitwarden-middle-click = Mittelklick
    .description = Shell-Befehl bei Mittelklick

settings-modules-bitwarden-scroll-up = Nach oben scrollen
    .description = Shell-Befehl beim Scrollen nach oben

settings-modules-bitwarden-scroll-down = Nach unten scrollen
    .description = Shell-Befehl beim Scrollen nach unten

## Format placeholders

settings-modules-bitwarden-format-placeholder-count = Anzahl der Einträge im geöffneten Tresor (leer, solange gesperrt)
