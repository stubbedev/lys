### lys-shell Configuration - Media Module

## Media Module Configuration

settings-modules-media-icon-type = Symboltyp
    .description = Anzeigemodus des Symbols (default, application, spinning-disc, application-mapped). spinning-disc benötigt wegen der Animation etwas mehr CPU

settings-modules-media-player-icons = Player-Symbole
    .description = Eigene Zuordnung von Playern zu Symbolen (Glob-Muster zu Symbolname)

settings-modules-media-players-ignored = Ignorierte Player
    .description = Muster für Bus-Namen von Playern, die von der Erkennung ausgeschlossen werden

settings-modules-media-player-priority = Player-Priorität
    .description = Bevorzugte Reihenfolge der Player als Glob-Muster für Bus-Namen

settings-modules-media-lyrics = Liedtexte
    .description = Synchronisierte Liedtexte von LRCLIB im Medien-Dropdown anzeigen. Sendet Interpret, Titel, Album und Länge jedes Titels an lrclib.net

settings-modules-media-format = Format
    .description = Beschriftungsformat mit Platzhaltern: {"{{ title }}"}, {"{{ artist }}"}, {"{{ album }}"}, {"{{ status }}"}, {"{{ status_icon }}"}

settings-modules-media-icon-name = Symbolname
    .description = Symbolischer Symbolname für den Standardmodus

settings-modules-media-spinning-disc-icon = Symbol für drehende Scheibe
    .description = Symbol für den Modus spinning-disc

settings-modules-media-border-show = Rahmen anzeigen
    .description = Rahmen um die Schaltfläche anzeigen

settings-modules-media-border-color = Rahmenfarbe
    .description = Farbtoken des Rahmens

settings-modules-media-icon-show = Symbol anzeigen
    .description = Modulsymbol anzeigen

settings-modules-media-tooltip = Kurzinfo anzeigen
    .description = Den Zustand des Moduls beim Überfahren in einer Kurzinfo anzeigen

settings-modules-media-icon-color = Symbolfarbe
    .description = Vordergrundfarbe des Symbols

settings-modules-media-icon-bg-color = Symbolhintergrund
    .description = Hintergrundfarbe des Symbolcontainers

settings-modules-media-label-show = Beschriftung anzeigen
    .description = Textbeschriftung anzeigen

settings-modules-media-label-color = Beschriftungsfarbe
    .description = Textfarbe der Beschriftung

settings-modules-media-label-max-length = Maximale Beschriftungslänge
    .description = Maximale Zeichenanzahl vor dem Kürzen

settings-modules-media-button-bg-color = Schaltflächenhintergrund
    .description = Hintergrundfarbe der Schaltfläche

settings-modules-media-left-click = Linksklick
    .description = Aktion bei Linksklick

settings-modules-media-right-click = Rechtsklick
    .description = Shell-Befehl bei Rechtsklick

settings-modules-media-middle-click = Mittelklick
    .description = Shell-Befehl bei Mittelklick

settings-modules-media-scroll-up = Nach oben scrollen
    .description = Shell-Befehl beim Scrollen nach oben

settings-modules-media-scroll-down = Nach unten scrollen
    .description = Shell-Befehl beim Scrollen nach unten


## MediaIconType variants
enum-media-icon-type-default = Standard
enum-media-icon-type-application = Anwendung
enum-media-icon-type-spinning-disc = Drehende Scheibe
enum-media-icon-type-application-mapped = Anwendung zugeordnet

## Format placeholders

settings-modules-media-format-placeholder-title = Titel des Stücks

settings-modules-media-format-placeholder-artist = Name(n) der Interpreten

settings-modules-media-format-placeholder-album = Albumname

settings-modules-media-format-placeholder-status = Wiedergabestatus (Wiedergabe, Pausiert, Gestoppt)

settings-modules-media-format-placeholder-status-icon = Zeichen für den Wiedergabestatus
