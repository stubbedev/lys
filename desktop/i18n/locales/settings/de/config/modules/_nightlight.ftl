### lys-shell Configuration - Night Light Module

## Night Light Module Configuration

settings-modules-nightlight-temperature = Temperatur
    .description = Farbtemperatur in Kelvin bei aktivem Filter (1000–20000)

settings-modules-nightlight-gamma = Gamma
    .description = Bildschirm-Gamma in Prozent bei aktivem Filter (0–200)

settings-modules-nightlight-auto-schedule = Automatischer Zeitplan
    .description = Den Filter von selbst ein- und ausschalten, nach dem Zeitplan unten

settings-modules-nightlight-schedule = Zeitplan
    .description = Der Sonne von Sonnenuntergang bis Sonnenaufgang folgen oder die Zeiten selbst festlegen

settings-modules-nightlight-custom-hours = Zeiten
    .description = Wann der Filter einschaltet und wann er wieder ausschaltet

settings-modules-nightlight-transition-duration = Übergang
    .description = Dauer des Übergangs zwischen Tag und Nacht in Minuten, 0 für sofortiges Umschalten (0–120)

settings-modules-nightlight-location = Ort
    .description = Wo der Zeitplan der Sonne folgt: die Stadt deiner Zeitzone oder eine gewählte Stadt

settings-modules-nightlight-icon-off = Symbol für aus
    .description = Symbol, wenn der Filter deaktiviert ist

settings-modules-nightlight-icon-on = Symbol für an
    .description = Symbol, wenn der Filter aktiviert ist

settings-modules-nightlight-format = Format
    .description = Formatzeichenkette der Beschriftung. Platzhalter: {"{{ status }}"}, {"{{ temp }}"}, {"{{ gamma }}"}, {"{{ config_temp }}"}, {"{{ config_gamma }}"}

settings-modules-nightlight-border-show = Rahmen anzeigen
    .description = Rahmen um die Schaltfläche anzeigen

settings-modules-nightlight-border-color = Rahmenfarbe
    .description = Farbtoken des Rahmens

settings-modules-nightlight-icon-show = Symbol anzeigen
    .description = Modulsymbol anzeigen

settings-modules-nightlight-tooltip = Kurzinfo anzeigen
    .description = Den Zustand des Moduls beim Überfahren in einer Kurzinfo anzeigen

settings-modules-nightlight-icon-color = Symbolfarbe
    .description = Vordergrundfarbe des Symbols

settings-modules-nightlight-icon-bg-color = Symbolhintergrund
    .description = Hintergrundfarbe des Symbolcontainers

settings-modules-nightlight-label-show = Beschriftung anzeigen
    .description = Beschriftung anzeigen

settings-modules-nightlight-label-color = Beschriftungsfarbe
    .description = Textfarbe der Beschriftung

settings-modules-nightlight-label-max-length = Maximale Beschriftungslänge
    .description = Maximale Zeichenanzahl vor dem Kürzen

settings-modules-nightlight-button-bg-color = Schaltflächenhintergrund
    .description = Hintergrundfarbe der Schaltfläche

settings-modules-nightlight-left-click = Linksklick
    .description = Aktion bei Linksklick. `:toggle` für das eingebaute An/Aus, leer für keine Aktion oder ein Shell-Befehl

settings-modules-nightlight-right-click = Rechtsklick
    .description = Shell-Befehl bei Rechtsklick

settings-modules-nightlight-middle-click = Mittelklick
    .description = Shell-Befehl bei Mittelklick

settings-modules-nightlight-scroll-up = Nach oben scrollen
    .description = Shell-Befehl beim Scrollen nach oben

settings-modules-nightlight-scroll-down = Nach unten scrollen
    .description = Shell-Befehl beim Scrollen nach unten

## Format placeholders

settings-modules-nightlight-format-placeholder-status = Filterstatus (An, Aus)

settings-modules-nightlight-format-placeholder-temp = Aktuelle Temperatur in Kelvin (-- wenn aus)

settings-modules-nightlight-format-placeholder-gamma = Aktuelles Gamma in Prozent (-- wenn aus)

settings-modules-nightlight-format-placeholder-config-temp = Eingestellte Temperatur in Kelvin

settings-modules-nightlight-format-placeholder-config-gamma = Eingestelltes Gamma in Prozent

enum-nightlight-schedule-sunset-to-sunrise = Sonnenuntergang bis Sonnenaufgang
enum-nightlight-schedule-custom = Eigene Zeiten
