### lys-shell Configuration - Notifications Module

## Notifications Module Configuration

settings-modules-notifications-enabled = Aktiviert
    .description = Benachrichtigungsdienst und -modul aktivieren

settings-modules-notifications-icon-name = Standardsymbol
    .description = Symbol, wenn keine Benachrichtigungen vorhanden sind

settings-modules-notifications-icon-unread = Symbol für ungelesen
    .description = Symbol, wenn Benachrichtigungen vorhanden sind

settings-modules-notifications-icon-dnd = Symbol für „Nicht stören“
    .description = Symbol, wenn „Nicht stören“ aktiv ist

settings-modules-notifications-border-show = Rahmen anzeigen
    .description = Rahmen um die Schaltfläche anzeigen

settings-modules-notifications-border-color = Rahmenfarbe
    .description = Farbtoken des Rahmens

settings-modules-notifications-icon-show = Symbol anzeigen
    .description = Modulsymbol anzeigen

settings-modules-notifications-tooltip = Kurzinfo anzeigen
    .description = Den Zustand des Moduls beim Überfahren in einer Kurzinfo anzeigen

settings-modules-notifications-icon-color = Symbolfarbe
    .description = Vordergrundfarbe des Symbols

settings-modules-notifications-icon-bg-color = Symbolhintergrund
    .description = Hintergrundfarbe des Symbolcontainers

settings-modules-notifications-label-show = Beschriftung anzeigen
    .description = Beschriftung mit der Anzahl der Benachrichtigungen anzeigen

settings-modules-notifications-label-color = Beschriftungsfarbe
    .description = Textfarbe der Beschriftung

settings-modules-notifications-label-max-length = Maximale Beschriftungslänge
    .description = Maximale Zeichenanzahl vor dem Kürzen

settings-modules-notifications-button-bg-color = Schaltflächenhintergrund
    .description = Hintergrundfarbe der Schaltfläche

settings-modules-notifications-middle-click = Mittelklick
    .description = Shell-Befehl bei Mittelklick

settings-modules-notifications-scroll-up = Nach oben scrollen
    .description = Shell-Befehl beim Scrollen nach oben

settings-modules-notifications-scroll-down = Nach unten scrollen
    .description = Shell-Befehl beim Scrollen nach unten

settings-modules-notifications-left-click = Linksklick
    .description = Aktion bei Linksklick

settings-modules-notifications-right-click = Rechtsklick
    .description = Aktion bei Rechtsklick

settings-modules-notifications-popup-shadow = Popup-Schatten
    .description = Einen Schatten hinter Benachrichtigungs-Popups zeichnen

settings-modules-notifications-thresholds = Schwellenwerte
    .description = Farbschwellen, die sich nach dem Wert ändern


## PopupPosition variants

## StackingOrder variants
enum-stacking-order-newest-first = Neueste zuerst
enum-stacking-order-oldest-first = Älteste zuerst

## PopupStack variants
enum-popup-stack-per-app = Pro App
enum-popup-stack-all = Alle
enum-popup-stack-off = Aus

## PopupCloseBehavior variants
enum-popup-close-behavior-dismiss = Verwerfen
enum-popup-close-behavior-remove = Entfernen

## UrgencyBarThreshold variants
enum-urgency-bar-threshold-low = Niedrig
enum-urgency-bar-threshold-normal = Normal
enum-urgency-bar-threshold-critical = Kritisch
enum-urgency-bar-threshold-none = Keine

## IconSource variants
enum-icon-source-automatic = Automatisch
enum-icon-source-mapped = Zugeordnet
enum-icon-source-application = Anwendung

## NotificationSounds variants
enum-notification-sounds-off = Aus
enum-notification-sounds-critical = Nur kritische
enum-notification-sounds-all = Alle

## Do Not Disturb and Focus

settings-modules-notifications-dnd-allow-critical = Kritische erlauben
    .description = Kritische Benachrichtigungen, etwa ein schwacher Akku oder ein Alarm, erscheinen auch bei „Nicht stören“

settings-modules-notifications-dnd-allow-apps = Erlaubte Apps
    .description = App-Namen oder Desktop-Einträge, mit Globs * und ?, deren Benachrichtigungen auch bei „Nicht stören“ erscheinen

settings-modules-notifications-dnd-schedule = Zeitplan
    .description = Zeiten in der Woche, zu denen sich „Nicht stören“ selbst einschaltet; ein Zeitfenster, das vor seinem Beginn endet, reicht über Mitternacht

settings-modules-notifications-dnd-while-sharing = Während der Bildschirmfreigabe
    .description = „Nicht stören“ einschalten, solange der Bildschirm freigegeben oder aufgenommen wird

settings-modules-notifications-dnd-while-fullscreen = Im Vollbild
    .description = „Nicht stören“ einschalten, solange ein Fenster im Vollbild ist
