

settings-osd-enabled = Attivato
    .description = Mostra gli indicatori a schermo per volume, luminosità, ecc.

settings-osd-presets = Preset dei toast
    .description = Toast riutilizzabili attivabili con `lys-shell toast --preset <id>`

settings-osd-position = Posizione
    .description = Dove appare l'OSD sullo schermo (ad es. top-center, bottom-right)

settings-osd-duration = Durata
    .description = Per quanto resta visibile l'OSD prima di chiudersi automaticamente (ms)

settings-osd-monitor = Monitor
    .description = Quale monitor mostra l'OSD: "primary" o un connettore come "DP-1"

settings-osd-margin = Margine
    .description = Distanza tra il bordo dello schermo e l'OSD

settings-osd-border = Bordo
    .description = Disegna un bordo attorno all'OSD

settings-osd-layer = Livello
    .description = Livello layer-shell su cui viene posto l'OSD. La modalità tearing declassa overlay a top.

settings-osd-text-align = Allineamento del testo
    .description = Allineamento orizzontale del contenuto di toast e interruttori dell'OSD

settings-osd-media = In riproduzione
    .description = Cambi di brano che mostrano l'OSD della riproduzione: su richiesta (tasti multimediali e `lys-shell media next`), sempre o mai


## OsdPosition variants
enum-osd-position-top-left = In alto a sinistra
enum-osd-position-top = Alto
enum-osd-position-top-right = In alto a destra
enum-osd-position-right = Destra
enum-osd-position-bottom-right = In basso a destra
enum-osd-position-bottom = Basso
enum-osd-position-bottom-left = In basso a sinistra
enum-osd-position-left = Sinistra


## OsdTextAlign variants
enum-osd-text-align-start = Inizio
enum-osd-text-align-center = Centro
enum-osd-text-align-end = Fine


## OsdMedia variants
enum-osd-media-requested = Su richiesta
enum-osd-media-always = Sempre
enum-osd-media-never = Mai
