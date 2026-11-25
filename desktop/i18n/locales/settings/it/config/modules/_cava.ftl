### lys-shell Configuration - Cava Module

## Audio Processing

settings-modules-cava-bars = Numero di barre
    .description = Numero di barre di frequenza

settings-modules-cava-framerate = Frequenza dei fotogrammi
    .description = Frequenza di aggiornamento della visualizzazione in fotogrammi al secondo

settings-modules-cava-stereo = Stereo
    .description = Divide le barre tra i canali audio sinistro e destro

settings-modules-cava-noise-reduction = Riduzione del rumore
    .description = Intensità del filtro di riduzione del rumore (da 0.0 veloce/rumoroso a 1.0 lento/morbido)

settings-modules-cava-monstercat = Smussamento Monstercat
    .description = Decadimento morbido tra barre adiacenti (0.0 = disattivato)

settings-modules-cava-waves = Smussamento a onda
    .description = Numero di passaggi di smussamento a onda (0 = disattivato)

settings-modules-cava-low-cutoff = Taglio basso
    .description = Frequenza di taglio inferiore in Hz

settings-modules-cava-high-cutoff = Taglio alto
    .description = Frequenza di taglio superiore in Hz

settings-modules-cava-input = Metodo di ingresso
    .description = Backend di acquisizione audio

settings-modules-cava-source = Sorgente
    .description = Il suono che seguono le barre: scelto automaticamente, o una sorgente per nome

## Visualization

settings-modules-cava-style = Stile
    .description = Stile di resa della visualizzazione (barre, onda o picchi)

settings-modules-cava-direction = Direzione
    .description = Direzione di crescita delle barre rispetto al bordo della barra

settings-modules-cava-color = Colore
    .description = Colore della visualizzazione

settings-modules-cava-bar-width = Larghezza delle barre
    .description = Larghezza di ogni barra di frequenza in pixel

settings-modules-cava-bar-gap = Distanza tra le barre
    .description = Distanza tra le barre di frequenza in pixel

## Container

settings-modules-cava-internal-padding = Spaziatura interna
    .description = Spaziatura alle estremità del visualizzatore in rem
    .description-pixels = Spaziatura alle estremità del visualizzatore in pixel

settings-modules-cava-button-bg-color = Colore di sfondo
    .description = Colore di sfondo del modulo

settings-modules-cava-border-show = Mostra il bordo
    .description = Mostra un bordo attorno al visualizzatore

settings-modules-cava-border-color = Colore del bordo
    .description = Token del colore del bordo

settings-modules-cava-left-click = Clic sinistro
    .description = Azione al clic sinistro

settings-modules-cava-right-click = Clic destro
    .description = Azione al clic destro

settings-modules-cava-middle-click = Clic centrale
    .description = Azione al clic centrale

settings-modules-cava-scroll-up = Scorrimento in alto
    .description = Azione allo scorrimento in alto

settings-modules-cava-scroll-down = Scorrimento in basso
    .description = Azione allo scorrimento in basso


## CavaStyle variants
enum-cava-style-bars = Barre
enum-cava-style-wave = Onda
enum-cava-style-peaks = Picchi

## CavaDirection variants
enum-cava-direction-normal = Normale
enum-cava-direction-reverse = Inverso
enum-cava-direction-mirror = Speculare
