### lys-shell Configuration - Niri Workspaces Module

## Niri Workspaces Module Configuration

settings-modules-niri-workspaces-min-workspace-count = Spazi di lavoro minimi
    .description = Mostra sempre gli spazi di lavoro con nome numerico fino a questo indice, anche se vuoti

settings-modules-niri-workspaces-monitor-specific = Per monitor
    .description = Mostra solo gli spazi di lavoro di questo monitor

settings-modules-niri-workspaces-hide-trailing-empty = Nascondi vuoto finale
    .description = Nasconde lo spazio di lavoro vuoto allocato automaticamente da niri alla fine di ogni uscita

settings-modules-niri-workspaces-display-mode = Modalità di visualizzazione
    .description = Cosa identifica ogni spazio di lavoro (etichetta, icona o nulla)

settings-modules-niri-workspaces-label-strategy = Strategia dell'etichetta
    .description = Come comporre l'etichetta dello spazio di lavoro da nome e indice

settings-modules-niri-workspaces-divider = Divisore
    .description = Testo tra l'identificativo dello spazio di lavoro e le icone delle app

settings-modules-niri-workspaces-urgent-show = Mostra urgenti
    .description = Animazione pulsante sugli spazi di lavoro con finestre urgenti

settings-modules-niri-workspaces-urgent-mode = Modalità urgente
    .description = Fa pulsare l'intero spazio di lavoro o solo l'icona dell'app urgente

settings-modules-niri-workspaces-app-icons-show = Mostra le icone delle app
    .description = Mostra le icone delle finestre per spazio di lavoro

settings-modules-niri-workspaces-app-icons-dedupe = Deduplica le icone
    .description = Mostra un'icona per app_id invece di una per finestra

settings-modules-niri-workspaces-app-icons-fallback = Icona di riserva
    .description = Icona per le finestre non presenti in app-icon-map

settings-modules-niri-workspaces-app-icons-empty = Icona vuota
    .description = Icona mostrata quando uno spazio di lavoro non ha finestre di applicazioni

settings-modules-niri-workspaces-icon-gap = Distanza tra le icone
    .description = Spaziatura tra le icone delle app

settings-modules-niri-workspaces-workspace-padding = Spaziatura dello spazio di lavoro
    .description = Spaziatura per il contenuto più largo del cerchio: icone delle app, un divisore o un nome lungo. Un'etichetta breve non ne ha: in quel caso dimensiona il cerchio

settings-modules-niri-workspaces-icon-size = Dimensione dell'icona
    .description = Moltiplicatore di scala per le icone degli spazi di lavoro (0.25-3.0)

settings-modules-niri-workspaces-label-size = Dimensione dell'etichetta
    .description = Moltiplicatore di scala per le etichette degli spazi di lavoro (0.25-3.0)

settings-modules-niri-workspaces-workspace-ignore = Ignora spazi di lavoro
    .description = Pattern glob confrontati con nome, indice o id

settings-modules-niri-workspaces-active-indicator = Indicatore attivo
    .description = Stile visivo dello spazio di lavoro attivo

settings-modules-niri-workspaces-active-color = Colore attivo
    .description = Colore per l'icona e l'etichetta dello spazio di lavoro attivo

settings-modules-niri-workspaces-occupied-color = Colore occupato
    .description = Colore per icone ed etichette degli spazi di lavoro occupati

settings-modules-niri-workspaces-empty-color = Colore vuoto
    .description = Colore per icone ed etichette degli spazi di lavoro vuoti

settings-modules-niri-workspaces-container-bg-color = Sfondo del contenitore
    .description = Colore di sfondo del contenitore degli spazi di lavoro

settings-modules-niri-workspaces-border-show = Mostra il bordo
    .description = Mostra un bordo attorno al contenitore degli spazi di lavoro

settings-modules-niri-workspaces-border-color = Colore del bordo
    .description = Colore del bordo del contenitore degli spazi di lavoro

settings-modules-niri-workspaces-workspace-map = Mappa degli spazi di lavoro
    .description = Icone e colori specifici per spazio di lavoro, indicizzati per nome o id

settings-modules-niri-workspaces-app-icon-map = Mappa delle icone delle app
    .description = Associazioni tra app_id o titolo della finestra e icona

settings-modules-niri-workspaces-left-click = Clic sinistro
    .description = Azione al clic sinistro

settings-modules-niri-workspaces-middle-click = Clic centrale
    .description = Azione al clic centrale

settings-modules-niri-workspaces-right-click = Clic destro
    .description = Azione al clic destro

settings-modules-niri-workspaces-scroll-up = Scorrimento in alto
    .description = Azione allo scorrimento in alto

settings-modules-niri-workspaces-scroll-down = Scorrimento in basso
    .description = Azione allo scorrimento in basso


## LabelStrategy variants
enum-label-strategy-index = Indice
enum-label-strategy-name-or-index = Nome o indice
enum-label-strategy-name-only = Solo nome
enum-label-strategy-index-and-name = Indice e nome
