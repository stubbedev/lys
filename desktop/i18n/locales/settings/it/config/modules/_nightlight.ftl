### lys-shell Configuration - Night Light Module

## Night Light Module Configuration

settings-modules-nightlight-temperature = Temperatura
    .description = Temperatura del colore in Kelvin quando il filtro è attivo (1000-20000)

settings-modules-nightlight-gamma = Gamma
    .description = Percentuale di gamma dello schermo quando il filtro è attivo (0-200)

settings-modules-nightlight-auto-schedule = Pianificazione automatica
    .description = Attiva gradualmente il filtro al crepuscolo e lo disattiva all'alba. Segue l'alba e il crepuscolo civili nella posizione indicata da GeoClue, oppure alle coordinate qui sotto quando sono impostate entrambe

settings-modules-nightlight-transition-duration = Transizione
    .description = Durata in minuti della transizione tra giorno e notte, 0 per un passaggio istantaneo (0-120)

settings-modules-nightlight-latitude = Latitudine
    .description = Latitudine in gradi decimali (nord positivo, da -90 a 90). Impostata insieme alla longitudine, sostituisce GeoClue

settings-modules-nightlight-longitude = Longitudine
    .description = Longitudine in gradi decimali (est positivo, da -180 a 180). Impostata insieme alla latitudine, sostituisce GeoClue

settings-modules-nightlight-icon-off = Icona disattivata
    .description = Icona quando il filtro è disattivato

settings-modules-nightlight-icon-on = Icona attivata
    .description = Icona quando il filtro è attivo

settings-modules-nightlight-format = Formato
    .description = Stringa di formato dell'etichetta. Segnaposto: {"{{ status }}"}, {"{{ temp }}"}, {"{{ gamma }}"}, {"{{ config_temp }}"}, {"{{ config_gamma }}"}

settings-modules-nightlight-border-show = Mostra il bordo
    .description = Mostra un bordo attorno al pulsante

settings-modules-nightlight-border-color = Colore del bordo
    .description = Token del colore del bordo

settings-modules-nightlight-icon-show = Mostra l'icona
    .description = Mostra l'icona del modulo

settings-modules-nightlight-tooltip = Mostra il suggerimento
    .description = Mostra lo stato del modulo in un suggerimento al passaggio del mouse

settings-modules-nightlight-icon-color = Colore dell'icona
    .description = Colore di primo piano dell'icona

settings-modules-nightlight-icon-bg-color = Sfondo dell'icona
    .description = Colore di sfondo del contenitore dell'icona

settings-modules-nightlight-label-show = Mostra l'etichetta
    .description = Mostra l'etichetta

settings-modules-nightlight-label-color = Colore dell'etichetta
    .description = Colore del testo dell'etichetta

settings-modules-nightlight-label-max-length = Lunghezza massima dell'etichetta
    .description = Numero massimo di caratteri prima del troncamento

settings-modules-nightlight-button-bg-color = Sfondo del pulsante
    .description = Colore di sfondo del pulsante

settings-modules-nightlight-left-click = Clic sinistro
    .description = Azione al clic sinistro. Usa `:toggle` per l'attivazione integrata, vuoto per nessuna azione, oppure un comando della shell

settings-modules-nightlight-right-click = Clic destro
    .description = Comando della shell al clic destro

settings-modules-nightlight-middle-click = Clic centrale
    .description = Comando della shell al clic centrale

settings-modules-nightlight-scroll-up = Scorrimento in alto
    .description = Comando della shell allo scorrimento in alto

settings-modules-nightlight-scroll-down = Scorrimento in basso
    .description = Comando della shell allo scorrimento in basso

## Format placeholders

settings-modules-nightlight-format-placeholder-status = Stato del filtro (Attiva, Disattiva)

settings-modules-nightlight-format-placeholder-temp = Temperatura attuale in Kelvin (-- se disattivato)

settings-modules-nightlight-format-placeholder-gamma = Percentuale di gamma attuale (-- se disattivato)

settings-modules-nightlight-format-placeholder-config-temp = Temperatura configurata in Kelvin

settings-modules-nightlight-format-placeholder-config-gamma = Percentuale di gamma configurata
