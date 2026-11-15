### lys-shell Configuration - Hyprland Workspaces Module

## Hyprland Workspaces Module Configuration

settings-modules-hyprland-workspaces-min-workspace-count = Spazi di lavoro minimi
    .description = Numero minimo di pulsanti da mostrare (0 = solo attivi/occupati)

settings-modules-hyprland-workspaces-monitor-specific = Per monitor
    .description = Mostra solo gli spazi di lavoro di questo monitor

settings-modules-hyprland-workspaces-show-special = Mostra speciali
    .description = Include gli spazi di lavoro scratchpad

settings-modules-hyprland-workspaces-highlight-active-on-other-monitor = Evidenzia attivi su altri monitor
    .description = Usa un colore diverso per gli spazi di lavoro attivi su altri monitor (si applica solo quando { settings-modules-hyprland-workspaces-monitor-specific } è disattivato)

settings-modules-hyprland-workspaces-urgent-show = Mostra urgenti
    .description = Animazione pulsante sugli spazi di lavoro con finestre urgenti

settings-modules-hyprland-workspaces-display-mode = Modalità di visualizzazione
    .description = Cosa identifica ogni spazio di lavoro (etichetta, icona o nulla)

settings-modules-hyprland-workspaces-label-use-name = Usa il nome
    .description = Mostra il nome dello spazio di lavoro invece del numero

settings-modules-hyprland-workspaces-numbering = Numerazione
    .description = Come vengono mostrati i numeri degli spazi di lavoro (assoluti o relativi)

settings-modules-hyprland-workspaces-divider = Divisore
    .description = Testo tra l'identificativo dello spazio di lavoro e le icone delle app

settings-modules-hyprland-workspaces-app-icons-show = Mostra le icone delle app
    .description = Mostra le icone delle finestre per spazio di lavoro

settings-modules-hyprland-workspaces-app-icons-dedupe = Deduplica le icone
    .description = Mostra un'icona per classe di finestra

settings-modules-hyprland-workspaces-app-icons-fallback = Icona di riserva
    .description = Icona per le applicazioni non riconosciute

settings-modules-hyprland-workspaces-app-icons-empty = Icona vuota
    .description = Icona segnaposto per gli spazi di lavoro vuoti

settings-modules-hyprland-workspaces-icon-gap = Distanza tra le icone
    .description = Spaziatura tra le icone delle app (rem)

settings-modules-hyprland-workspaces-workspace-padding = Spaziatura dello spazio di lavoro
    .description = Spaziatura per il contenuto più largo del cerchio: icone delle app, un divisore o un nome lungo. Un'etichetta breve non ne ha: in quel caso dimensiona il cerchio

settings-modules-hyprland-workspaces-icon-size = Dimensione dell'icona
    .description = Moltiplicatore di scala per le icone degli spazi di lavoro (0.25-3.0)

settings-modules-hyprland-workspaces-label-size = Dimensione dell'etichetta
    .description = Moltiplicatore di scala per le etichette degli spazi di lavoro (0.25-3.0)

settings-modules-hyprland-workspaces-workspace-ignore = Ignora spazi di lavoro
    .description = Pattern glob degli ID degli spazi di lavoro da nascondere

settings-modules-hyprland-workspaces-active-indicator = Indicatore attivo
    .description = Stile visivo dello spazio di lavoro attivo

settings-modules-hyprland-workspaces-active-color = Colore attivo
    .description = Colore per icone ed etichette dello spazio di lavoro con il focus

settings-modules-hyprland-workspaces-active-on-other-monitor-color = Colore attivo su altri monitor
    .description = Colore per icone ed etichette degli spazi di lavoro attivi su altri monitor (usato solo quando { settings-modules-hyprland-workspaces-highlight-active-on-other-monitor } è attivo)

settings-modules-hyprland-workspaces-occupied-color = Colore occupato
    .description = Colore per icone ed etichette degli spazi di lavoro occupati

settings-modules-hyprland-workspaces-empty-color = Colore vuoto
    .description = Colore per icone ed etichette degli spazi di lavoro vuoti

settings-modules-hyprland-workspaces-container-bg-color = Sfondo del contenitore
    .description = Colore di sfondo del contenitore degli spazi di lavoro

settings-modules-hyprland-workspaces-border-show = Mostra il bordo
    .description = Mostra un bordo attorno al contenitore degli spazi di lavoro

settings-modules-hyprland-workspaces-border-color = Colore del bordo
    .description = Colore del bordo del contenitore degli spazi di lavoro

settings-modules-hyprland-workspaces-workspace-map = Mappa degli spazi di lavoro
    .description = Icone e colori specifici per spazio di lavoro

settings-modules-hyprland-workspaces-app-icon-map = Mappa delle icone delle app
    .description = Associazioni tra classe/titolo della finestra e icona

settings-modules-hyprland-workspaces-urgent-mode = Modalità urgente
    .description = Come vengono evidenziati gli spazi di lavoro urgenti (per spazio di lavoro o per applicazione)


## DisplayMode variants
enum-display-mode-label = Etichetta
enum-display-mode-icon = Icona
enum-display-mode-none = Nessuno

## ActiveIndicator variants
enum-active-indicator-background = Sfondo
enum-active-indicator-underline = Sottolineatura

settings-modules-hyprland-workspaces-left-click = Clic sinistro
    .description = Azione al clic sinistro
settings-modules-hyprland-workspaces-middle-click = Clic centrale
    .description = Azione al clic centrale
settings-modules-hyprland-workspaces-right-click = Clic destro
    .description = Azione al clic destro
settings-modules-hyprland-workspaces-scroll-up = Scorrimento in alto
    .description = Azione allo scorrimento in alto
settings-modules-hyprland-workspaces-scroll-down = Scorrimento in basso
    .description = Azione allo scorrimento in basso
