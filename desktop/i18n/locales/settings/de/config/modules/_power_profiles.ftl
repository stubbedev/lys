### lys-shell Configuration - Power Profiles Module

## Power Profiles Module Configuration

settings-modules-power-profiles-format = Format
    .description = Formatzeichenkette der Beschriftung. Platzhalter: {"{{ profile }}"}

settings-modules-power-profiles-icon-power-saver = Symbol für Energiesparen
    .description = Symbol, solange das Profil Energiesparen aktiv ist

settings-modules-power-profiles-icon-balanced = Symbol für Ausgewogen
    .description = Symbol, solange das Profil Ausgewogen aktiv ist

settings-modules-power-profiles-icon-performance = Symbol für Leistung
    .description = Symbol, solange das Profil Leistung aktiv ist

settings-modules-power-profiles-color-power-saver = Farbe für Energiesparen
    .description = Symbol- und Beschriftungsfarbe, solange das Profil Energiesparen aktiv ist

settings-modules-power-profiles-color-balanced = Farbe für Ausgewogen
    .description = Symbol- und Beschriftungsfarbe, solange das Profil Ausgewogen aktiv ist

settings-modules-power-profiles-color-performance = Farbe für Leistung
    .description = Symbol- und Beschriftungsfarbe, solange das Profil Leistung aktiv ist

settings-modules-power-profiles-border-show = Rahmen anzeigen
    .description = Rahmen um die Schaltfläche anzeigen

settings-modules-power-profiles-border-color = Rahmenfarbe
    .description = Farbtoken des Rahmens

settings-modules-power-profiles-icon-show = Symbol anzeigen
    .description = Modulsymbol anzeigen

settings-modules-power-profiles-tooltip = Kurzinfo anzeigen
    .description = Den Zustand des Moduls beim Überfahren in einer Kurzinfo anzeigen

settings-modules-power-profiles-icon-color = Symbolfarbe
    .description = Vordergrundfarbe des Symbols (pro aktivem Profil überschrieben)

settings-modules-power-profiles-icon-bg-color = Symbolhintergrund
    .description = Hintergrundfarbe des Symbolcontainers

settings-modules-power-profiles-label-show = Beschriftung anzeigen
    .description = Beschriftung anzeigen

settings-modules-power-profiles-label-color = Beschriftungsfarbe
    .description = Textfarbe der Beschriftung (pro aktivem Profil überschrieben)

settings-modules-power-profiles-label-max-length = Maximale Beschriftungslänge
    .description = Maximale Zeichenanzahl vor dem Kürzen

settings-modules-power-profiles-button-bg-color = Schaltflächenhintergrund
    .description = Hintergrundfarbe der Schaltfläche

settings-modules-power-profiles-left-click = Linksklick
    .description = Aktion bei Linksklick. `:cycle` für den eingebauten Profilwechsel, leer für keine Aktion oder ein Shell-Befehl

settings-modules-power-profiles-right-click = Rechtsklick
    .description = Shell-Befehl bei Rechtsklick

settings-modules-power-profiles-middle-click = Mittelklick
    .description = Shell-Befehl bei Mittelklick

settings-modules-power-profiles-scroll-up = Nach oben scrollen
    .description = Shell-Befehl beim Scrollen nach oben

settings-modules-power-profiles-scroll-down = Nach unten scrollen
    .description = Shell-Befehl beim Scrollen nach unten

settings-modules-power-profiles-auto-saver-at = Automatisches Energiesparen
    .description = Akkustand, ab dem bis zur Rückkehr der Netzversorgung auf Energiesparen umgeschaltet wird

## Format placeholders

settings-modules-power-profiles-format-placeholder-profile = Name des aktiven Profils (power-saver, balanced, performance)

## AutoSaverLevel variants
enum-auto-saver-level-low = Niedrig
enum-auto-saver-level-critical = Kritisch
enum-auto-saver-level-never = Nie

