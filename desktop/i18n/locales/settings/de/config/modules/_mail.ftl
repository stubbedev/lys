### lys-shell Configuration - Mail Module

## Mail Module Configuration

settings-modules-mail-format = Format
    .description = Was die Beschriftung zeigt, zusammengesetzt aus den Werten des Elements und eigenem Text

settings-modules-mail-query = Abfrage
    .description = notmuch-Suchabfrage, deren Trefferzahl angezeigt wird (z. B. tag:unread)

settings-modules-mail-hide-when-zero = Bei null ausblenden
    .description = Das Modul ganz ausblenden, solange die Anzahl null ist

settings-modules-mail-accounts = Konten
    .description = Aufschlüsselung der Ungelesenen pro Konto im Dropdown; die Zahl in der Leiste ist ihre Summe

enum-mail-provider-generic = Allgemein
enum-mail-provider-gmail = Gmail
enum-mail-provider-outlook = Outlook
enum-mail-provider-icloud = iCloud
enum-mail-provider-proton = Proton
enum-mail-provider-fastmail = Fastmail
enum-mail-provider-yahoo = Yahoo

settings-modules-mail-notify = Bei neuer E-Mail benachrichtigen
    .description = Für jede neue Nachricht eine Desktop-Benachrichtigung auslösen (Absender, Betreff, Anbietersymbol)

settings-modules-mail-notify-summary = Benachrichtigungstitel
    .description = Der Titel der Benachrichtigung, zusammengesetzt aus Absender, Betreff und Anzahl der neuen E-Mails und eigenem Text

settings-modules-mail-notify-body = Benachrichtigungstext
    .description = Der Text der Benachrichtigung, zusammengesetzt aus Absender, Betreff und Anzahl der neuen E-Mails und eigenem Text

settings-modules-mail-notify-click = Befehl beim Klick auf die Benachrichtigung
    .description = Befehl, der beim Klick auf eine Benachrichtigung über neue E-Mails ausgeführt wird; er kann den Thread der E-Mail erhalten

settings-modules-mail-icon-name = Symbol
    .description = Modulsymbol

settings-modules-mail-border-show = Rahmen anzeigen
    .description = Rahmen um die Schaltfläche anzeigen

settings-modules-mail-border-color = Rahmenfarbe
    .description = Farbtoken des Rahmens

settings-modules-mail-icon-show = Symbol anzeigen
    .description = Modulsymbol anzeigen

settings-modules-mail-tooltip = Kurzinfo anzeigen
    .description = Den Zustand des Moduls beim Überfahren in einer Kurzinfo anzeigen

settings-modules-mail-icon-color = Symbolfarbe
    .description = Vordergrundfarbe des Symbols

settings-modules-mail-icon-bg-color = Symbolhintergrund
    .description = Hintergrundfarbe des Symbolcontainers

settings-modules-mail-label-show = Beschriftung anzeigen
    .description = Beschriftung anzeigen

settings-modules-mail-label-color = Beschriftungsfarbe
    .description = Textfarbe der Beschriftung

settings-modules-mail-label-max-length = Maximale Beschriftungslänge
    .description = Zeichen, die die Beschriftung zeigt, bevor sie gekürzt wird; 0 kürzt nie

settings-modules-mail-button-bg-color = Schaltflächenhintergrund
    .description = Hintergrundfarbe der Schaltfläche

settings-modules-mail-left-click = Linksklick
    .description = Aktion bei Linksklick. Leer für keine Aktion oder ein Shell-Befehl (z. B. Ihr E-Mail-Programm)

settings-modules-mail-right-click = Rechtsklick
    .description = Shell-Befehl bei Rechtsklick

settings-modules-mail-middle-click = Mittelklick
    .description = Shell-Befehl bei Mittelklick

settings-modules-mail-scroll-up = Nach oben scrollen
    .description = Shell-Befehl beim Scrollen nach oben

settings-modules-mail-scroll-down = Nach unten scrollen
    .description = Shell-Befehl beim Scrollen nach unten

## Format placeholders

settings-modules-mail-format-placeholder-count = Anzahl der Nachrichten, die der Abfrage entsprechen

settings-modules-mail-notify-summary-placeholder-sender = Absender der Nachricht (Name oder Adresse)

settings-modules-mail-notify-summary-placeholder-subject = Betreff der Nachricht

settings-modules-mail-notify-summary-placeholder-count = Gesamtzahl der Nachrichten, die der Abfrage entsprechen

settings-modules-mail-notify-summary-placeholder-new = Wie viele seit der letzten Zählung eingetroffen sind

settings-modules-mail-notify-body-placeholder-sender = Absender der Nachricht (Name oder Adresse)

settings-modules-mail-notify-body-placeholder-subject = Betreff der Nachricht

settings-modules-mail-notify-body-placeholder-count = Gesamtzahl der Nachrichten, die der Abfrage entsprechen

settings-modules-mail-notify-body-placeholder-new = Wie viele seit der letzten Zählung eingetroffen sind

settings-modules-mail-notify-click-placeholder-thread = Die notmuch-Thread-ID der Nachricht
