### lys-shell Configuration - Login Screen

## Login screen: the greeter and the lock screen

settings-login-background-mode = Hintergrund
    .description = Wie der Hintergrund des Anmeldebildschirms gezeichnet wird: Volltonfarbe, ein Bild oder das Hintergrundbild

settings-login-background-image = Hintergrundbild
    .description = Bild hinter dem Anmeldebildschirm, wenn der Hintergrund Bild ist

settings-login-background-color = Hintergrundfarbe
    .description = Füllfarbe hinter dem Anmeldebildschirm, wenn der Hintergrund Farbe ist

settings-login-blur = Unschärfe
    .description = Wie stark ein Bild- oder Hintergrundbild-Hintergrund verschwimmt; 0 lässt ihn scharf

settings-login-show-clock = Uhr anzeigen
    .description = Eine Uhr über der Anmeldeabfrage anzeigen

settings-login-clock-format = Uhrzeitformat
    .description = Wie die Uhrzeit auf dem Anmeldebildschirm aussieht

settings-login-date-format = Datumsformat
    .description = Wie das Datum auf dem Anmeldebildschirm aussieht

settings-login-max-attempts = Maximale Versuche
    .description = Falsche Passwörter auf dem Sperrbildschirm, bevor die Eingabe blockiert wird, der Bildschirm bleibt gesperrt; 0 setzt keine Grenze

settings-login-show-failed-attempts = Fehlversuche anzeigen
    .description = Die Anzahl der Fehlversuche auf dem Sperrbildschirm anzeigen

settings-login-lock-enabled = Aktiviert
    .description = lys-shell die Sitzung sperren lassen (reagiert auf loginctl lock-session und lys-shell lock)

settings-login-lock-lock-on-start = Beim Start sperren
    .description = Die Sitzung sperren, sobald die Shell startet, für automatische Anmeldungen, bei denen der Sperrbildschirm die Zugangskontrolle ist

settings-login-lock-grace-period-ms = Kulanzzeit
    .description = Wie lange nach dem Sperren der Bildschirm noch ohne Passwort entsperrt

settings-login-lock-blank-timeout-ms = Schwärzen nach
    .description = Den Sperrbildschirm schwärzen, nachdem er so lange unbenutzt ist; wann sich die Bildschirme abschalten, steht auf der Seite Leerlauf & Energie

settings-login-lock-pam-service = PAM-Dienst
    .description = PAM-Dienst, der das Passwort prüft, etwa system-auth oder login

settings-login-lock-show-media = Aktuelle Wiedergabe anzeigen
    .description = Den laufenden Titel mit Wiedergabe/Pause und Weiter auf dem Sperrbildschirm anzeigen (nur lokale Cover)

settings-login-lock-show-battery = Akku anzeigen
    .description = Den Akkustand auf dem Sperrbildschirm anzeigen; entfällt ohne Akku

settings-login-lock-fingerprint = Fingerabdruck
    .description = Mit einem registrierten Fingerabdruck (fprintd) zusätzlich zum Passwort entsperren

settings-login-lock-show-switch-user = Benutzerwechsel anzeigen
    .description = „Benutzer wechseln“ auf dem Sperrbildschirm anbieten, wenn der Rechner mehr als ein Konto hat

settings-login-greeter-show-user-list = Benutzerliste anzeigen
    .description = Anklickbare Avatare der Anmeldebenutzer des Rechners über dem Benutzernamenfeld anzeigen

settings-login-greeter-show-power-buttons = Energieknöpfe anzeigen
    .description = Die Knöpfe zum Herunterfahren/Neustarten unten auf dem Anmeldebildschirm anzeigen

settings-login-greeter-fingerprint = Anmeldung per Fingerabdruck
    .description = Mit einem registrierten Fingerabdruck (fprintd) zusätzlich zum Passwort anmelden

settings-login-greeter-match-cursor = Meinen Zeiger übernehmen
    .description = Den Zeiger der zuletzt angemeldeten Sitzung zeigen statt des hier gesetzten Themas und der Größe

settings-login-greeter-cursor-theme = Zeigerthema
    .description = Zeigerthema auf dem Anmeldebildschirm; nicht gesetzt, das des Systems

settings-login-greeter-cursor-size = Zeigergröße
    .description = Logische Zeigergröße; pro Bildschirm skaliert, damit HiDPI-Ausgaben einen scharfen Zeiger erhalten

settings-login-greeter-screen-reader = Bildschirmleser
    .description = Bildschirmleser, den das Barrierefreiheitsmenü des Anmeldebildschirms und Super+Alt+S starten; ausgeblendet, wenn nicht installiert

## LockBackground variants
enum-lock-background-color = Farbe
enum-lock-background-image = Bild
enum-lock-background-wallpaper = Hintergrundbild
