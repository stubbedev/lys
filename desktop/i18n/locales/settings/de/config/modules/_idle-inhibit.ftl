### lys-shell Configuration - Idle Inhibit Module

## Idle Inhibit Module Configuration

settings-modules-idle-inhibit-startup-duration = Dauer beim Start
    .description = Dauer in Minuten beim Start des Dienstes (0 für unbegrenzt)

settings-modules-idle-inhibit-icon-inactive = Symbol für inaktiv
    .description = Symbol, wenn die Leerlaufsperre aus ist

settings-modules-idle-inhibit-icon-active = Symbol für aktiv
    .description = Symbol, wenn die Leerlaufsperre an ist

settings-modules-idle-inhibit-format = Format
    .description = Formatzeichenkette mit Platzhaltern: {"{{ state }}"}, {"{{ remaining }}"}, {"{{ duration }}"}

settings-modules-idle-inhibit-border-show = Rahmen anzeigen
    .description = Rahmen um die Schaltfläche anzeigen

settings-modules-idle-inhibit-border-color = Rahmenfarbe
    .description = Farbtoken des Rahmens

settings-modules-idle-inhibit-icon-show = Symbol anzeigen
    .description = Modulsymbol anzeigen

settings-modules-idle-inhibit-tooltip = Kurzinfo anzeigen
    .description = Den Zustand des Moduls beim Überfahren in einer Kurzinfo anzeigen

settings-modules-idle-inhibit-icon-color = Symbolfarbe
    .description = Vordergrundfarbe des Symbols

settings-modules-idle-inhibit-icon-bg-color = Symbolhintergrund
    .description = Hintergrundfarbe des Symbolcontainers

settings-modules-idle-inhibit-label-show = Beschriftung anzeigen
    .description = Beschriftung anzeigen

settings-modules-idle-inhibit-label-color = Beschriftungsfarbe
    .description = Textfarbe der Beschriftung

settings-modules-idle-inhibit-label-max-length = Maximale Beschriftungslänge
    .description = Maximale Zeichenanzahl vor dem Kürzen

settings-modules-idle-inhibit-button-bg-color = Schaltflächenhintergrund
    .description = Hintergrundfarbe der Schaltfläche

settings-modules-idle-inhibit-left-click = Linksklick
    .description = Shell-Befehl bei Linksklick

settings-modules-idle-inhibit-right-click = Rechtsklick
    .description = Shell-Befehl bei Rechtsklick

settings-modules-idle-inhibit-middle-click = Mittelklick
    .description = Shell-Befehl bei Mittelklick

settings-modules-idle-inhibit-scroll-up = Nach oben scrollen
    .description = Shell-Befehl beim Scrollen nach oben

settings-modules-idle-inhibit-scroll-down = Nach unten scrollen
    .description = Shell-Befehl beim Scrollen nach unten

## Format placeholders

settings-modules-idle-inhibit-format-placeholder-state = Zustand der Sperre (An, Aus)

settings-modules-idle-inhibit-format-placeholder-remaining = Verbleibende Zeit als M:SS oder H:MM:SS (∞ bei unbegrenzt, - wenn aus)

settings-modules-idle-inhibit-format-placeholder-duration = Gesamtdauer in Minuten (∞ bei unbegrenzt)
