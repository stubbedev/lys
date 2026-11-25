### lys-shell Configuration - Media Module

## Media Module Configuration

settings-modules-media-icon-type = Tipo di icona
    .description = Cosa mostra l'icona multimediale: l'icona predefinita, quella dell'applicazione in riproduzione, un disco rotante o l'icona dell'applicazione dalla mappa delle icone. Il disco rotante usa un po' più di CPU per via dell'animazione

settings-modules-media-player-icons = Icone dei lettori
    .description = Associazioni personalizzate tra lettore e icona (pattern glob → nome dell'icona)

settings-modules-media-players-ignored = Lettori ignorati
    .description = Pattern dei nomi bus dei lettori da escludere dal rilevamento

settings-modules-media-player-priority = Priorità dei lettori
    .description = Ordine preferito dei lettori come pattern glob che corrispondono ai nomi bus

settings-modules-media-lyrics = Testi
    .description = Mostra i testi sincronizzati da LRCLIB nel menu a discesa multimediale. Invia artista, titolo, album e durata di ogni brano a lrclib.net

settings-modules-media-format = Formato
    .description = Formato dell'etichetta con i segnaposto: {"{{ title }}"}, {"{{ artist }}"}, {"{{ album }}"}, {"{{ status }}"}, {"{{ status_icon }}"}

settings-modules-media-icon-name = Nome dell'icona
    .description = Nome dell'icona simbolica per la modalità predefinita

settings-modules-media-spinning-disc-icon = Icona del disco rotante
    .description = Icona mostrata quando il tipo di icona è il disco rotante

settings-modules-media-border-show = Mostra il bordo
    .description = Mostra un bordo attorno al pulsante

settings-modules-media-border-color = Colore del bordo
    .description = Token del colore del bordo

settings-modules-media-icon-show = Mostra l'icona
    .description = Mostra l'icona del modulo

settings-modules-media-tooltip = Mostra il suggerimento
    .description = Mostra lo stato del modulo in un suggerimento al passaggio del mouse

settings-modules-media-icon-color = Colore dell'icona
    .description = Colore di primo piano dell'icona

settings-modules-media-icon-bg-color = Sfondo dell'icona
    .description = Colore di sfondo del contenitore dell'icona

settings-modules-media-label-show = Mostra l'etichetta
    .description = Mostra l'etichetta di testo

settings-modules-media-label-color = Colore dell'etichetta
    .description = Colore del testo dell'etichetta

settings-modules-media-label-max-length = Lunghezza massima dell'etichetta
    .description = Numero massimo di caratteri prima del troncamento

settings-modules-media-button-bg-color = Sfondo del pulsante
    .description = Colore di sfondo del pulsante

settings-modules-media-left-click = Clic sinistro
    .description = Azione al clic sinistro

settings-modules-media-right-click = Clic destro
    .description = Comando della shell al clic destro

settings-modules-media-middle-click = Clic centrale
    .description = Comando della shell al clic centrale

settings-modules-media-scroll-up = Scorrimento in alto
    .description = Comando della shell allo scorrimento in alto

settings-modules-media-scroll-down = Scorrimento in basso
    .description = Comando della shell allo scorrimento in basso


## MediaIconType variants
enum-media-icon-type-default = Predefinita
enum-media-icon-type-application = Applicazione
enum-media-icon-type-spinning-disc = Disco rotante
enum-media-icon-type-application-mapped = Associata all'applicazione

## Format placeholders

settings-modules-media-format-placeholder-title = Titolo del brano

settings-modules-media-format-placeholder-artist = Nome dell'artista o degli artisti

settings-modules-media-format-placeholder-album = Nome dell'album

settings-modules-media-format-placeholder-status = Stato della riproduzione (In riproduzione, In pausa, Fermato)

settings-modules-media-format-placeholder-status-icon = Simbolo dello stato della riproduzione
