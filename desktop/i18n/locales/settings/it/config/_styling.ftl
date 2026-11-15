### lys-shell Configuration - Styling Settings

## Styling Configuration

settings-styling-scale = Scala
    .description = Moltiplicatore di scala per menu a discesa, popover e finestre di dialogo

settings-styling-rounding = Arrotondamento
    .description = Arrotondamento degli angoli per menu a discesa, popover e finestre di dialogo

settings-styling-appearance = Aspetto
    .description = Modalità chiara/scura. Automatico segue la tavolozza/il provider; Chiaro/Scuro la impongono

settings-styling-high-contrast = Contrasto elevato
    .description = Una tavolozza ad alto contrasto, bordi pieni e superfici opache per la shell, e un contrasto più elevato richiesto alle app tramite il portale desktop

settings-styling-reduce-transparency = Riduci la trasparenza
    .description = Rende opache tutte le superfici della shell, senza sfocatura dello sfondo

settings-styling-theme-provider = Provider del tema
    .description = Origine della tavolozza dei colori (lys-shell, wallpaper, matugen, pywal, wallust)

settings-styling-theming-monitor = Monitor per il tema
    .description = Monitor il cui sfondo guida l'estrazione dei colori

## Matugen Configuration

settings-styling-matugen-scheme = Schema Matugen
    .description = Tipo di schema di colori per matugen e per il provider wallpaper

settings-styling-matugen-contrast = Contrasto Matugen
    .description = Livello di contrasto per matugen e per il provider wallpaper (da -1.0 a 1.0)

settings-styling-matugen-source-color = Colore di origine Matugen
    .description = Indice del colore di origine per matugen e per il provider wallpaper

settings-styling-matugen-light = Modalità chiara Matugen
    .description = Genera uno schema di colori chiaro con matugen o con il provider wallpaper

## Wallust Configuration

settings-styling-wallust-palette = Tavolozza Wallust
    .description = Modalità della tavolozza per la generazione dei colori di wallust

settings-styling-wallust-saturation = Saturazione Wallust
    .description = Saturazione dei colori alternativa per wallust (0 per disattivare)

settings-styling-wallust-check-contrast = Controllo del contrasto Wallust
    .description = Se wallust impone un contrasto minimo tra i colori

settings-styling-wallust-backend = Backend Wallust
    .description = Metodo di campionamento dell'immagine per l'estrazione dei colori di wallust

settings-styling-wallust-colorspace = Spazio colore Wallust
    .description = Spazio colore usato per la scelta del colore dominante in wallust

settings-styling-wallust-apply-globally = Applica Wallust globalmente
    .description = Applica i colori di wallust ai terminali e agli strumenti esterni

## Pywal Configuration

settings-styling-pywal-saturation = Saturazione Pywal
    .description = Saturazione dei colori per pywal (da 0.0 a 1.0)

settings-styling-pywal-contrast = Contrasto Pywal
    .description = Rapporto di contrasto minimo per pywal (da 1.0 a 21.0)

settings-styling-pywal-light = Modalità chiara Pywal
    .description = Genera uno schema di colori chiaro con pywal

settings-styling-pywal-apply-globally = Applica Pywal globalmente
    .description = Applica i colori di pywal ai terminali e agli strumenti esterni

## Palette Configuration

settings-palette-bg = Sfondo
    .description = Colore di sfondo di base (il livello più scuro)

settings-palette-surface = Superficie
    .description = Colore di sfondo di schede e barra laterale

settings-palette-elevated = Rialzato
    .description = Colore di sfondo degli elementi rialzati

settings-palette-fg = Primo piano
    .description = Colore del testo principale

settings-palette-fg-muted = Primo piano attenuato
    .description = Colore del testo secondario

settings-palette-primary = Primario
    .description = Colore d'accento per gli elementi interattivi

settings-palette-red = Rosso
    .description = Colore semantico rosso

settings-palette-yellow = Giallo
    .description = Colore semantico giallo

settings-palette-green = Verde
    .description = Colore semantico verde

settings-palette-blue = Blu
    .description = Colore semantico blu


## Appearance variants
enum-appearance-auto = Automatico
enum-appearance-light = Chiaro
enum-appearance-dark = Scuro

## ThemeProvider variants
enum-theme-provider-lys-shell = lys-shell
enum-theme-provider-matugen = Matugen
enum-theme-provider-pywal = Pywal
enum-theme-provider-wallust = Wallust
enum-theme-provider-wallpaper = Sfondo

## MatugenScheme variants
enum-matugen-scheme-content = Contenuto
enum-matugen-scheme-expressive = Espressivo
enum-matugen-scheme-fidelity = Fedeltà
enum-matugen-scheme-fruit-salad = Macedonia
enum-matugen-scheme-monochrome = Monocromatico
enum-matugen-scheme-neutral = Neutro
enum-matugen-scheme-rainbow = Arcobaleno
enum-matugen-scheme-tonal-spot = Punto tonale
enum-matugen-scheme-vibrant = Vivace

## WallustPalette variants
enum-wallust-palette-dark16 = Scuro 16
enum-wallust-palette-dark = Scuro
enum-wallust-palette-darkcomp = Darkcomp
enum-wallust-palette-darkcomp16 = Darkcomp16
enum-wallust-palette-harddark = Harddark
enum-wallust-palette-harddark16 = Harddark16
enum-wallust-palette-harddarkcomp = Harddarkcomp
enum-wallust-palette-harddarkcomp16 = Harddarkcomp16
enum-wallust-palette-light = Chiaro
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
enum-wallust-backend-full = Completa
enum-wallust-backend-resized = Ridimensionata
enum-wallust-backend-wal = Wal
enum-wallust-backend-thumb = Miniatura
enum-wallust-backend-fastresize = Fastresize
enum-wallust-backend-kmeans = K-Means

## WallustColorspace variants
enum-wallust-colorspace-lab = Lab
enum-wallust-colorspace-labmixed = Labmixed
enum-wallust-colorspace-lch = Lch
enum-wallust-colorspace-lchmixed = Lchmixed
enum-wallust-colorspace-lchansi = Lchansi

## FontWeightClass variants
enum-font-weight-class-normal = Normale
enum-font-weight-class-medium = Medio
enum-font-weight-class-semibold = Semigrassetto
enum-font-weight-class-bold = Grassetto

## RoundingLevel variants
enum-rounding-level-default = Predefinito
enum-rounding-level-none = Nessuno
enum-rounding-level-sm = Piccolo
enum-rounding-level-md = Medio
enum-rounding-level-lg = Grande
enum-rounding-level-full = Completo
