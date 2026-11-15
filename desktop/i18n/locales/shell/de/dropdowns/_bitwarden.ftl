### Bitwarden Dropdown

dropdown-bitwarden-title = Bitwarden
dropdown-bitwarden-tooltip-logged-out = Bitwarden: Anmelden, um den Tresor zu öffnen
dropdown-bitwarden-tooltip-locked = Bitwarden: Der Tresor ist gesperrt
dropdown-bitwarden-tooltip-unlocked = Bitwarden: Der Tresor ist geöffnet
dropdown-bitwarden-unavailable = Tresor nicht verfügbar
dropdown-bitwarden-server = Server: { $server }
dropdown-bitwarden-account = Gesperrt: { $email }
dropdown-bitwarden-email = E-Mail
dropdown-bitwarden-email-placeholder = sie@example.com
dropdown-bitwarden-master-password = Master-Passwort
dropdown-bitwarden-client-id = API-Schlüssel client_id
dropdown-bitwarden-client-secret = API-Schlüssel client_secret
dropdown-bitwarden-required = Erforderlich
dropdown-bitwarden-login = Anmelden
dropdown-bitwarden-login-tooltip = Beim Bitwarden-Konto anmelden
dropdown-bitwarden-use-api-key = Mit API-Schlüssel anmelden
dropdown-bitwarden-use-api-key-tooltip = Mit dem persönlichen API-Schlüssel anmelden, der keinen zweiten Faktor benötigt
dropdown-bitwarden-use-password = Mit Master-Passwort anmelden
dropdown-bitwarden-use-password-tooltip = Mit E-Mail-Adresse und Master-Passwort anmelden
dropdown-bitwarden-two-factor-title = Zweistufige Anmeldung
dropdown-bitwarden-two-factor-app = Geben Sie den Code aus Ihrer Authentifizierungs-App ein.
dropdown-bitwarden-two-factor-email = Geben Sie den Code ein, der an { $email } gesendet wurde.
dropdown-bitwarden-new-device-title = Neues Gerät
dropdown-bitwarden-new-device-desc = Bitwarden hat einen Bestätigungscode an { $email } gesendet, um dieses Gerät zu bestätigen.
dropdown-bitwarden-code-placeholder = Code
dropdown-bitwarden-send-code = Code senden
dropdown-bitwarden-send-code-tooltip = Den Bestätigungscode an die Adresse des Kontos senden
dropdown-bitwarden-back = Zurück
dropdown-bitwarden-back-login-tooltip = Zurück zur Anmeldung
dropdown-bitwarden-back-vault-tooltip = Zurück zum Tresor
dropdown-bitwarden-verify = Bestätigen
dropdown-bitwarden-verify-tooltip = Code prüfen und Anmeldung abschließen
dropdown-bitwarden-unlock = Entsperren
dropdown-bitwarden-unlock-tooltip = Tresor mit dem Master-Passwort öffnen
dropdown-bitwarden-sync = Synchronisieren
dropdown-bitwarden-lock = Sperren
dropdown-bitwarden-logout = Abmelden
dropdown-bitwarden-search = Tresor durchsuchen
dropdown-bitwarden-no-items = Keine Einträge
dropdown-bitwarden-more = { $shown } von { $total } Einträgen werden angezeigt. Suchen Sie, um sie einzugrenzen.
dropdown-bitwarden-undecryptable = Konnte nicht entschlüsselt werden
dropdown-bitwarden-copy = Kopieren
dropdown-bitwarden-copy-reprompt-tooltip = Master-Passwort prüfen und das geschützte Feld kopieren
dropdown-bitwarden-cancel = Abbrechen
dropdown-bitwarden-cancel-tooltip = Nichts kopieren und zum Tresor zurückkehren
dropdown-bitwarden-copy-field = { $field } kopieren
dropdown-bitwarden-copied = { $field } kopiert.
dropdown-bitwarden-copied-clears = { $field } kopiert. Die Zwischenablage wird in { $seconds } s geleert.
dropdown-bitwarden-reprompt-title = Erneute Abfrage des Master-Passworts
dropdown-bitwarden-reprompt-desc = { $item } ist geschützt: Geben Sie Ihr Master-Passwort ein, um das Feld „{ $field }“ zu kopieren.
dropdown-bitwarden-field-username = Benutzername
dropdown-bitwarden-field-password = Passwort
dropdown-bitwarden-field-totp = Bestätigungscode
dropdown-bitwarden-field-card-number = Kartennummer
dropdown-bitwarden-field-card-code = Sicherheitscode
dropdown-bitwarden-field-email = E-Mail
dropdown-bitwarden-field-notes = Notiz
dropdown-bitwarden-field-passphrase = Passphrase
dropdown-bitwarden-generator = Passwort generieren
dropdown-bitwarden-generate = Neues Feld „{ $field }“ generieren und kopieren
dropdown-bitwarden-generator-password-caption = { $length } Zeichen: { $charsets }
dropdown-bitwarden-generator-passphrase-caption = { $words } Wörter
dropdown-bitwarden-generator-empty = Keine Generator-Vorlagen
dropdown-bitwarden-generator-empty-hint = Fügen Sie sie unter modules.bitwarden.generator-passwords und generator-passphrases hinzu.
dropdown-bitwarden-error-wrong-password = Falsches Master-Passwort.
dropdown-bitwarden-error-credentials = E-Mail, Master-Passwort oder Code ist falsch.
dropdown-bitwarden-error-login-required = Die Sitzung ist beendet. Melden Sie sich erneut an.
dropdown-bitwarden-error-unavailable = Der Server ist nicht verfügbar. Versuchen Sie es später erneut.
dropdown-bitwarden-error-server = Der Server hat abgelehnt: { $message }
dropdown-bitwarden-error-network = Der Server ist nicht erreichbar.
launcher-bitwarden-locked = Bitwarden ist gesperrt. Geben Sie Ihr Master-Passwort ein und drücken Sie Eingabe.
launcher-bitwarden-logged-out = Melden Sie sich zuerst über das Bitwarden-Dropdown in der Leiste an.
launcher-bitwarden-hint = Eingabe kopiert das Passwort oder erste Feld eines Eintrags; Umschalt+Eingabe listet seine Felder auf.
dropdown-bitwarden-totp-live = { $code } · noch { $seconds } s
dropdown-bitwarden-notify-sync-failed = Bitwarden konnte nicht synchronisieren
dropdown-bitwarden-notify-logged-out = Bitwarden hat Sie abgemeldet
dropdown-bitwarden-notify-logged-out-body = Die Sitzung ist abgelaufen oder wurde widerrufen. Melden Sie sich über die Leiste erneut an.
dropdown-bitwarden-sends = Bitwarden Send
dropdown-bitwarden-send-name = Name
dropdown-bitwarden-send-name-placeholder = Was Sie senden
dropdown-bitwarden-send-text = Text
dropdown-bitwarden-send-text-placeholder = Der Text, den der Link öffnet
dropdown-bitwarden-send-deletion = Löschen nach
dropdown-bitwarden-send-max-access = Maximale Aufrufe (0 für unbegrenzt)
dropdown-bitwarden-send-password = Passwort
dropdown-bitwarden-send-password-placeholder = Optional, wird vor dem Anzeigen des Textes abgefragt
dropdown-bitwarden-send-create = Erstellen und Link kopieren
dropdown-bitwarden-send-create-tooltip = Send erstellen und den Freigabelink kopieren
dropdown-bitwarden-send-none = Keine Sends
dropdown-bitwarden-send-delete = { $name } löschen
dropdown-bitwarden-send-hours = { $hours ->
    [one] 1 Stunde
   *[other] { $hours } Stunden
}
dropdown-bitwarden-send-days = { $days ->
    [one] 1 Tag
   *[other] { $days } Tage
}
dropdown-bitwarden-send-accesses = { $count ->
    [one] einmal aufgerufen
   *[other] { $count }-mal aufgerufen
}
dropdown-bitwarden-send-accesses-of = { $count } von { $max } Mal aufgerufen
dropdown-bitwarden-send-deletes = wird gelöscht am { $date }
dropdown-bitwarden-send-protected = Passwort
dropdown-bitwarden-send-disabled = deaktiviert
dropdown-bitwarden-field-send-link = Send-Link
