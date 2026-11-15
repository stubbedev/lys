### lys-shell Configuration - Bar Settings

## Bar Configuration - Bar Settings

settings-bar-layout = Anordnung
    .description = Leistenanordnung pro Monitor, die die Anordnung der Module festlegt

settings-bar-scale = Skalierung
    .description = Leistenspezifischer Skalierungsfaktor für Abstände, Radius und andere Elemente

settings-bar-inset-edge = Randabstand
    .description = Abstand zwischen Leiste und dem Bildschirmrand, an dem sie sitzt

settings-bar-inset-ends = Endabstand
    .description = Abstand an den Enden der Leiste (links/rechts bei waagerecht, oben/unten bei senkrecht)

settings-bar-padding = Innenabstand
    .description = Innerer Abstand entlang der Leistendicke

settings-bar-padding-ends = Innenabstand an den Enden
    .description = Innerer Abstand an den Enden der Leiste

settings-bar-module-gap = Modulabstand
    .description = Abstand zwischen Modulen und Gruppen auf der Leiste

settings-bar-location = Position
    .description = Position der Leiste am Bildschirmrand (oben, unten, links, rechts)

settings-bar-exclusive = Exklusiv
    .description = Bildschirmplatz für die Leiste reservieren, statt über Fenstern zu zeichnen

settings-bar-auto-hide = Automatisch ausblenden
    .description = Wann sich die Leiste ausblendet; ein schmaler Streifen am Rand bleibt und zeigt sie unter dem Zeiger wieder an

settings-bar-auto-hide-delay-ms = Verzögerung beim Ausblenden
    .description = Wie lange die Leiste nach dem Verlassen durch den Zeiger sichtbar bleibt (Millisekunden)

settings-bar-layer = Ebene
    .description = Layer-Shell-Ebene, auf der die Leiste liegt

settings-bar-bg = Hintergrund
    .description = Hintergrundfarbe der Leiste

settings-bar-background-opacity = Hintergrunddeckkraft
    .description = Transparenz des Leistenhintergrunds (0–100 %)

settings-bar-border-location = Rahmenposition
    .description = Platzierung des Rahmens der Leiste

settings-bar-border-width = Rahmenbreite
    .description = Rahmenstärke in Pixeln

settings-bar-border-color = Rahmenfarbe
    .description = Rahmenfarbe der Leiste

settings-bar-rounding = Abrundung
    .description = Grad der Eckenabrundung der Leiste

settings-bar-shadow-enabled = Schatten
    .description = Schlagschatten für die Leiste aktivieren

## Bar Configuration - Button/Module Settings

settings-bar-button-opacity = Schaltflächendeckkraft
    .description = Transparenz der Leistenschaltflächen (0–100 %)

settings-bar-button-bg-opacity = Hintergrunddeckkraft der Schaltflächen
    .description = Transparenz der Hintergrundfarbe der Leistenschaltflächen (0–100 %)

settings-bar-button-icon-size = Symbolgröße der Schaltflächen
    .description = Größenfaktor für Schaltflächensymbole

settings-bar-button-icon-padding = Symbolabstand der Schaltflächen
    .description = Abstand um Schaltflächensymbole. Gilt nur für die Varianten block-prefix und icon-square.

settings-bar-button-label-size = Beschriftungsgröße der Schaltflächen
    .description = Größenfaktor für Schaltflächentext

settings-bar-button-label-weight = Schriftstärke der Schaltflächen
    .description = Schriftstärke der Schaltflächenbeschriftungen

settings-bar-button-label-padding = Beschriftungsabstand der Schaltflächen
    .description = Abstand um Schaltflächenbeschriftungen

settings-bar-button-gap = Schaltflächenabstand
    .description = Abstand zwischen Symbol und Beschriftung in Schaltflächen

settings-bar-button-icon-position = Symbolposition der Schaltflächen
    .description = Platzierung des Symbols relativ zur Beschriftung (Anfang oder Ende)

settings-bar-button-border-location = Rahmenposition der Schaltflächen
    .description = Platzierung des Rahmens der Leistenschaltflächen

settings-bar-button-border-width = Rahmenbreite der Schaltflächen
    .description = Rahmenstärke der Leistenschaltflächen in Pixeln

## Bar Configuration - Button Group Settings

settings-bar-button-group-border-location = Rahmenposition der Gruppe
    .description = Platzierung des Rahmens der Schaltflächengruppen

settings-bar-button-group-border-width = Rahmenbreite der Gruppe
    .description = Rahmenstärke der Schaltflächengruppen in Pixeln

settings-bar-button-group-padding = Innenabstand der Gruppe
    .description = Innerer Abstand in Schaltflächengruppen

settings-bar-group-module-gap = Modulabstand in der Gruppe
    .description = Abstand zwischen Modulen innerhalb einer Gruppe

settings-bar-button-group-background = Gruppenhintergrund
    .description = Hintergrundfarbe der Schaltflächengruppen

settings-bar-button-group-opacity = Gruppendeckkraft
    .description = Transparenz der Schaltflächengruppen (0–100 %)

settings-bar-button-group-border-color = Rahmenfarbe der Gruppe
    .description = Rahmenfarbe der Schaltflächengruppen

## Bar Configuration - Dropdown Settings

settings-bar-dropdown-size = Dropdown-Größe
    .description = Größenfaktor für Dropdown-Felder

settings-bar-dropdown-shadow = Dropdown-Schatten
    .description = Schlagschatten für Dropdown-Menüs aktivieren

settings-bar-dropdown-opacity = Dropdown-Deckkraft
    .description = Transparenz der Dropdown-Felder (0–100 %)

settings-bar-dropdown-autohide = Dropdown automatisch schließen
    .description = Dropdown beim Klick außerhalb schließen

settings-bar-dropdown-freeze-label = Beschriftung einfrieren
    .description = Beschriftung der Leistenschaltfläche nicht aktualisieren, solange ihr Dropdown offen ist

settings-bar-shadow = Schatten
    .description = Schlagschatten aktivieren

settings-bar-button-variant = Schaltflächenvariante
    .description = Visueller Stil der Leistenschaltflächen

settings-bar-button-rounding = Schaltflächenabrundung
    .description = Eckenabrundung der Leistenschaltflächen

settings-bar-button-group-module-gap = Modulabstand in der Gruppe
    .description = Abstand zwischen Modulen innerhalb einer Gruppe

settings-bar-button-group-rounding = Gruppenabrundung
    .description = Eckenabrundung der Schaltflächengruppen

## Location enum variants
enum-location-top = Oben
enum-location-bottom = Unten
enum-location-left = Links
enum-location-right = Rechts


## BarAutoHide variants
enum-bar-auto-hide-never = Nie
enum-bar-auto-hide-always = Immer
enum-bar-auto-hide-windows-overlap = Wenn Fenster überlappen
enum-bar-auto-hide-fullscreen = Im Vollbild

## ShadowPreset variants
enum-shadow-preset-none = Keiner
enum-shadow-preset-drop = Schlagschatten
enum-shadow-preset-floating = Schwebend

## BorderLocation variants
enum-border-location-none = Keine
enum-border-location-top = Oben
enum-border-location-bottom = Unten
enum-border-location-left = Links
enum-border-location-right = Rechts
enum-border-location-all = Alle

## BarButtonVariant variants
enum-bar-button-variant-basic = Einfach
enum-bar-button-variant-block-prefix = Blockpräfix
enum-bar-button-variant-icon-square = Symbolquadrat

## IconPosition variants
enum-icon-position-start = Anfang
enum-icon-position-end = Ende
