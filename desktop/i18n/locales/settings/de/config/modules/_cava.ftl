### lys-shell Configuration - Cava Module

## Audio Processing

settings-modules-cava-bars = Anzahl der Balken
    .description = Anzahl der Frequenzbalken

settings-modules-cava-framerate = Bildrate
    .description = Aktualisierungsrate der Visualisierung in Bildern pro Sekunde

settings-modules-cava-stereo = Stereo
    .description = Balken auf linken und rechten Audiokanal aufteilen

settings-modules-cava-noise-reduction = Rauschunterdrückung
    .description = Stärke des Rauschfilters (0.0 schnell/unruhig bis 1.0 langsam/glatt)

settings-modules-cava-monstercat = Monstercat-Glättung
    .description = Weicher Abfall über benachbarte Balken (0.0 = aus)

settings-modules-cava-waves = Wellenglättung
    .description = Anzahl der wellenartigen Glättungen (0 = aus)

settings-modules-cava-low-cutoff = Untere Grenzfrequenz
    .description = Untere Grenzfrequenz in Hz

settings-modules-cava-high-cutoff = Obere Grenzfrequenz
    .description = Obere Grenzfrequenz in Hz

settings-modules-cava-input = Eingabemethode
    .description = Backend für die Audioaufnahme

settings-modules-cava-source = Quelle
    .description = Der Ton, dem die Balken folgen: automatisch gewählt oder eine Quelle nach Namen

## Visualization

settings-modules-cava-style = Stil
    .description = Darstellungsstil der Visualisierung (Balken, Welle oder Spitzen)

settings-modules-cava-direction = Richtung
    .description = Wachstumsrichtung der Balken relativ zum Leistenrand

settings-modules-cava-color = Farbe
    .description = Farbe der Visualisierung

settings-modules-cava-bar-width = Balkenbreite
    .description = Breite jedes Frequenzbalkens in Pixeln

settings-modules-cava-bar-gap = Balkenabstand
    .description = Abstand zwischen Frequenzbalken in Pixeln

## Container

settings-modules-cava-internal-padding = Innenabstand
    .description = Abstand an den Enden der Visualisierung in rem
    .description-pixels = Abstand an den Enden der Visualisierung in Pixeln

settings-modules-cava-button-bg-color = Hintergrundfarbe
    .description = Hintergrundfarbe des Moduls

settings-modules-cava-border-show = Rahmen anzeigen
    .description = Rahmen um die Visualisierung anzeigen

settings-modules-cava-border-color = Rahmenfarbe
    .description = Farbtoken des Rahmens

settings-modules-cava-left-click = Linksklick
    .description = Aktion bei Linksklick

settings-modules-cava-right-click = Rechtsklick
    .description = Aktion bei Rechtsklick

settings-modules-cava-middle-click = Mittelklick
    .description = Aktion bei Mittelklick

settings-modules-cava-scroll-up = Nach oben scrollen
    .description = Aktion beim Scrollen nach oben

settings-modules-cava-scroll-down = Nach unten scrollen
    .description = Aktion beim Scrollen nach unten


## CavaStyle variants
enum-cava-style-bars = Balken
enum-cava-style-wave = Welle
enum-cava-style-peaks = Spitzen

## CavaDirection variants
enum-cava-direction-normal = Normal
enum-cava-direction-reverse = Umgekehrt
enum-cava-direction-mirror = Gespiegelt
