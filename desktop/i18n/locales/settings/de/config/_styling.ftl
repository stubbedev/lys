### lys-shell Configuration - Styling Settings

## Styling Configuration

settings-styling-scale = Skalierung
    .description = Skalierungsfaktor für Dropdowns, Popover und Dialoge

settings-styling-rounding = Abrundung
    .description = Eckenabrundung für Dropdowns, Popover und Dialoge

settings-styling-appearance = Erscheinungsbild
    .description = Heller/dunkler Modus. Automatisch folgt der Palette/dem Anbieter; Hell/Dunkel erzwingen ihn

settings-styling-high-contrast = Hoher Kontrast
    .description = Eine kontrastreiche Palette, durchgehende Rahmen und deckende Flächen für die Shell sowie höherer Kontrast für Apps über das Desktop-Portal

settings-styling-reduce-transparency = Transparenz reduzieren
    .description = Alle Shell-Oberflächen deckend machen, ohne Hintergrundunschärfe

settings-styling-theme-provider = Themenanbieter
    .description = Quelle der Farbpalette (lys-shell, wallpaper, matugen, pywal, wallust)

settings-styling-theming-monitor = Monitor für das Thema
    .description = Monitor, dessen Hintergrundbild die Farbextraktion bestimmt

## Matugen Configuration

settings-styling-matugen-scheme = Matugen-Schema
    .description = Farbschematyp für matugen und den Hintergrundbild-Anbieter

settings-styling-matugen-contrast = Matugen-Kontrast
    .description = Kontraststufe für matugen und den Hintergrundbild-Anbieter (-1.0 bis 1.0)

settings-styling-matugen-source-color = Matugen-Quellfarbe
    .description = Index der Quellfarbe für matugen und den Hintergrundbild-Anbieter

settings-styling-matugen-light = Matugen heller Modus
    .description = Ein helles Farbschema mit matugen oder dem Hintergrundbild-Anbieter erzeugen

## Wallust Configuration

settings-styling-wallust-palette = Wallust-Palette
    .description = Palettenmodus für die Farberzeugung mit wallust

settings-styling-wallust-saturation = Wallust-Sättigung
    .description = Farbsättigung für wallust überschreiben (0 zum Deaktivieren)

settings-styling-wallust-check-contrast = Wallust-Kontrastprüfung
    .description = Ob wallust einen Mindestkontrast zwischen Farben erzwingt

settings-styling-wallust-backend = Wallust-Backend
    .description = Bildabtastverfahren für die Farbextraktion mit wallust

settings-styling-wallust-colorspace = Wallust-Farbraum
    .description = Farbraum für die Auswahl der dominanten Farbe in wallust

settings-styling-wallust-apply-globally = Wallust global anwenden
    .description = Wallust-Farben auf Terminals und externe Werkzeuge anwenden

## Pywal Configuration

settings-styling-pywal-saturation = Pywal-Sättigung
    .description = Farbsättigung für pywal (0.0 bis 1.0)

settings-styling-pywal-contrast = Pywal-Kontrast
    .description = Mindestkontrastverhältnis für pywal (1.0 bis 21.0)

settings-styling-pywal-light = Pywal heller Modus
    .description = Ein helles Farbschema mit pywal erzeugen

settings-styling-pywal-apply-globally = Pywal global anwenden
    .description = Pywal-Farben auf Terminals und externe Werkzeuge anwenden

## Palette Configuration

settings-palette-bg = Hintergrund
    .description = Grundhintergrundfarbe (dunkelste Ebene)

settings-palette-surface = Fläche
    .description = Hintergrundfarbe für Karten und Seitenleisten

settings-palette-elevated = Erhöht
    .description = Hintergrundfarbe erhöhter Elemente

settings-palette-fg = Vordergrund
    .description = Primäre Textfarbe

settings-palette-fg-muted = Gedämpfter Vordergrund
    .description = Sekundäre Textfarbe

settings-palette-primary = Primär
    .description = Akzentfarbe für interaktive Elemente

settings-palette-red = Rot
    .description = Semantische Farbe Rot

settings-palette-yellow = Gelb
    .description = Semantische Farbe Gelb

settings-palette-green = Grün
    .description = Semantische Farbe Grün

settings-palette-blue = Blau
    .description = Semantische Farbe Blau


## Appearance variants
enum-appearance-auto = Automatisch
enum-appearance-light = Hell
enum-appearance-dark = Dunkel

## ThemeProvider variants
enum-theme-provider-lys-shell = lys-shell
enum-theme-provider-matugen = Matugen
enum-theme-provider-pywal = Pywal
enum-theme-provider-wallust = Wallust
enum-theme-provider-wallpaper = Hintergrundbild

## MatugenScheme variants
enum-matugen-scheme-content = Inhalt
enum-matugen-scheme-expressive = Ausdrucksstark
enum-matugen-scheme-fidelity = Originalgetreu
enum-matugen-scheme-fruit-salad = Obstsalat
enum-matugen-scheme-monochrome = Monochrom
enum-matugen-scheme-neutral = Neutral
enum-matugen-scheme-rainbow = Regenbogen
enum-matugen-scheme-tonal-spot = Tonaler Akzent
enum-matugen-scheme-vibrant = Lebendig

## WallustPalette variants
enum-wallust-palette-dark16 = Dark 16
enum-wallust-palette-dark = Dark
enum-wallust-palette-darkcomp = Darkcomp
enum-wallust-palette-darkcomp16 = Darkcomp16
enum-wallust-palette-harddark = Harddark
enum-wallust-palette-harddark16 = Harddark16
enum-wallust-palette-harddarkcomp = Harddarkcomp
enum-wallust-palette-harddarkcomp16 = Harddarkcomp16
enum-wallust-palette-light = Light
enum-wallust-palette-light16 = Light16
enum-wallust-palette-lightcomp = Lightcomp
enum-wallust-palette-lightcomp16 = Lightcomp16
enum-wallust-palette-softdark = Softdark
enum-wallust-palette-softdark16 = Softdark16
enum-wallust-palette-softdarkcomp = Softdarkcomp
enum-wallust-palette-softdarkcomp16 = Softdarkcomp16
enum-wallust-palette-softlight = Softlight
enum-wallust-palette-softlight16 = Softlight16
enum-wallust-palette-softlightcomp = Softlightcomp
enum-wallust-palette-softlightcomp16 = Softlightcomp16
enum-wallust-palette-ansidark = Ansidark
enum-wallust-palette-ansidark16 = Ansidark16

## WallustBackend variants
enum-wallust-backend-full = Full
enum-wallust-backend-resized = Resized
enum-wallust-backend-wal = Wal
enum-wallust-backend-thumb = Thumb
enum-wallust-backend-fastresize = Fastresize
enum-wallust-backend-kmeans = K-Means

## WallustColorspace variants
enum-wallust-colorspace-lab = Lab
enum-wallust-colorspace-labmixed = Labmixed
enum-wallust-colorspace-lch = Lch
enum-wallust-colorspace-lchmixed = Lchmixed
enum-wallust-colorspace-lchansi = Lchansi

## FontWeightClass variants
enum-font-weight-class-normal = Normal
enum-font-weight-class-medium = Mittel
enum-font-weight-class-semibold = Halbfett
enum-font-weight-class-bold = Fett

## RoundingLevel variants
enum-rounding-level-default = Standard
enum-rounding-level-none = Keine
enum-rounding-level-sm = Klein
enum-rounding-level-md = Mittel
enum-rounding-level-lg = Groß
enum-rounding-level-full = Voll
